#!/usr/bin/env python3
"""
softscan —— 候选技能确定性自查：frontmatter 合规、软化词扫描、体积、引文格式。
用法: python3 softscan.py <skill_dir_or_SKILL.md> [--json]
输出 JSON: {frontmatter, soft_rules, size, refs, verdict}
本脚本是质量门确定性项的唯一实现；文档只描述调用与解读。
"""
import sys, json, re, argparse
from pathlib import Path

NAME_RE = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
REF_RE = re.compile(r"\[ref:([A-Za-z0-9_\-]+)\]")
SOFT_PATTERNS = [
    (r"建议(?!\s*(?:升级|安装|使用))", "建议句式"),
    (r"可以(?:考虑|选择|参考)", "软化建议"),
    (r"根据情况", "模糊兜底"),
    (r"是可选的", "可选声明"),
]
MAX_DESC = 1024
MAX_LINES = 150


def parse_frontmatter(text: str) -> tuple[dict, list]:
    issues = []
    meta = {}
    if not text.startswith("---"):
        return meta, ["缺少 frontmatter（须以 --- 开头）"]
    parts = text.split("---", 2)
    if len(parts) < 3:
        return meta, ["frontmatter 未闭合"]
    body = parts[1]
    for field in ("name", "description", "version"):
        # 用 [ \t]* 而非 \s*（\s 含换行会跨行匹配导致取行错乱）；允许缩进：version 常嵌套在 metadata: 下
        m = re.search(rf"^[ \t]*{field}[ \t]*:", body, re.MULTILINE)
        if m:
            line_val = body[m.start():].split("\n", 1)[0].split(":", 1)[1].strip()
            if field == "description" and line_val in (">", "|", ">-", "|-"):
                # YAML 折叠/字面块：值在后续行，触发语义检查由下方 body 全文兜底
                meta[field] = "（多行块）"
            else:
                meta[field] = line_val
        else:
            issues.append(f"缺少 {field} 字段")
    name = meta.get("name", "")
    if name and not NAME_RE.match(name):
        issues.append(f"name 不符合小写连字符规范: {name}")
    desc = meta.get("description", "")
    if desc and len(desc) > MAX_DESC:
        issues.append(f"description 超 {MAX_DESC} 字符")
    # description 常为 YAML 折叠多行块（>），触发语义检查对整个 frontmatter 文本做
    if desc and "时使用" not in body and "时调用" not in body:
        issues.append("description 未说明\"何时使用\"（触发路由信号缺失）")
    return meta, issues


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("target", help="技能目录或 SKILL.md 路径")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    p = Path(args.target)
    if p.is_dir():
        p = p / "SKILL.md"
    if not p.is_file():
        print(json.dumps({"error": f"找不到 {p}"}, ensure_ascii=False))
        return 1

    text = p.read_text(encoding="utf-8")
    lines = text.split("\n")
    meta, fm_issues = parse_frontmatter(text)

    soft = []
    for i, line in enumerate(lines, 1):
        for pat, label in SOFT_PATTERNS:
            if re.search(pat, line):
                soft.append({"line": i, "label": label, "text": line.strip()[:80]})

    n_lines = len(lines)
    size_ok = n_lines <= MAX_LINES

    refs = REF_RE.findall(text)
    bad_refs = [r for r in refs if not r.startswith("ref_")]
    # 引文可达性：正文引用的 references/ 相对路径是否存在
    ref_dir = p.parent / "references"
    broken_links = []
    for m in re.finditer(r"\(([^)]+\.md)\)", text):
        link = m.group(1).replace("{base_dir}/", "").replace("./", "")
        if link.startswith("http"):
            continue
        if not (p.parent / link).is_file():
            broken_links.append(link)

    verdict_ok = not fm_issues and not bad_refs and not broken_links and size_ok
    print(json.dumps({
        "frontmatter": {"meta": meta, "issues": fm_issues},
        "soft_rules": soft,
        "size": {"lines": n_lines, "ok": size_ok, "max": MAX_LINES},
        "refs": {"count": len(refs), "bad_format": bad_refs, "broken_links": broken_links},
        "verdict": "pass" if verdict_ok else "fail",
    }, ensure_ascii=False, indent=1))
    return 0 if verdict_ok else 2


if __name__ == "__main__":
    sys.exit(main())
