#!/usr/bin/env python3
"""从一个 3D 模型自动生成 Buddy 骨架：判分支、出 buddy.json、无 clip 时出 rig.json 初稿。

用法：
  python3 gen_buddy.py --model cow.glb --id cow --name 小牛 --out ./cow
  python3 gen_buddy.py --model cow.glb              # 只报告，不落盘
  python3 gen_buddy.py --help

判定规则（确定性部分）：
  - GLB 内含 AnimationClip  → gltf-clip：直接产出 clips 映射草稿
  - 无 clip                → procedural：产出 rig.json 初稿（部件判据 + 动作参数）

部件分组是启发式，报告里逐组给置信度，必须由调用方复核后再用。
输出：stdout JSON 报告；落盘时写 buddy.json（及 rig.json）。
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from verify_buddy import parse_glb  # noqa: E402

# rig 参数默认值来自参考实现，独立存放在 rig-defaults.json（可替换，不视为规范）
DEFAULTS_FILE = Path(__file__).resolve().parent / "rig-defaults.json"
# 完整 rig 骨架（mindx.buddy.rig/1）：动作参数与 pivot 规则取参考实现，部件判据在生成时按模型换算
TEMPLATE_FILE = Path(__file__).resolve().parent / "rig-template.json"


def load_defaults():
    try:
        d = json.loads(DEFAULTS_FILE.read_text(encoding="utf-8"))
        return d
    except OSError:
        return {
            "head_y": 0.55, "ear_abs_z": 0.34, "hand_y": [0.19, 0.4],
            "hand_max_x": 0.25, "tail_max_x": -0.3, "motions": {},
        }


DEFAULTS = load_defaults()

CLIP_HINTS = {
    "idle": ("idle", "breath", "wait", "stand"),
    "busy": ("typing", "work", "busy", "think"),
    "question": ("confus", "question", "curious"),
    "complete": ("happy", "celebrate", "success", "win"),
    "failed": ("sad", "fail", "lose"),
    "play": ("pet", "pat", "touch"),
}


def normalize(meshes):
    """把每个 mesh 的包围盒换算到模型整体 0..1 空间。"""
    mins = [min(m["bbox"]["min"][k] for m in meshes) for k in range(3)]
    maxs = [max(m["bbox"]["max"][k] for m in meshes) for k in range(3)]
    span = [maxs[k] - mins[k] or 1.0 for k in range(3)]
    out = []
    for m in meshes:
        nb = {
            "min": [round((m["bbox"]["min"][k] - mins[k]) / span[k], 3) for k in range(3)],
            "max": [round((m["bbox"]["max"][k] - mins[k]) / span[k], 3) for k in range(3)],
            "center": [round((m["center"][k] - mins[k]) / span[k], 3) for k in range(3)],
            "size": [round((m["bbox"]["max"][k] - m["bbox"]["min"][k]) / span[k], 3) for k in range(3)],
        }
        out.append({"mesh": m["mesh"], "verts": m["verts"], "name": m["name"], **nb})
    return out, {"min": mins, "max": maxs}


def group_parts(funcs):
    """启发式分件，返回 {部件: [mesh...], 置信度}。阈值取自牛牛实测并换算比例。"""
    head = [f for f in funcs if f["max"][1] > DEFAULTS["head_y"]]
    ears = [f for f in head if max(abs(f["min"][2] - 0.5), abs(f["max"][2] - 0.5)) > DEFAULTS["ear_abs_z"]]
    hx = [f["size"][2] for f in head] or [0.5]
    eyes = [
        f for f in head
        if f["verts"] < max(1, sum(x["verts"] for x in head) / max(1, len(head)) * 0.35)
        and f["center"][0] > 0.5
    ]
    hands = [
        f for f in funcs
        if DEFAULTS["hand_y"][0] < f["center"][1] < DEFAULTS["hand_y"][1] and f["max"][0] < DEFAULTS["hand_max_x"] + 0.5
    ]
    tails = [f for f in funcs if f["center"][0] < DEFAULTS["tail_max_x"] + 0.5]
    return {
        "head": {"meshes": [f["mesh"] for f in head], "confidence": "high" if head else "low"},
        "ear": {"meshes": [f["mesh"] for f in ears], "confidence": "high" if len(ears) >= 2 else "low"},
        "eye": {"meshes": [f["mesh"] for f in eyes], "confidence": "low", "note": "顶点数与位置启发式，需人工确认"},
        "hand": {"meshes": [f["mesh"] for f in hands], "confidence": "medium" if hands else "low"},
        "tail": {"meshes": [f["mesh"] for f in tails], "confidence": "medium" if tails else "low"},
    }, (0.5 - DEFAULTS["ear_abs_z"]) if head else 0.5


def guess_clips(clips):
    out = {}
    lowered = {c.lower(): c for c in clips}
    for semantic, hints in CLIP_HINTS.items():
        for h in hints:
            for low, real in lowered.items():
                if h in low:
                    out[semantic] = real
                    break
            if semantic in out:
                break
    if "idle" not in out and clips:
        out["idle"] = clips[0]
    return out


def build_rig(model_bb, defaults):
    """出 mindx.buddy.rig/1 骨架：动作参数取模板，部件判据按本模型包围盒把
    0..1 归一化空间阈值换算回模型局部坐标（procedural 按局部坐标匹配）。

    y 轴：局部 = min.y + t * span.y；x/z 轴：0..1 空间的 t 直接换算成局部，z 的“距中线”阈值乘以 span.z。
    """
    tpl = json.loads(TEMPLATE_FILE.read_text(encoding="utf-8"))
    mins, maxs = model_bb["min"], model_bb["max"]
    span = [(maxs[k] - mins[k]) or 1.0 for k in range(3)]

    def y(t):
        return round(mins[1] + t * span[1], 3)

    def x(t):
        return round(mins[0] + t * span[0], 3)

    def zc(t):
        return round(t * span[2], 3)

    parts = {p["name"]: p for p in tpl["parts"]}
    tpl["head"]["select"] = {"maxYAbove": y(defaults["head_y"])}
    parts["ear"]["select"] = {"absZAbove": zc(defaults["ear_abs_z"])}
    parts["eye"]["select"] = {
        "centerXAbove": x(0.55),
        "centerYBetween": [y(0.6), y(0.75)],
        "absZBelow": zc(0.2),
    }
    parts["hand"]["select"] = {
        "minYAbove": y(defaults["hand_y"][0]),
        "maxYBelow": y(defaults["hand_y"][1]),
        "maxXBelow": x(0.5 + defaults["hand_max_x"]),
    }
    parts["tail"]["select"] = {"maxXBelow": x(0.5 + defaults["tail_max_x"])}
    return tpl


def main():
    ap = argparse.ArgumentParser(description="从 GLB 生成 Buddy 骨架")
    ap.add_argument("--model", required=True, help="模型文件（.glb）")
    ap.add_argument("--id", default=None, help="组件 id，缺省取文件名")
    ap.add_argument("--name", default=None, help="展示名，缺省同 id")
    ap.add_argument("--out", default=None, help="落盘目录；不给则只输出报告")
    ap.add_argument("--version", default="0.1.0")
    ap.add_argument("--description", default="")
    args = ap.parse_args()

    model = Path(args.model).expanduser()
    if not model.is_file():
        print(f"[ERROR] 模型不存在：{model}", file=sys.stderr)
        return 1
    if model.suffix.lower() != ".glb":
        print("[ERROR] 只支持 .glb（glTF 2.0 二进制）", file=sys.stderr)
        return 1

    try:
        info = parse_glb(model)
    except ValueError as e:
        print(f"[ERROR] GLB 解析失败：{e}", file=sys.stderr)
        return 1

    pid = args.id or model.stem.lower().replace(" ", "-")
    name = args.name or pid
    has_clips = bool(info["clips"])
    branch = "gltf-clip" if has_clips else "procedural"

    funcs, model_bb = normalize(info["meshes"])
    groups, _ = group_parts(funcs)

    report = {
        "rigDefaults": "scripts/rig-defaults.json (参考实现 A 参数)",
        "model": str(model),
        "size_mb": round(model.stat().st_size / 1048576, 2),
        "triangles": info["triangles"],
        "clips": info["clips"],
        "branch": branch,
        "meshes": funcs,
        "groups": groups,
    }

    manifest = {
        "id": pid,
        "name": name,
        "version": args.version,
        "renderer": branch,
        "model": model.name,
        "viewport": {"width": 107, "height": 107, "anchor": "bottom-center"},
    }
    if args.description:
        manifest["description"] = args.description

    rig = None
    if has_clips:
        manifest["clips"] = guess_clips(info["clips"])
        report["clips_mapping"] = manifest["clips"]
    else:
        rig = build_rig(model_bb, DEFAULTS)
        manifest["rig"] = "rig.json"

    report["manifest"] = manifest
    if rig:
        report["rig"] = rig

    if args.out:
        out = Path(args.out).expanduser()
        # 目录名必须等于 id（verify 按此判），不一致时自动落到 out/<id>
        if out.name != pid:
            out = out / pid
        out.mkdir(parents=True, exist_ok=True)
        (out / "buddy.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        import shutil
        shutil.copy2(model, out / model.name)
        if rig:
            (out / "rig.json").write_text(json.dumps(rig, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        report["written"] = str(out)

    print(json.dumps(report, ensure_ascii=False, indent=2))
    print(f"[OK] 分支 {branch}（{'有 ' + str(len(info['clips'])) + ' 条 clip' if has_clips else '无 clip，走程序化 rig'}）", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
