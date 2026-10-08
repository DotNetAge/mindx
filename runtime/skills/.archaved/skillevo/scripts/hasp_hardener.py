#!/usr/bin/env python3
"""
HASP 规则硬化器 —— 分析执行日志，识别被忽略的软规则，生成 hardening 建议。
用法: python hasp_hardener.py <skill_dir> [--results <results.tsv>] [--log <execution_log.json>] [--output <report.json>] [--apply] [--dry-run]
      --results: results.tsv 路径（用于提取违规历史），默认 skill_dir/results.tsv
      --log: 执行日志 JSON 路径
      --output: 报告输出路径，默认 skill_dir/hardening_report.json
      --apply: 将硬化建议实际写入 SKILL.md（含备份）
      --dry-run: 模拟 apply，只报告不写入
输出: JSON 硬化建议报告。
"""
import sys, json, re, argparse
from pathlib import Path
from datetime import datetime, timezone, timedelta

try:
    from luban_common import parse_frontmatter, extract_triggers, parse_tsv
except ImportError:
    def parse_frontmatter(text: str) -> dict:
        if not text.startswith("---"):
            return {}
        parts = text.split("---", 2)
        if len(parts) < 3:
            return {}
        yb = parts[1].strip()
        result = {}
        for field in ["name", "description"]:
            m = re.search(rf"^{field}\s*:\s*(.+)", yb, re.MULTILINE)
            if m:
                result[field] = m.group(1).strip()
        return result

    def extract_triggers(text: str, max_len: int = 20) -> set:
        triggers = set()
        desc_match = re.search(r"^description\s*:\s*(.+)", text, re.MULTILINE)
        if desc_match:
            desc = desc_match.group(1)
            for m in re.finditer(r"['\"「『]([^'\"」』]+)['\"」』]", desc):
                word = m.group(1).strip()
                if word and len(word) <= max_len:
                    triggers.add(word)
            for part in desc.split(","):
                part = part.strip().strip("'\"\\").strip()
                if part and len(part) <= max_len and not part.startswith(("触发", "如", "例", "当")):
                    triggers.add(part)
        for m in re.finditer(r"(?:触发|关键词)[：:]\s*(.+)", text):
            for word in re.split(r"[,，、/]", m.group(1)):
                word = word.strip().strip("'\"").strip()
                if word and len(word) <= 30:
                    triggers.add(word)
        return triggers

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


try:
    import jieba
    _JIEBA_AVAILABLE = True
except ImportError:
    _JIEBA_AVAILABLE = False

CST = timezone(timedelta(hours=8))

SOFT_RULE_PATTERNS = [
    (r"建议(?!\s*(?:升级|安装|使用))", "建议句式"),
    (r"可以(?:考虑|选择|参考)", "软化建议"),
    (r"根据情况", "模糊兜底"),
    (r"是可选的", "可选声明"),
    (r"视情况而定", "弹性策略"),
    (r"(?:通常|一般|大多数)情况下", "非强制条件"),
    (r"如果(?:需要|必要|方便)的话", "条件软化"),
]


def extract_soft_rules(skill_text: str) -> list[dict]:
    rules = []
    for lineno, line in enumerate(skill_text.split("\n"), 1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or stripped.startswith("```"):
            continue
        for pattern, rule_type in SOFT_RULE_PATTERNS:
            if re.search(pattern, stripped):
                rules.append({
                    "line": lineno,
                    "text": stripped[:200],
                    "rule_type": rule_type,
                    "hardenable": _check_hardenable(stripped),
                })
                break
    return rules


def _check_hardenable(text: str) -> bool:
    if re.search(r"(?:输出|格式|章节|字段|文件大小|触发词|必须包含|强制|限制|≤|>=|<|>)", text):
        return True
    if re.search(r"(?:风格|创意|灵感|审美|感受|偏好)", text):
        return False
    return True


def extract_hard_rules(skill_text: str) -> list[dict]:
    fm_match = re.search(r"^---\s*\n(.*?)\n---", skill_text, re.DOTALL)
    if not fm_match:
        return []
    frontmatter = fm_match.group(1)
    rules = []
    hr_match = re.search(r"hard_rules\s*:\s*\n((?:\s+-.+\n?)+)", frontmatter)
    if hr_match:
        block = hr_match.group(1)
        for m in re.finditer(r"-\s*id\s*:\s*(.+?)\n(?:\s*should_activate\s*:\s*(.+?)\n)?", block):
            rules.append({"id": m.group(1).strip(), "should_activate": (m.group(2) or "").strip()})
    return rules


def find_violations(skill_text: str, results: list[dict], log_data: dict | None) -> list[dict]:
    violations = []
    soft_rules = extract_soft_rules(skill_text)

    for rec in results:
        status = rec.get("status", "").lower()
        if status in ("revert", "rollback", "failed", "failure"):
            note = rec.get("note", "")
            for rule in soft_rules:
                rule_kw = _tokenize(rule["text"])
                note_kw = _tokenize(note)
                if len(rule_kw & note_kw) >= 2:
                    violations.append({
                        "rule_line": rule["line"],
                        "rule_text": rule["text"][:120],
                        "violated_at": rec.get("timestamp", ""),
                        "context": note[:120],
                        "violation_count": 1,
                    })
                    break

    if log_data:
        log_entries = log_data.get("entries", log_data.get("failures", []))
        for entry in log_entries:
            entry_text = entry.get("message", entry.get("error", str(entry)))
            for rule in soft_rules:
                if _text_overlap(rule["text"], entry_text):
                    violations.append({
                        "rule_line": rule["line"],
                        "rule_text": rule["text"][:120],
                        "violated_at": entry.get("timestamp", ""),
                        "context": entry_text[:120],
                        "violation_count": 1,
                    })

    merged: dict[int, dict] = {}
    for v in violations:
        line = v["rule_line"]
        if line in merged:
            merged[line]["violation_count"] += 1
        else:
            merged[line] = v
    return list(merged.values())


def find_score_gaps(results: list[dict]) -> list[dict]:
    gaps = []
    for rec in results:
        status = rec.get("status", "").lower()
        if status not in ("fail", "revert"):
            continue
        note = rec.get("note", "")
        dim = rec.get("dimension", "")
        tid = rec.get("test_id", "")

        m = re.search(r"触发词 '(.+?)' 未在SKILL\.md中找到", note)
        if m:
            gaps.append({"type": "missing_trigger", "test_id": tid, "dimension": dim,
                         "trigger_word": m.group(1), "note": note})
            continue

        m = re.search(r"格式约束 '(.+?)' 未在SKILL\.md中体现", note)
        if m:
            gaps.append({"type": "missing_format", "test_id": tid, "dimension": dim,
                         "constraint": m.group(1), "note": note})
            continue

        m = re.search(r"规则关键词仅匹配", note)
        if m:
            gaps.append({"type": "weak_rule", "test_id": tid, "dimension": dim,
                         "note": note, "auto_fixable": False})
            continue

        m = re.search(r"引用文件 (.+?) 不存在", note)
        if m:
            gaps.append({"type": "missing_ref", "test_id": tid, "dimension": dim,
                         "file": m.group(1), "note": note, "auto_fixable": False})
            continue

        gaps.append({"type": "unknown", "test_id": tid, "dimension": dim,
                     "note": note, "auto_fixable": False})

    return gaps


def apply_score_gaps(skill_path: Path, gaps: list[dict], dry_run: bool = False) -> dict:
    text = skill_path.read_text(encoding="utf-8")
    fixed_triggers = 0
    fixed_formats = 0

    for gap in gaps:
        if gap["type"] == "missing_trigger" and gap.get("auto_fixable", True):
            tw = gap["trigger_word"]
            if len(tw) > 30 or "无关" in tw or "反" in gap.get("test_id", ""):
                continue
            if tw in text:
                continue
            desc_match = re.search(r'^description\s*:\s*"(.+)"\s*$', text, re.MULTILINE)
            if desc_match:
                inner = desc_match.group(1)
                new_inner = inner.rstrip() + '，"' + tw + '"'
                text = text.replace(desc_match.group(0), f'description: "{new_inner}"', 1)
                fixed_triggers += 1

        elif gap["type"] == "missing_format" and gap.get("auto_fixable", True):
            constraint = gap["constraint"]
            if constraint not in text:
                text = text.rstrip() + f"\n\n## 输出格式\n- {constraint}\n"
                fixed_formats += 1

    if not dry_run and (fixed_triggers or fixed_formats):
        backup = skill_path.with_suffix(skill_path.suffix + ".gap.bak")
        backup.write_text(skill_path.read_text(encoding="utf-8"), encoding="utf-8")
        skill_path.write_text(text, encoding="utf-8")

    return {"fixed_triggers": fixed_triggers, "fixed_formats": fixed_formats,
            "total_fixable": sum(1 for g in gaps if g.get("auto_fixable", True))}


def _tokenize(text: str) -> set[str]:
    if _JIEBA_AVAILABLE:
        words = jieba.lcut(text)
        return {w.strip() for w in words if len(w.strip()) >= 2 and not re.fullmatch(r"[\s\d\W_]+", w)}
    return set(re.findall(r"[\u4e00-\u9fff]{2,6}", text))


def _text_overlap(a: str, b: str, min_overlap: int = 2) -> bool:
    return len(_tokenize(a) & _tokenize(b)) >= min_overlap


def generate_hardening(skill_text: str, violations: list[dict]) -> list[dict]:
    suggestions = []
    soft_rules = extract_soft_rules(skill_text)

    for rule in soft_rules:
        if not rule["hardenable"]:
            continue
        count = 0
        for v in violations:
            if v["rule_line"] == rule["line"]:
                count = v.get("violation_count", 1)
                break

        tier = ""
        if count == 0:
            tier = "T0-基线"
        elif count <= 2:
            tier = "T1-措辞强化"
        else:
            tier = "T2-PF硬化"

        if tier == "T0-基线":
            continue

        sug = {
            "rule_line": rule["line"],
            "rule_text": rule["text"][:150],
            "rule_type": rule["rule_type"],
            "violation_count": count,
            "tier": tier,
            "hardenable": rule["hardenable"],
        }

        if tier == "T1-措辞强化":
            hardened = rule["text"]
            hardened = re.sub(r"建议", "必须", hardened)
            hardened = re.sub(r"可以(?:考虑|选择|参考)", "必须", hardened)
            hardened = re.sub(r"根据情况", "明确按以下步骤", hardened)
            hardened = re.sub(r"是可选的", "是强制必需的", hardened)
            sug["hardened_text"] = hardened[:200]
            sug["action"] = "replace_soft_with_must"
            sug["instruction"] = f"行 {rule['line']} 替换为强制规则"

        elif tier == "T2-PF硬化":
            rule_id = f"rule_{rule['line']:03d}"
            condition = _extract_activating_condition(rule["text"])
            sug["pf_definition"] = {
                "id": rule_id,
                "should_activate": condition,
                "intervene": {"type": "block_and_warn",
                              "action": f"违反规则: {rule['text'][:80]}"},
                "severity": "high" if count >= 5 else "medium",
                "last_violated": datetime.now(tz=CST).strftime("%Y-%m-%d"),
                "violation_count": count,
            }
            sug["action"] = "inject_pf_rule"
            sug["instruction"] = f"frontmatter hard_rules 中注入 PF 规则 {rule_id}"
            sug["hardened_text"] = f"强制: {rule['text'][:150]}"

        suggestions.append(sug)

    suggestions.sort(key=lambda x: x["violation_count"], reverse=True)
    return suggestions


def _extract_activating_condition(text: str) -> str:
    cond = re.search(r"(?:当|如果|若)(.+?)(?:时|，|,|则|应)", text)
    if cond:
        return cond.group(1).strip()[:100]
    cond2 = re.search(r"(?:在|遇到|出现)(.+?)(?:时|场景|情况)", text)
    if cond2:
        return cond2.group(1).strip()[:100]
    return "相关场景发生"


def apply_hardening(skill_path: Path, suggestions: list[dict], dry_run: bool = False) -> dict:
    """将硬化建议写入 SKILL.md

    P0 修复要点：
    - T2 PF 注入使用正则重建 frontmatter 块，避免行号索引错位。
    - 新建 hard_rules 时在闭合 --- 之前插入（而非之后）。
    - backup 变量在 dry_run 时也正确定义（外层初始化）。
    """
    text = skill_path.read_text(encoding="utf-8")
    lines = text.split("\n")
    applied_t1 = 0
    applied_t2 = 0
    pf_defs = []
    backup = None  # P0: 外层初始化，避免 dry_run=True 时 NameError

    for sug in suggestions:
        if sug["tier"] == "T1-措辞强化":
            line_no = sug["rule_line"] - 1
            if 0 <= line_no < len(lines):
                old = lines[line_no]
                new = old
                new = re.sub(r"建议", "必须", new)
                new = re.sub(r"可以(?:考虑|选择|参考)", "必须", new)
                new = re.sub(r"根据情况", "明确按以下步骤", new)
                if new != old:
                    lines[line_no] = new
                    applied_t1 += 1

        elif sug["tier"] == "T2-PF硬化":
            pf = sug.get("pf_definition", {})
            if pf:
                pf_defs.append(pf)
                applied_t2 += 1

    if pf_defs:
        fm_match = re.search(r"^(---\s*\n)(.*?)(\n---)", text, re.DOTALL)
        if fm_match:
            fm_body = fm_match.group(2)
            hr_match = re.search(r"hard_rules\s*:\s*\n", fm_body)

            pf_yaml_lines = []
            if not hr_match:
                pf_yaml_lines.append("hard_rules:")
            for pf in pf_defs:
                pf_yaml_lines.append(f"  - id: {pf['id']}")
                pf_yaml_lines.append(f"    should_activate: {pf['should_activate']}")
                pf_yaml_lines.append(f"    intervene:")
                pf_yaml_lines.append(f"      type: {pf['intervene']['type']}")
                pf_yaml_lines.append(f"      action: \"{pf['intervene']['action']}\"")
                pf_yaml_lines.append(f"    severity: {pf['severity']}")
                pf_yaml_lines.append(f"    last_violated: {pf.get('last_violated','')}")
                pf_yaml_lines.append(f"    violation_count: {pf.get('violation_count',0)}")

            pf_block = "\n".join(pf_yaml_lines)

            if hr_match:
                new_fm_body = fm_body[:hr_match.end()] + pf_block[len("hard_rules:"):] + "\n" + fm_body[hr_match.end():]
            else:
                new_fm_body = fm_body.rstrip() + "\n" + pf_block

            text = fm_match.group(1) + new_fm_body + "\n---" + text[fm_match.end(3):]

    if not dry_run:
        backup = skill_path.with_suffix(skill_path.suffix + ".bak")
        backup.write_text(skill_path.read_text(encoding="utf-8"), encoding="utf-8")
        skill_path.write_text(text, encoding="utf-8")

    return {
        "applied_t1": applied_t1,
        "applied_t2": applied_t2,
        "backup": str(backup) if backup else None,
        "applied_lines": [s["rule_line"] for s in suggestions if s["tier"] in ("T1-措辞强化", "T2-PF硬化")],
    }


def main():
    parser = argparse.ArgumentParser(description="HASP 规则硬化器")
    parser.add_argument("skill_dir", help="目标 skill 目录路径")
    parser.add_argument("--results", default=None, help="results.tsv 路径，默认 skill_dir/results.tsv")
    parser.add_argument("--log", default=None, help="执行日志 JSON 路径")
    parser.add_argument("--output", default=None, help="报告输出路径，默认 skill_dir/hardening_report.json")
    parser.add_argument("--apply", action="store_true", help="将硬化建议实际写入 SKILL.md（含备份）")
    parser.add_argument("--dry-run", action="store_true", help="模拟 apply，只报告不写入")
    args = parser.parse_args()

    sd = Path(args.skill_dir).resolve()
    sm = sd / "SKILL.md"

    if not sm.exists():
        print(json.dumps({"error": f"SKILL.md 不存在: {sm}"}, ensure_ascii=False))
        sys.exit(1)

    skill_text = sm.read_text(encoding="utf-8")
    soft_rules = extract_soft_rules(skill_text)
    existing_hard = extract_hard_rules(skill_text)

    results = []
    log_data = None

    if args.results:
        results = parse_tsv(Path(args.results))
    else:
        default_tsv = sd / "results.tsv"
        results = parse_tsv(default_tsv)

    if args.log:
        log_path = Path(args.log)
        if log_path.exists():
            log_data = json.loads(log_path.read_text(encoding="utf-8"))

    violations = find_violations(skill_text, results, log_data)
    suggestions = generate_hardening(skill_text, violations)

    t1_count = sum(1 for s in suggestions if s["tier"] == "T1-措辞强化")
    t2_count = sum(1 for s in suggestions if s["tier"] == "T2-PF硬化")

    report = {
        "skill_dir": str(sd),
        "skill_name": sm.stem,
        "scanned_at": datetime.now(tz=CST).isoformat(),
        "summary": {
            "total_soft_rules": len(soft_rules),
            "hardening_candidates": len(suggestions),
            "t1_wording_strengthen": t1_count,
            "t2_pf_injection": t2_count,
            "existing_hard_rules": len(existing_hard),
            "total_violations_found": len(violations),
        },
        "soft_rules": soft_rules,
        "existing_hard_rules": existing_hard,
        "violations": violations,
        "suggestions": suggestions,
    }

    output_path = sd / "hardening_report.json"
    if args.output:
        output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")

    if args.apply or args.dry_run:
        dry = args.dry_run
        score_gaps = find_score_gaps(results)
        report["score_gaps"] = score_gaps
        gap_fix = apply_score_gaps(sm, score_gaps, dry_run=dry)
        report["gap_fix"] = gap_fix
        apply_result = apply_hardening(sm, suggestions, dry_run=dry)
        report["apply"] = apply_result
        output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")

    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
