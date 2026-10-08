#!/usr/bin/env python3
"""
Skill Distill 引用矩阵分析器 —— 计算指标自由度 F_approx，生成精简建议。
用法: python distill_analyzer.py <skill_dir> [--output <report.json>] [--diagnostics <path>]
      --output: 报告输出路径，默认 skill_dir/distill_report.json
      --diagnostics: diagnostics.tsv 路径，默认 skill_dir/diagnostics.tsv
输出: JSON，包含每个 references 文件的 F_approx 评分和精简建议。
"""
import sys, json, re, os, argparse
from pathlib import Path
from datetime import datetime, timezone, timedelta

CST = timezone(timedelta(hours=8))

try:
    from luban_common import parse_tsv
except ImportError:
    def parse_tsv(tsv_path: Path) -> list:
        records = []
        if not tsv_path.exists():
            return records
        lines = tsv_path.read_text(encoding="utf-8").strip().split("\n")
        if len(lines) < 2:
            return records
        headers = lines[0].split("\t")
        for line in lines[1:]:
            fields = line.split("\t")
            if len(fields) >= len(headers):
                records.append(dict(zip(headers, fields)))
        return records


def extract_ref_citations(skill_md_text: str, ref_dir: Path) -> dict[str, int]:
    citations: dict[str, int] = {}
    if not ref_dir.exists():
        return citations
    for ref_file in ref_dir.rglob("*"):
        if not ref_file.is_file():
            continue
        rel = str(ref_file.relative_to(ref_dir)).replace("\\", "/")
        citations[rel] = 0
        patterns = [
            rf"references/{re.escape(rel)}",
            rf"\[.*?\]\(.*?{re.escape(rel)}.*?\)",
        ]
        for p in patterns:
            citations[rel] += len(re.findall(p, skill_md_text))
    return citations


def compute_f_approx(citations: dict[str, int], ref_dir: Path) -> list[dict]:
    if not ref_dir.exists():
        return []

    file_sizes: dict[str, int] = {}
    for ref_file in ref_dir.rglob("*"):
        if ref_file.is_file():
            rel = str(ref_file.relative_to(ref_dir)).replace("\\", "/")
            file_sizes[rel] = len(ref_file.read_text(encoding="utf-8"))

    if not file_sizes:
        return []

    mean_size = sum(file_sizes.values()) / len(file_sizes) if file_sizes else 1
    mean_size = max(mean_size, 100)
    results = []

    for rel, size in sorted(file_sizes.items(), key=lambda x: x[1], reverse=True):
        normalized_size = size / mean_size if mean_size > 0 else 1.0
        cite_count = citations.get(rel, 0)
        f_approx = round(1.0 - (cite_count / (normalized_size + 1e-6)), 3)
        f_approx = max(0.0, min(1.0, f_approx))

        priority = ""
        if f_approx >= 0.7:
            priority = "P0-可精简"
        elif f_approx <= 0.3:
            priority = "核心资产-保留"
        else:
            priority = "正常"

        size_saving_estimate = size

        results.append({
            "file": rel,
            "size_chars": size,
            "normalized_size": round(normalized_size, 3),
            "citations": cite_count,
            "f_approx": f_approx,
            "priority": priority,
            "size_saving_estimate": size_saving_estimate,
        })

    return results


def write_diagnostics(results: list[dict], diag_path: str):
    os.makedirs(os.path.dirname(diag_path), exist_ok=True)
    mode = "a" if os.path.exists(diag_path) else "w"
    with open(diag_path, mode, encoding="utf-8") as f:
        if mode == "w":
            f.write("file\tf_approx\tpriority\tsize_chars\tsize_saving\ttimestamp\n")
        for r in results:
            ts = datetime.now(tz=CST).strftime("%Y-%m-%d %H:%M:%S")
            f.write(f"{r['file']}\t{r['f_approx']}\t{r['priority']}\t"
                    f"{r['size_chars']}\t{r['size_saving_estimate']}\t{ts}\n")


def main():
    parser = argparse.ArgumentParser(description="Skill Distill 引用矩阵分析器")
    parser.add_argument("skill_dir", help="目标 skill 目录路径")
    parser.add_argument("--output", default=None, help="报告输出路径，默认 skill_dir/distill_report.json")
    parser.add_argument("--diagnostics", default=None, help="diagnostics.tsv 路径，默认 skill_dir/diagnostics.tsv")
    args = parser.parse_args()

    sd = Path(args.skill_dir).resolve()
    sm = sd / "SKILL.md"
    ref_dir = sd / "references"

    if not sm.exists():
        print(json.dumps({"error": f"SKILL.md 不存在: {sm}"}, ensure_ascii=False))
        sys.exit(1)

    text = sm.read_text(encoding="utf-8")
    citations = extract_ref_citations(text, ref_dir)
    results = compute_f_approx(citations, ref_dir)

    total_size = sum(r["size_chars"] for r in results)
    total_saving = sum(r["size_saving_estimate"] for r in results if r["priority"] == "P0-可精简")
    p0_count = sum(1 for r in results if r["priority"] == "P0-可精简")

    report = {
        "skill_dir": str(sd),
        "total_refs": len(results),
        "total_size_chars": total_size,
        "total_size_saving_estimate": total_saving,
        "p0_count": p0_count,
        "analysis": results,
    }

    output_path = sd / "distill_report.json"
    if args.output:
        output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")

    diag_path = sd / "diagnostics.tsv"
    if args.diagnostics:
        diag_path = Path(args.diagnostics)
    write_diagnostics(results, str(diag_path))

    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
