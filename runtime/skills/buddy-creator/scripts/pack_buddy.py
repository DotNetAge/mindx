#!/usr/bin/env python3
"""把 Buddy 组件目录打成可安装的 .mindpkg（zip），并输出 sha256。

用法：
  python3 pack_buddy.py ./my-pet
  python3 pack_buddy.py ./my-pet --out dist/my-pet.mindpkg
  python3 pack_buddy.py --help

前置：先过校验（内部调用 verify_buddy.verify），有硬错误即拒绝打包。
输出：stdout JSON（包路径、大小、sha256、文件清单）；退出码 0 成功、1 校验未过或打包失败。
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from verify_buddy import verify  # noqa: E402

SKIP_NAMES = {".DS_Store", "Thumbs.db", "README.md"}
SKIP_DIRS = {".git", "node_modules", "__pycache__"}


def collect(root: Path):
    files = []
    for p in sorted(root.rglob("*")):
        if not p.is_file():
            continue
        rel = p.relative_to(root)
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        if p.name in SKIP_NAMES or p.name.startswith("."):
            continue
        files.append(rel)
    return files


def main():
    ap = argparse.ArgumentParser(description="打包 Buddy 组件为 .mindpkg")
    ap.add_argument("dir", help="组件目录（目录名须等于清单 id）")
    ap.add_argument("--out", default=None, help="输出包路径，缺省 <id>-<version>.mindpkg")
    ap.add_argument("--skip-verify", action="store_true", help="跳过校验（仅调试用）")
    args = ap.parse_args()

    root = Path(args.dir).expanduser().resolve()
    if not root.is_dir():
        print(f"[ERROR] 目录不存在：{root}", file=sys.stderr)
        return 1

    if not args.skip_verify:
        report = verify(root)
        if report["errors"]:
            print(json.dumps({"ok": False, "errors": report["errors"]}, ensure_ascii=False, indent=2))
            for e in report["errors"]:
                print(f"[ERROR] {e['code']}: {e['message']}", file=sys.stderr)
            return 1

    manifest = json.loads((root / "buddy.json").read_text(encoding="utf-8"))
    out = Path(args.out).expanduser() if args.out else Path(f"{manifest['id']}-{manifest['version']}.mindpkg")
    out.parent.mkdir(parents=True, exist_ok=True)

    files = collect(root)
    if not any(str(f) == "buddy.json" for f in files):
        print("[ERROR] 目录内找不到 buddy.json", file=sys.stderr)
        return 1

    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        for rel in files:
            z.write(root / rel, rel.as_posix())

    raw = out.read_bytes()
    sha = hashlib.sha256(raw).hexdigest()
    result = {
        "ok": True,
        "package": str(out),
        "size_mb": round(len(raw) / 1048576, 2),
        "sha256": sha,
        "id": manifest["id"],
        "version": manifest["version"],
        "files": [f.as_posix() for f in files],
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    print(f"[OK] {out}（{len(files)} 个文件，{result['size_mb']} MB）", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
