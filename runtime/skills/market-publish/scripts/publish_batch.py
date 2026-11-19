#!/usr/bin/env python3
"""批量增量发布：把目录下「变更过」的实体逐包发布到资源社区。

用法：
  python3 publish_batch.py --kind all [--src <dir>] [--dry-run] [--no-bump] [--limit N]

  --kind    skill | agent | team | all（all 扫 ~/.mindx 下三类目录）
  --src     自定义根目录（含 <name>/SKILL.md 等）；缺省按 kind 取 ~/.mindx 标准根
  --dry-run 只打印将要发布的清单，不上传、不改版本
  --no-bump 内容变更但远端已有同版本时不自动升版（默认升 patch）
  --limit   只处理前 N 个（首次跑批量时用来小批验证）

判定「变更」的口径：实体目录全部文件内容指纹（排除 frontmatter 的 version 行），
与状态台账 ~/.mindx/market-publish-state.json 比对。打包字节带构建时间戳、不可复现，
因此不能用包 sha256 判断变更。

输出：stdout 一份 JSON 汇总（published / skipped / failed / needs_manual）；
进度走 stderr。退出码：0 全成功或无事可做，1 有失败。
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

MINDX_BIN = os.environ.get("MINDX_BIN") or str(Path.home() / ".mindx" / "bin" / "mindx")
HOME = Path.home()
STATE_PATH = HOME / ".mindx" / "market-publish-state.json"
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
VERSION_LINE_RE = re.compile(r"^(\s*)version:\s*[0-9][^\s]*\s*$")

ROOTS = {
    "skill": HOME / ".mindx" / "skills",
    "agent": HOME / ".mindx" / "agents",
    "team": HOME / ".mindx" / "teams",
}
MARKERS = {"skill": "SKILL.md", "agent": "IDENTITY.md", "team": "TEAM.md"}


def log(msg: str) -> None:
    print(f"[publish-batch] {msg}", file=sys.stderr)


def run_mindx(args: list[str], timeout: int = 300) -> tuple[int, str]:
    try:
        proc = subprocess.run(
            [MINDX_BIN, *args],
            capture_output=True,
            text=True,
            timeout=timeout,
            env={**os.environ, "NO_COLOR": "1", "TERM": "dumb"},
        )
    except FileNotFoundError:
        log(f"找不到 mindx 可执行文件：{MINDX_BIN}")
        return 127, ""
    except subprocess.TimeoutExpired:
        log(f"mindx {' '.join(args)} 超时（{timeout}s）")
        return 124, ""
    return proc.returncode, ANSI_RE.sub("", proc.stdout + proc.stderr)


def parse_json_stdout(stdout: str) -> dict | None:
    """取输出里第一个完整 JSON 对象：stdout/stderr 合流后，回执前后都夹着网关日志。"""
    idx = stdout.find("{")
    if idx < 0:
        return None
    try:
        obj, _ = json.JSONDecoder().raw_decode(stdout[idx:])
        return obj if isinstance(obj, dict) else None
    except json.JSONDecodeError:
        return None


def read_frontmatter_version(path: Path) -> str:
    """取 frontmatter 版本：技能在 metadata.version（嵌套），agent/team 在顶层 version。"""
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return ""
    end = text.find("\n---", 3)
    body = text[3:end] if end > 0 else text[3:]
    top = ""
    nested = ""
    parent = None
    for raw in body.splitlines():
        indent = len(raw) - len(raw.lstrip())
        m = re.match(r"^([A-Za-z_]+):\s*(.*)$", raw.strip())
        if not m:
            continue
        key, value = m.group(1), m.group(2).strip()
        if indent == 0:
            parent = key if value == "" else None
            if key == "version" and value:
                top = value
        elif parent == "metadata" and key == "version" and value:
            nested = value
    return (nested or top).strip().strip('"').strip("'")


def entity_fingerprint(entry_dir: Path) -> str:
    """实体内容指纹：目录内全部文件按相对路径排序后拼接内容，剔除 frontmatter version 行。"""
    parts: list[str] = []
    for path in sorted(entry_dir.rglob("*"), key=lambda p: str(p)):
        if not path.is_file() or path.name.startswith("."):
            continue
        rel = str(path.relative_to(entry_dir))
        try:
            data = path.read_bytes().decode("utf-8")
        except (UnicodeDecodeError, OSError):
            parts.append(f"{rel}:bin:{hashlib.sha256(path.read_bytes()).hexdigest()}")
            continue
        lines = [ln for ln in data.splitlines() if not VERSION_LINE_RE.match(ln)]
        parts.append(f"{rel}:" + hashlib.sha256("\n".join(lines).encode("utf-8")).hexdigest())
    return hashlib.sha256("|".join(parts).encode("utf-8")).hexdigest()


def load_state() -> dict:
    if not STATE_PATH.is_file():
        return {}
    try:
        return json.loads(STATE_PATH.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        log("状态台账损坏，按空台账继续（本轮会把所有实体视为首次发布）")
        return {}


def save_state(state: dict) -> None:
    STATE_PATH.parent.mkdir(parents=True, exist_ok=True)
    tmp = STATE_PATH.with_suffix(".tmp")
    tmp.write_text(json.dumps(state, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
    tmp.replace(STATE_PATH)


def prefetch_remote(kind: str) -> dict[str, str]:
    """一次性拉取某类的远端包清单（name → 最新版本）。

    逐包调用 list 会让批量发布退化成 N 次 CLI 往返（每次 3s 级），500 包不可用；
    这里分页拉全量，失败时返回空字典，调用方退回逐包查询。
    """
    mapping: dict[str, str] = {}
    for page in range(1, 8):
        code, out = run_mindx(["market", "list", "--kind", kind, "--page", str(page),
                                "--page-size", "100", "--json"])
        if code != 0:
            break
        payload = parse_json_stdout(out) or {}
        pkgs = payload.get("packages") or []
        if not pkgs:
            break
        for pkg in pkgs:
            mapping.setdefault(str(pkg.get("name") or ""), str(pkg.get("version") or ""))
        if len(pkgs) < 100:
            break
    log(f"远端清单预取：{kind} {len(mapping)} 个包")
    return mapping


def remote_latest(kind: str, name: str) -> str:
    code, out = run_mindx(["market", "list", "--kind", kind, "--q", name, "--json"])
    if code != 0:
        return ""
    payload = parse_json_stdout(out) or {}
    for pkg in payload.get("packages") or []:
        if pkg.get("name") == name:
            return str(pkg.get("version") or "")
    return ""


def bump_patch(version: str) -> str:
    parts = version.split(".")
    if len(parts) != 3 or not all(p.isdigit() for p in parts):
        return ""
    return f"{parts[0]}.{parts[1]}.{int(parts[2]) + 1}"


def write_version(marker: Path, new_version: str) -> bool:
    """把 frontmatter 版本改写成 new_version（只动 version 行，其余原样）。"""
    text = marker.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return False
    end = text.find("\n---", 3)
    if end < 0:
        return False
    head, body = text[: end], text[end:]
    lines = head.splitlines()
    changed = False
    parent = None
    for i, raw in enumerate(lines):
        indent = len(raw) - len(raw.lstrip())
        m = re.match(r"^([A-Za-z_]+):\s*(.*)$", raw.strip())
        if not m:
            continue
        key, value = m.group(1), m.group(2).strip()
        if indent == 0:
            parent = key if value == "" else None
            if key == "version" and value:
                lines[i] = raw.replace(value, new_version, 1)
                changed = True
        elif parent == "metadata" and key == "version" and value:
            lines[i] = raw.replace(value, new_version, 1)
            changed = True
    if not changed:
        return False
    marker.write_text("\n".join(lines) + body, encoding="utf-8")
    return True


def collect_entries(kind: str, src: str | None) -> list[tuple[str, str, Path]]:
    root = Path(src) if src else ROOTS[kind]
    marker_name = MARKERS[kind]
    if not root.is_dir():
        return []
    out: list[tuple[str, str, Path]] = []
    for child in sorted(root.iterdir()):
        if not child.is_dir() or child.name.startswith("."):
            continue
        marker = child / marker_name
        if marker.is_file():
            out.append((kind, child.name, child))
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description="批量增量发布实体到资源社区")
    ap.add_argument("--kind", default="all", choices=["skill", "agent", "team", "all"])
    ap.add_argument("--src", default=None, help="自定义实体根目录（含 <name>/<marker>）")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--no-bump", action="store_true")
    ap.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()

    kinds = ["skill", "agent", "team"] if args.kind == "all" else [args.kind]
    remote_index = {k: prefetch_remote(k) for k in kinds}
    state = load_state()
    entries: list[tuple[str, str, Path]] = []
    for kind in kinds:
        entries.extend(collect_entries(kind, args.src))

    result = {"published": [], "skipped": [], "failed": [], "needs_manual": [], "dry_run": args.dry_run}
    plan: list[dict] = []

    for kind, name, entry_dir in entries:
        key = f"{kind}/{name}"
        fp = entity_fingerprint(entry_dir)
        local_version = read_frontmatter_version(entry_dir / MARKERS[kind])
        prev = state.get(key) or {}
        if prev.get("fingerprint") == fp and local_version and prev.get("version") == local_version:
            result["skipped"].append({"key": key, "reason": "未变更"})
            continue
        remote = remote_index.get(kind, {}).get(name, "")
        # 台账无记录且本地版本与远端一致 = 该实体是从社区安装下来的副本，
        # 不是本机改动；登记基线后跳过，避免把安装展开物当新内容重发污染元数据。
        if not prev and local_version and remote == local_version:
            state[key] = {"fingerprint": fp, "version": local_version, "baseline": True}
            result["skipped"].append({"key": key, "reason": "与远端同版本，登记为安装副本基线"})
            continue
        target = local_version
        if remote and remote == local_version:
            bumped = bump_patch(local_version)
            if not bumped:
                result["needs_manual"].append({
                    "key": key,
                    "reason": f"内容已变更但版本 {local_version} 非 X.Y.Z，无法自动升版",
                })
                continue
            target = bumped
        plan.append({
            "kind": kind, "name": name, "key": key, "entry_dir": str(entry_dir),
            "marker": str(entry_dir / MARKERS[kind]),
            "local_version": local_version, "remote_version": remote, "target_version": target,
        })

    if args.limit > 0:
        plan = plan[: args.limit]

    log(f"待处理 {len(plan)} 个实体（变更或首次发布）")
    for item in plan:
        if args.dry_run:
            log(f"将发布 {item['key']} {item['local_version']} → {item['target_version']}（远端 {item['remote_version'] or '无'}）")
            continue
        if item["target_version"] != item["local_version"]:
            if args.no_bump:
                result["needs_manual"].append({"key": item["key"], "reason": "--no-bump：需先手工升版"})
                continue
            if not write_version(Path(item["marker"]), item["target_version"]):
                result["needs_manual"].append({"key": item["key"], "reason": "无法写入 frontmatter 版本"})
                continue
        code, out = run_mindx([
            "market", "publish", item["kind"], item["name"],
            "--version", item["target_version"], "--json",
        ])
        payload = parse_json_stdout(out)
        # 版本占用兜底：list 的 version 只是当前主版本，历史版本列表里可能已有更高版本
        # （升版后未改 latest 指针等情况）→ 服务端回 409。自动再升一次 patch 重试，
        # 这是批量发布不因个别历史版本中断的必要路径。
        attempts = 0
        while code != 0 and "同版本已存在" in out and not args.no_bump and attempts < 3:
            attempts += 1
            retry_version = bump_patch(item["target_version"])
            if not retry_version or not write_version(Path(item["marker"]), retry_version):
                break
            log(f"{item['key']} 版本 {item['target_version']} 已被占用，升至 {retry_version} 重试")
            item["target_version"] = retry_version
            code, out = run_mindx([
                "market", "publish", item["kind"], item["name"],
                "--version", retry_version, "--json",
            ])
            payload = parse_json_stdout(out)
        if code != 0 or not payload:
            detail = out.strip().replace("\n", " ")[:200] or "发布失败（无输出）"
            result["failed"].append({"key": item["key"], "detail": detail})
            continue
        result["published"].append({
            "key": item["key"], "version": item["target_version"],
            "sha256": payload.get("sha256"), "warnings": payload.get("warnings"),
        })
        state[item["key"]] = {
            "fingerprint": entity_fingerprint(Path(item["entry_dir"])),
            "version": item["target_version"],
        }

    if not args.dry_run:
        save_state(state)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 1 if result["failed"] else 0


if __name__ == "__main__":
    sys.exit(main())
