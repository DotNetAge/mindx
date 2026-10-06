#!/usr/bin/env python3
"""
refcheck —— 引文连锁校验：扫描全部 [ref:ID] 标记，对照 references/ 目录报告断裂引用。
用法: python3 refcheck.py <agents_dir> [--json]
输出 JSON: {total, valid, broken:[{ref, file, line}], refs_dirs}
本脚本是引文校验的唯一实现；文档只描述调用与解读。
"""
import sys, json, re, argparse
from pathlib import Path

REF_RE = re.compile(r"\[ref:([A-Za-z0-9_\-]+)\]")
SCAN_DIRS = ("memory", "notes", "skills")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("agents_dir", help="项目 .agents 目录")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    agents = Path(args.agents_dir)
    refs_dir = agents / "references"

    known = set()
    if refs_dir.is_dir():
        for d in refs_dir.iterdir():
            if d.is_dir() and d.name.startswith("ref_"):
                known.add(d.name)
    index_csv = refs_dir / "index.csv"
    if index_csv.is_file():
        for line in index_csv.read_text(encoding="utf-8").splitlines()[1:]:
            first = line.split(",")[0].strip()
            if first.startswith("ref_"):
                known.add(first)

    total, valid, broken = 0, 0, []
    scanned = 0
    for sub in SCAN_DIRS:
        base = agents / sub
        if not base.is_dir():
            continue
        for f in base.rglob("*.md"):
            if ".archive" in f.parts:
                continue
            scanned += 1
            try:
                text = f.read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            for i, line in enumerate(text.split("\n"), 1):
                for ref in REF_RE.findall(line):
                    total += 1
                    if ref in known:
                        valid += 1
                    else:
                        broken.append({"ref": ref, "file": str(f.relative_to(agents)), "line": i})

    print(json.dumps({
        "total": total, "valid": valid, "broken": broken,
        "refs_dirs": sorted(known), "files_scanned": scanned,
    }, ensure_ascii=False, indent=1))
    return 0 if not broken else 2


if __name__ == "__main__":
    sys.exit(main())
