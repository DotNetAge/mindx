#!/usr/bin/env python3
"""
EvoSkill 失败驱动技能修补器 —— 捕获失败上下文、分类缺口、生成补丁建议。
用法: python evo_skill_patcher.py <skill_dir> [--context <json_file>]
      --context: 失败上下文 JSON 文件路径；不传则从 stdin 读取。
输出: JSON 补丁建议。
"""
import sys, json, re, argparse
from pathlib import Path

try:
    from luban_common import extract_triggers
except ImportError:
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


def analyze_trigger_gap(skill_text: str, user_input: str) -> dict | None:
    triggers = extract_triggers(skill_text)
    if not triggers:
        return None
    user_lower = user_input.lower()
    for tw in triggers:
        if tw.lower() in user_lower:
            return None
    user_keywords = re.findall(r"[\u4e00-\u9fff]{2,8}|[a-zA-Z]{3,15}", user_input)
    uncovered = [w for w in user_keywords if not any(w in t for t in triggers)]
    if not uncovered:
        return None
    return {
        "gap_type": "触发词遗漏",
        "uncovered_keywords": uncovered[:5],
        "suggestion": f"在 SKILL.md description 中追加触发词: {', '.join(uncovered[:5])}",
    }


def analyze_rule_gap(skill_text: str, user_input: str) -> dict | None:
    sections = re.findall(r"^(#{1,4}\s+.+)$", skill_text, re.MULTILINE)
    user_topics = re.findall(r"[\u4e00-\u9fff]{2,10}", user_input)
    covered = any(any(t in s for t in user_topics) for s in sections)
    if covered:
        return None
    return {
        "gap_type": "规则缺失/模糊",
        "missing_topic": user_topics[:3] if user_topics else ["未知"],
        "suggestion": f"在 SKILL.md 中新增章节覆盖场景: {user_input[:80]}",
    }


def analyze_conflict(skill_text: str) -> list[dict]:
    conflicts = []
    must_lines = []
    should_lines = []
    for lineno, line in enumerate(skill_text.split("\n"), 1):
        stripped = line.strip()
        if "必须" in stripped:
            must_lines.append((lineno, stripped))
        if re.search(r"(?:建议|可以|考虑)", stripped) and "强制" not in stripped:
            should_lines.append((lineno, stripped))
    for ml_lineno, ml_text in must_lines:
        ml_keywords = set(re.findall(r"[\u4e00-\u9fff]{2,6}", ml_text))
        for sl_lineno, sl_text in should_lines:
            sl_keywords = set(re.findall(r"[\u4e00-\u9fff]{2,6}", sl_text))
            overlap = ml_keywords & sl_keywords
            if len(overlap) >= 2 and abs(ml_lineno - sl_lineno) < 200:
                conflicts.append({
                    "gap_type": "指令冲突",
                    "must_line": ml_lineno,
                    "must_text": ml_text[:120],
                    "should_line": sl_lineno,
                    "should_text": sl_text[:120],
                    "shared_keywords": list(overlap)[:5],
                    "suggestion": f"行 {ml_lineno} 用'必须'，行 {sl_lineno} 用'建议'，主题重叠需统一措辞",
                })
    return conflicts[:5]


def analyze_flow_gap(skill_text: str) -> list[dict]:
    gaps = []
    lines = skill_text.split("\n")
    step_chunks: list[tuple[int, str]] = []
    current_step = None
    current_lines: list[str] = []
    for lineno, line in enumerate(lines, 1):
        if re.match(r"^(?:Step\s*\d+|步骤\s*\d+)", line, re.IGNORECASE):
            if current_step is not None:
                step_chunks.append((current_step, "\n".join(current_lines)))
            current_step = lineno
            current_lines = [line]
        elif re.match(r"^#{1,4}\s", line) and current_step is not None:
            step_chunks.append((current_step, "\n".join(current_lines)))
            current_step = None
            current_lines = []
        elif current_step is not None:
            current_lines.append(line)
    if current_step is not None:
        step_chunks.append((current_step, "\n".join(current_lines)))

    for i, (start_line, block) in enumerate(step_chunks):
        has_action = bool(re.search(r"(?:执行|运行|调用|检查|扫描|计算|生成)", block, re.IGNORECASE))
        if not has_action and len(block) > 30:
            gaps.append({
                "gap_type": "流程漏洞",
                "step": i + 1,
                "issue": "步骤缺少明确动作动词",
                "context": block.strip()[:120],
                "suggestion": f"步骤 {i+1} 需补充具体操作动作",
            })
    return gaps[:3]


def analyze_output_format(skill_text: str) -> list[dict]:
    issues = []
    has_table = "表格" in skill_text or "|" in skill_text
    has_code = "```" in skill_text
    has_json = "json" in skill_text.lower()
    has_structured = has_table or has_code or has_json
    if not has_structured:
        issues.append({
            "gap_type": "输出格式不当",
            "issue": "SKILL.md 未定义结构化输出格式（表格/代码块/JSON）",
            "suggestion": "在输出格式章节补充 Markdown 表格或代码块示例",
        })
    return issues


def analyze_version_compat(skill_text: str) -> list[dict]:
    deps = []
    dep_patterns = [
        r"(?:需要|依赖|安装|pip install|npm install|>=)\s*([\w\-\.]+)",
        r"arXiv[:\s]*(\d{4}\.\d{4,5}(?:v\d+)?)",
        r"(?:API|v)\s*(\d+\.\d+)",
    ]
    for pattern in dep_patterns:
        for m in re.finditer(pattern, skill_text):
            deps.append({"dependency": m.group(1), "context": m.group(0)[:80]})
    if not deps:
        return []
    return [{
        "gap_type": "版本兼容",
        "dependencies": deps[:10],
        "suggestion": "确认上述依赖是否标注版本号，未标注则追加当前版本",
    }]


def generate_patch(skill_dir: Path, gap: dict) -> dict:
    sm = skill_dir / "SKILL.md"
    patch = {
        "file": str(sm),
        "gap_type": gap.get("gap_type", "未知"),
    }

    if gap["gap_type"] == "触发词遗漏":
        kw = gap.get("uncovered_keywords", [])
        patch["location"] = "SKILL.md frontmatter description 字段"
        patch["old_str"] = "description: "
        patch["new_str"] = f"description: ...追加触发词: {', '.join(kw)}"
        patch["reason"] = f"用户输入含关键词 {kw} 但技能未覆盖，触发词遗漏可能导致未触发"

    elif gap["gap_type"] == "规则缺失/模糊":
        topic = gap.get("missing_topic", ["未知"])
        patch["location"] = "SKILL.md 需要新增章节"
        patch["old_str"] = ""
        patch["new_str"] = f"## 新增 {topic[0] if topic else '场景'} 处理规则\n\n[待补充具体规则]"
        patch["reason"] = "当前技能未覆盖此场景"

    elif gap["gap_type"] == "指令冲突":
        patch["location"] = f"SKILL.md 行 {gap.get('must_line', '?')} / 行 {gap.get('should_line', '?')}"
        patch["old_str"] = gap.get("should_text", "")
        patch["new_str"] = f"【统一为强制规则】{gap.get('must_text', '')}"
        patch["reason"] = f"两个规则主题重叠关键词 {gap.get('shared_keywords', [])}，措辞强度矛盾"

    elif gap["gap_type"] == "流程漏洞":
        step = gap.get("step", "?")
        patch["location"] = f"步骤 {step}"
        patch["old_str"] = gap.get("context", "")
        patch["new_str"] = f"[需要补充] 步骤 {step} 的具体操作动作 + 输入/输出定义"
        patch["reason"] = "步骤缺少明确动作动词或输入/输出描述"

    elif gap["gap_type"] == "输出格式不当":
        patch["location"] = "SKILL.md 输出格式章节"
        patch["old_str"] = ""
        patch["new_str"] = "## 输出格式\n\n```markdown\n[具体格式示例]\n```"
        patch["reason"] = "技能未定义结构化输出格式"

    elif gap["gap_type"] == "版本兼容":
        deps = gap.get("dependencies", [])
        patch["location"] = "SKILL.md frontmatter 或依赖声明章节"
        patch["old_str"] = ""
        patch["new_str"] = "需要为以下依赖标注版本并做兼容性检查"
        dep_list = [d["dependency"] for d in deps[:5]]
        patch["reason"] = f"依赖 {dep_list} 未标注版本号，可能在未来版本变更时失效"

    return patch


def main():
    parser = argparse.ArgumentParser(description="EvoSkill 失败驱动技能修补器")
    parser.add_argument("skill_dir", help="目标 skill 目录路径")
    parser.add_argument("--context", default=None, help="失败上下文 JSON 文件路径；不传则从 stdin 读取")
    args = parser.parse_args()

    sd = Path(args.skill_dir).resolve()
    sm = sd / "SKILL.md"

    if not sm.exists():
        print(json.dumps({"error": f"SKILL.md 不存在: {sm}"}, ensure_ascii=False))
        sys.exit(1)

    skill_text = sm.read_text(encoding="utf-8")

    context = {}
    if args.context:
        ctx_path = Path(args.context)
        if ctx_path.exists():
            context = json.loads(ctx_path.read_text(encoding="utf-8"))
    else:
        try:
            stdin_data = sys.stdin.read().strip()
            if stdin_data:
                context = json.loads(stdin_data)
        except Exception:
            pass

    user_input = context.get("user_input", context.get("error", context.get("feedback", "")))
    error_output = context.get("output", context.get("error_output", ""))

    all_gaps: list[dict] = []

    if user_input:
        tg = analyze_trigger_gap(skill_text, user_input)
        if tg:
            all_gaps.append(tg)
        rg = analyze_rule_gap(skill_text, user_input)
        if rg:
            all_gaps.append(rg)

    conflicts = analyze_conflict(skill_text)
    all_gaps.extend(conflicts)

    flow_gaps = analyze_flow_gap(skill_text)
    all_gaps.extend(flow_gaps)

    format_issues = analyze_output_format(skill_text)
    all_gaps.extend(format_issues)

    version_issues = analyze_version_compat(skill_text)
    all_gaps.extend(version_issues)

    patches = [generate_patch(sd, g) for g in all_gaps]

    report = {
        "skill_dir": str(sd),
        "skill_name": sm.stem,
        "context": {
            "user_input": user_input[:200] if user_input else "",
            "error_output": error_output[:200] if error_output else "",
        },
        "total_gaps": len(all_gaps),
        "total_patches": len(patches),
        "gaps": all_gaps,
        "patches": patches,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
