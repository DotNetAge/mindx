#!/usr/bin/env python3
"""
fapprox —— 记忆精简先验：计算 .agents/memory/ 各条目的引用密度 F_approx。
用法: python3 fapprox.py <agents_dir> [--json]
F ≈ 1 - (引用次数 / 字符数归一化)。verdict 判读：
  P0 未被引用 → 建议删除 | P1 高F大文件 → 提取引用段后精简
  P2 中间态 → 暂不处理 | P3 低F（≤0.3）→ 核心资产保留
本脚本是 F_approx 算法与阈值的唯一实现；文档只描述调用与判读。
"""
import sys, json, argparse
from pathlib import Path

TEXT_EXTS = {".md", ".json", ".csv", ".py", ".sh", ".yaml", ".yml"}
SCAN_DIRS = ("memory", "skills", "notes", "references")


def count_cites(stem: str, roots: list[Path], self_path: Path) -> int:
    """统计条目名（去扩展名）在其他文本文件中的出现次数（排除自身）。"""
    n = 0
    for root in roots:
        if not root.exists():
            continue
        for f in root.rglob("*"):
            if not f.is_file() or f.suffix not in TEXT_EXTS or f == self_path:
                continue
            try:
                n += f.read_text(encoding="utf-8").count(stem)
            except (UnicodeDecodeError, OSError):
                pass
    return n


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("agents_dir", help="项目 .agents 目录")
    ap.add_argument("--json", action="store_true", help="输出 JSON（默认就是 JSON，占位保持兼容）")
    args = ap.parse_args()
    agents = Path(args.agents_dir)
    mem_dir = agents / "memory"
    if not mem_dir.is_dir():
        print(json.dumps({"error": f"memory 目录不存在: {mem_dir}", "entries": []}, ensure_ascii=False))
        return 1

    roots = [agents / d for d in SCAN_DIRS]
    files = [f for f in mem_dir.rglob("*.md") if f.is_file() and ".archive" not in f.parts]

    sizes = {f: f.stat().st_size for f in files}
    mean_size = max(sum(sizes.values()) / len(sizes), 100) if sizes else 1

    out = []
    for f in sorted(files, key=lambda x: -sizes[x]):
        stem = f.stem
        size = sizes[f]
        norm = size / mean_size
        cites = count_cites(stem, roots, f)
        f_approx = 1 - (cites / norm if norm > 0 else cites)
        if cites == 0:
            verdict = "P0-未引用"
        elif f_approx >= 0.7:
            verdict = "P1-可精简"
        elif f_approx <= 0.3:
            verdict = "P3-核心"
        else:
            verdict = "P2-暂不处理"
        out.append({"file": str(f.relative_to(agents)), "cites": cites, "size": size,
                    "f": round(f_approx, 3), "verdict": verdict})

    print(json.dumps({"entries": out, "count": len(out)}, ensure_ascii=False, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
