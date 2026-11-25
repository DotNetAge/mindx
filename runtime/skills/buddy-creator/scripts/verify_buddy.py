#!/usr/bin/env python3
"""校验一个 Buddy 组件目录是否符合 buddy.json 契约与准入线。

用法：
  python3 verify_buddy.py <组件目录>
  python3 verify_buddy.py --inspect <model.glb>   # 输出网格分布，供生成 rig.json
  python3 verify_buddy.py --help

输出：stdout 一份 JSON 报告；stderr 逐条人类可读结论。
退出码：0 通过；1 存在硬错误；2 仅警告。
"""

from __future__ import annotations

import json
import math
import os
import re
import struct
import sys
from pathlib import Path

ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{1,31}$")
VER_RE = re.compile(r"^\d+\.\d+\.\d+$")
RENDERERS = {"gltf-clip", "sprite", "lottie", "video", "procedural", "custom"}
CLIP_KEYS = {
    "idle", "busy", "question", "complete", "failed",
    "notice", "update", "play", "jump", "drag",
}
ANCHORS = {"bottom-center", "bottom-left", "bottom-right", "center"}

LIMITS = {
    "glb": 10 * 1024 * 1024,
    "atlas": 2 * 1024 * 1024,
    "sound": 512 * 1024,
    "total": 14 * 1024 * 1024,
    "tex_side": 1024,
    "triangles": 350_000,
}


def fail(errors, code, msg, path=None):
    errors.append({"code": code, "message": msg, "path": path})


def warn(warnings, code, msg, path=None):
    warnings.append({"code": code, "message": msg, "path": path})


# ---------- 图片尺寸（PNG / JPEG / WebP） ----------

def image_size(path: Path):
    try:
        data = path.read_bytes()[:64]
    except OSError:
        return None
    if data[:8] == b"\x89PNG\r\n\x1a\n":
        w, h = struct.unpack(">II", data[16:24])
        return w, h
    if data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        chunk = data[12:16]
        if chunk == b"VP8X":
            w = int.from_bytes(data[24:27], "little") + 1
            h = int.from_bytes(data[27:30], "little") + 1
            return w, h
        if chunk == b"VP8 ":
            raw = path.read_bytes()
            w, h = struct.unpack("<HH", raw[26:30])
            return w & 0x3FFF, h & 0x3FFF
    if data[:2] == b"\xff\xd8":
        raw = path.read_bytes()
        i = 2
        while i < len(raw) - 9:
            if raw[i] != 0xFF:
                i += 1
                continue
            marker = raw[i + 1]
            if marker in (0xC0, 0xC1, 0xC2):
                h, w = struct.unpack(">HH", raw[i + 5:i + 9])
                return w, h
            seg = struct.unpack(">H", raw[i + 2:i + 4])[0]
            i += 2 + seg
    return None


# ---------- GLB 解析 ----------

def parse_glb(path: Path):
    raw = path.read_bytes()
    if raw[:4] != b"glTF":
        raise ValueError("不是 GLB（magic 不是 glTF）")
    version, length = struct.unpack("<II", raw[4:12])
    if version != 2:
        raise ValueError(f"GLB version={version}，只支持 2")
    if length != len(raw):
        raise ValueError(f"GLB 头声明长度 {length} 与实际 {len(raw)} 不一致")
    chunk_len, chunk_type = struct.unpack("<II", raw[12:20])
    if chunk_type != 0x4E4F534A:
        raise ValueError("首个 chunk 不是 JSON")
    doc = json.loads(raw[20:20 + chunk_len].decode("utf-8"))
    clips = [a.get("name", "") for a in doc.get("animations", [])]
    accessors = doc.get("accessors", [])
    tris = 0
    meshes = []
    for mi, mesh in enumerate(doc.get("meshes", [])):
        bbox_min = [math.inf] * 3
        bbox_max = [-math.inf] * 3
        verts = 0
        for prim in mesh.get("primitives", []):
            if "indices" in prim:
                tris += accessors[prim["indices"]].get("count", 0) // 3
            pos_idx = prim.get("attributes", {}).get("POSITION")
            if pos_idx is None:
                continue
            acc = accessors[pos_idx]
            verts += acc.get("count", 0)
            if "min" in acc and "max" in acc:
                for k in range(3):
                    bbox_min[k] = min(bbox_min[k], acc["min"][k])
                    bbox_max[k] = max(bbox_max[k], acc["max"][k])
        if verts:
            center = [round((bbox_min[k] + bbox_max[k]) / 2, 3) for k in range(3)]
            meshes.append({
                "mesh": mi,
                "name": mesh.get("name") or "",
                "verts": verts,
                "bbox": {"min": [round(v, 3) for v in bbox_min], "max": [round(v, 3) for v in bbox_max]},
                "center": center,
            })
    images = doc.get("images", [])
    return {"version": version, "clips": clips, "triangles": tris, "meshes": meshes, "images": len(images)}


# ---------- 校验主流程 ----------

def verify(root: Path):
    errors, warnings = [], []
    report = {"dir": str(root), "ok": False, "errors": errors, "warnings": warnings}

    if not root.is_dir():
        fail(errors, "dir_missing", f"目录不存在：{root}")
        return report

    manifest_path = root / "buddy.json"
    if not manifest_path.is_file():
        fail(errors, "manifest_missing", "缺少 buddy.json")
        return report

    try:
        m = json.loads(manifest_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        fail(errors, "manifest_broken", f"buddy.json 不是合法 JSON：{e}")
        return report

    for key in ("id", "name", "version", "renderer", "model"):
        if not m.get(key):
            fail(errors, "field_missing", f"缺少必填字段 {key}")

    pid = m.get("id", "")
    if pid and not ID_RE.match(pid):
        fail(errors, "id_invalid", f"id 不合规：{pid}（须小写字母数字连字符，2-32 位）")
    if pid and pid != root.name:
        # 作者目录（<id>/）与部署目录（<id>/<version>/）两种形态都合法
        if not (VER_RE.match(root.name) and root.parent.name == pid):
            fail(errors, "id_dir_mismatch", f"id={pid} 与目录名 {root.name} 不一致（须为 <id>/ 或 <id>/<version>/）")
    if m.get("version") and not VER_RE.match(m["version"]):
        fail(errors, "version_invalid", f"version 不合规：{m['version']}")

    renderer = m.get("renderer")
    if renderer and renderer not in RENDERERS:
        fail(errors, "renderer_invalid", f"renderer 必须是 {sorted(RENDERERS)}")

    model = m.get("model")
    model_path = None
    if model:
        model_path = (root / model).resolve()
        if not str(model_path).startswith(str(root.resolve())):
            fail(errors, "path_escape", f"model 越出组件目录：{model}")
        elif not model_path.is_file():
            fail(errors, "model_missing", f"model 文件不存在：{model}")

    # 体积
    total = 0
    for p in root.rglob("*"):
        if p.is_file():
            total += p.stat().st_size
    if total > LIMITS["total"]:
        fail(errors, "size_total", f"目录总资源 {total/1048576:.2f} MB 超过 14 MB")
    report["total_bytes"] = total

    if model_path and model_path.is_file():
        size = model_path.stat().st_size
        limit = LIMITS["glb"] if model_path.suffix.lower() == ".glb" else LIMITS["atlas"]
        if size > limit:
            fail(errors, "size_model", f"{model} 为 {size/1048576:.2f} MB，超过 {limit//1048576} MB")

    # 渲染器专项
    if renderer == "sprite":
        frames = m.get("frames")
        if not isinstance(frames, dict):
            fail(errors, "frames_missing", "sprite 渲染器必须声明 frames")
        else:
            count, cols, fps = frames.get("count"), frames.get("cols"), frames.get("fps")
            if not isinstance(count, int) or count < 1:
                fail(errors, "frames_invalid", "frames.count 必须是 ≥1 的整数")
            if not isinstance(cols, int) or cols < 1:
                fail(errors, "frames_invalid", "frames.cols 必须是 ≥1 的整数")
            if not isinstance(fps, (int, float)) or not (1 <= fps <= 30):
                fail(errors, "frames_invalid", "frames.fps 必须在 1-30")
            if model_path and model_path.is_file() and isinstance(cols, int) and cols > 0:
                size = image_size(model_path)
                if size:
                    w, h = size
                    if w % cols != 0:
                        warn(warnings, "atlas_not_divisible", f"图集宽 {w} 不能被 cols={cols} 整除")
                    rows = math.ceil((count or 0) / cols) if count else 0
                    if rows and h % rows != 0:
                        warn(warnings, "atlas_not_divisible", f"图集高 {h} 不能被 rows={rows} 整除")
                    if max(w, h) > LIMITS["tex_side"]:
                        warn(warnings, "tex_side", f"图集边长 {w}x{h} 超过 1024")

    if renderer == "gltf-clip" and model_path and model_path.is_file():
        try:
            info = parse_glb(model_path)
            report["glb"] = info
            if info["triangles"] > LIMITS["triangles"]:
                fail(errors, "triangles", f"三角面 {info['triangles']} 超过 350000")
            clips = m.get("clips") or {}
            if not clips.get("idle"):
                fail(errors, "clip_idle_missing", "clips.idle 必填（组件常驻动作）")
            available = set(info["clips"])
            for key, name in clips.items():
                if key not in CLIP_KEYS:
                    warn(warnings, "clip_key_unknown", f"clips 键 {key} 不在语义集合内")
                    continue
                if name not in available:
                    fail(errors, "clip_not_found", f"clips.{key}={name} 在 GLB 内不存在；可用：{sorted(available)}")
        except ValueError as e:
            fail(errors, "glb_invalid", f"GLB 解析失败：{e}")

    if renderer == "procedural":
        rig = m.get("rig")
        if not rig:
            fail(errors, "rig_missing", "procedural 渲染器必须声明 rig 字段")
        elif not (root / str(rig)).is_file():
            fail(errors, "rig_missing", f"rig 文件不存在：{rig}")

    if renderer == "custom":
        runtime = root / "runtime.js"
        if not runtime.is_file():
            fail(errors, "runtime_missing", "custom 渲染器必须提供 runtime.js")
        else:
            warn(warnings, "custom_code", "custom 含可执行代码，须按沙箱约束装载（见 references/renderers.md）")

    # 可选字段
    viewport = m.get("viewport")
    if viewport is None:
        warn(warnings, "viewport_missing", "未声明 viewport，壳将按资源边界推导")
    elif not isinstance(viewport, dict):
        fail(errors, "viewport_invalid", "viewport 必须是对象")
    else:
        for k in ("width", "height"):
            v = viewport.get(k)
            if not isinstance(v, int) or not (16 <= v <= 512):
                fail(errors, "viewport_invalid", f"viewport.{k} 必须是 16-512 的整数")
        if viewport.get("anchor") not in ANCHORS:
            fail(errors, "viewport_invalid", f"viewport.anchor 必须是 {sorted(ANCHORS)}")

    sound = m.get("sound")
    if sound:
        sp = root / sound
        if not sp.is_file():
            fail(errors, "sound_missing", f"sound 文件不存在：{sound}")
        elif sp.stat().st_size > LIMITS["sound"]:
            warn(warnings, "sound_size", f"{sound} 超过 512 KB")

    durations = m.get("durations")
    if isinstance(durations, dict):
        for k, v in durations.items():
            if k not in CLIP_KEYS:
                warn(warnings, "duration_key_unknown", f"durations 键 {k} 不在语义集合内")
            if not isinstance(v, int) or not (200 <= v <= 3000):
                warn(warnings, "duration_invalid", f"durations.{k}={v} 应在 200-3000 毫秒")

    if not m.get("description"):
        warn(warnings, "description_missing", "未填 description，设置页只显示名称")

    report["ok"] = not errors
    return report


def main(argv):
    if len(argv) == 2 and argv[1] == "--inspect":
        print(__doc__.strip())
        print("\n--inspect <model.glb>：输出每个 mesh 的包围盒、顶点数与动画名，供生成 rig.json。")
        return 2
    if len(argv) == 3 and argv[1] == "--inspect":
        p = Path(argv[2]).expanduser()
        if not p.is_file():
            print(f"[ERROR] 文件不存在：{p}", file=sys.stderr)
            return 1
        try:
            info = parse_glb(p)
        except ValueError as e:
            print(f"[ERROR] GLB 解析失败：{e}", file=sys.stderr)
            return 1
        info.pop("version", None)
        print(json.dumps({"file": str(p), **info}, ensure_ascii=False, indent=2))
        return 0
    if len(argv) != 2 or argv[1] in ("--help", "-h"):
        print(__doc__.strip())
        return 0 if len(argv) == 2 else 2
    report = verify(Path(argv[1]).expanduser())
    print(json.dumps(report, ensure_ascii=False, indent=2))
    for e in report["errors"]:
        print(f"[ERROR] {e['code']}: {e['message']}", file=sys.stderr)
    for w in report["warnings"]:
        print(f"[WARN]  {w['code']}: {w['message']}", file=sys.stderr)
    if report["errors"]:
        return 1
    if report["warnings"]:
        return 2
    print("[OK] 校验通过", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
