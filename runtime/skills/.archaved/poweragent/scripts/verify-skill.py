#!/usr/bin/env python3
"""
verify-skill.py - tiangong-skill SKILL.md 结构验证脚本

读取 SKILL.md，检查以下维度并输出 JSON 报告：
  1. YAML frontmatter 完整性
  2. CHECKPOINT 数量与位置
  3. H2 标题统计
  4. 规则冲突检测（DO vs DON'T 语义交叉）
  5. 岗位型深度结构校验（11 项：身份/使命/工作流/流派/交付物/工作流程/沟通/极限行为/KPI/知识库/诚实边界）

用法：
  python verify-skill.py [--skill path/to/SKILL.md] [--quiet] [--json]
"""

import argparse
import json
import os
import re
import sys
from typing import Any

import yaml

# 共享工具函数
from _common import (
    count_table_rows as _count_table_rows,
    count_list_items as _count_list_items,
    count_numbered_steps as _count_numbered_steps,
    count_code_blocks as _count_code_blocks,
    extract_h2_sections,
    get_section as _get_section,
)

# ─────────────────────────────────────────────────────────
# 辅助函数
# ─────────────────────────────────────────────────────────


def extract_frontmatter(text):
    if not text.startswith("---"):
        return None, "", 0
    lines = text.split("\n")
    end_idx = -1
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            end_idx = i
            break
    if end_idx == -1:
        return None, "", 0
    raw = "\n".join(lines[1:end_idx])
    parsed = yaml.safe_load(raw) or {}
    return parsed, raw, end_idx


def count_checkpoints(text):
    lines = text.split("\n")
    total = 0
    positions = []
    for i, line in enumerate(lines, 1):
        if re.search(r"CHECKPOINT", line, re.IGNORECASE):
            total += 1
            positions.append(i)
    return {"total_occurrences": total, "line_numbers": positions}


def count_h2_headings(text):
    lines = text.split("\n")
    headings = []
    for line in lines:
        if re.match(r"^##\s+(?!#)", line):
            headings.append(line.strip())
    stage_headings = [h for h in headings if re.search("阶段|阶段一|阶段二|阶段三|阶段四", h)]
    design_headings = [h for h in headings if re.search("设计模式|人物蒸馏|岗位型", h)]
    utility_headings = [h for h in headings if re.search("反膨胀|规则冲突|迭代|回测|文件结构|核心模块|激活|技能类型|失败模式|快速诊断", h)]
    return {
        "total": len(headings),
        "stage_related": len(stage_headings),
        "design_related": len(design_headings),
        "utility_related": len(utility_headings),
        "headings": headings,
    }


# ── Rule conflict helpers (shared) ──

_CONFLICT_KW_PAIRS = [
    # Action-level: DO requires X, DON'T prohibits X
    ("搜索", "搜索"), ("读取", "读取"), ("输出", "输出"), ("生成", "生成"),
    ("调用", "调用"), ("访问", "访问"), ("修改", "修改"), ("删除", "删除"),
    ("执行", "执行"), ("加载", "加载"), ("主动", "主动"), ("自动", "自动"),
    ("直接", "直接"), ("拒绝", "拒绝"), ("忽略", "忽略"),
    ("所有", "所有"), ("全部", "全部"),
    # Quality-level: contradictory requirements
    ("详细", "简洁"), ("完整", "简洁"), ("全面", "简洁"),
    ("详细", "精简"), ("完整", "精简"), ("详细", "简短"),
    ("详细", "概括"), ("完整", "概括"),
]

_NEGATION_WORDS = ["禁止", "绝不", "严禁", "不能", "不得", "不应", "不要", "不可", "不许", "切勿"]

_ACTION_VERB_RE = re.compile(
    r'(要求|必须|应当|应该|务必|确保|保证|提供|展示|说明|解释|搜索|读取|输出|'
    r'生成|创建|调用|访问|修改|删除|执行|加载|分析|处理|检查|验证|确认|'
    r'询问|回复|回答|拒绝|忽略|跳过|保留|记录|保存|发送|接收|'
    r'使用|引用|标注|声明|遵循|遵守|限制|约束)'
)

_TARGET_NOUN_RE = re.compile(
    r'(数据|信息|文件|文档|内容|结果|答案|回复|用户|请求|指令|命令|'
    r'外部|内部|网络|系统|工具|API|资源|来源|引用|参考|模板|'
    r'格式|标准|规范|规则|流程|步骤|操作)'
)


def _extract_verbs(text):
    return set(re.findall(_ACTION_VERB_RE, text))


def _extract_nouns(text):
    return set(re.findall(_TARGET_NOUN_RE, text))


def _has_negation(text):
    return any(w in text for w in _NEGATION_WORDS)


def _is_contradictory_pair(verb_a, verb_b):
    contradictory_pairs = {
        frozenset(["详细", "简洁"]), frozenset(["详细", "简短"]),
        frozenset(["完整", "简洁"]), frozenset(["全面", "简洁"]),
        frozenset(["详细", "概括"]), frozenset(["完整", "概括"]),
        frozenset(["详细", "精简"]), frozenset(["完整", "精简"]),
        frozenset(["生成", "限制"]), frozenset(["输出", "限制"]),
        frozenset(["执行", "跳过"]), frozenset(["处理", "忽略"]),
        frozenset(["提供", "限制"]), frozenset(["使用", "禁止"]),
    }
    return frozenset([verb_a, verb_b]) in contradictory_pairs


def _is_prerequisite(action, possible_prereq):
    prereq_map = {
        "搜索": ["访问", "调用", "加载", "读取"],
        "读取": ["访问", "加载"],
        "生成": ["调用", "搜索", "读取", "分析"],
        "分析": ["读取", "搜索"],
        "输出": ["生成", "分析", "读取", "处理"],
        "引用": ["搜索", "读取"],
        "验证": ["读取", "检查"],
        "检查": ["读取", "访问"],
        "调用": ["访问"],
    }
    return possible_prereq in prereq_map.get(action, [])


def detect_rule_conflicts(text):
    """Full matrix cross-detection per quality-design-process.md Rule Conflict Detection.

    Phase 1: Extract all DO (D1, D2...) and DON'T (N1, N2...) rules with numbering.
    Phase 2: Quick keyword pre-screening on candidate conflict pairs.
    Phase 3: Semantic analysis (action overlap, target overlap, prerequisite chains).
    Phase 4: Report conflicts with pairing, description, severity, fix_suggestion.

    3 conflict patterns:
      - DO vs DON'T: full DxN matrix
      - DO vs DO: upper-triangle DxD
      - DON'T vs DON'T: upper-triangle NxN
    """
    conflicts = []

    # ── Phase 1: Extract numbered DO/DON'T rules ──
    do_rules = []     # [{"id":"D1","line":L,"text":"..."}, ...]
    dont_rules = []   # [{"id":"N1","line":L,"text":"..."}, ...]
    in_do_block = False
    in_dont_block = False
    for i, line in enumerate(text.split("\n"), 1):
        stripped = line.strip()

        # Phase 1: block-end detection (must run BEFORE block-start, so a
        # heading that ends one block and starts another is handled
        # correctly — first close the old block, then open the new one).
        if in_do_block and re.match(r"^(#{1,3}\s|##\s)", stripped) and "DO" not in stripped:
            in_do_block = False
            # fall through to check if this heading starts a new block
        if in_dont_block and re.match(r"^(#{1,3}\s|##\s)", stripped) and "DON" not in stripped:
            in_dont_block = False
            # fall through to check if this heading starts a new block

        # Phase 2: block-start detection
        if re.search("必须做\\(DO\\)", stripped, re.IGNORECASE):
            in_do_block = True
            in_dont_block = False  # DO heading always ends DON'T block
            continue
        if re.search("绝不做的\\(DON'?T\\)", stripped, re.IGNORECASE) or re.search("禁止做\\(DON'?T\\)", stripped, re.IGNORECASE):
            in_dont_block = True
            in_do_block = False  # DON'T heading always ends DO block
            continue

        # Phase 3: rule extraction (only one block active after correct transitions)
        if in_do_block and re.match(r"^[-*]\s+", stripped):
            do_rules.append({"id": "D{}".format(len(do_rules) + 1),
                             "line": i,
                             "text": re.sub(r"^[-*]\s+", "", stripped)})
        if in_dont_block and re.match(r"^[-*]\s+", stripped):
            dont_rules.append({"id": "N{}".format(len(dont_rules) + 1),
                               "line": i,
                               "text": re.sub(r"^[-*]\s+", "", stripped)})

    if not do_rules and not dont_rules:
        return conflicts

    # ── Phase 2+3: Full matrix cross-check ──

    # Pattern 1: DO vs DON'T — complete DxN matrix
    for d in do_rules:
        d_verbs = _extract_verbs(d["text"])
        d_nouns = _extract_nouns(d["text"])
        for n in dont_rules:
            reasons = []
            # 2a. Keyword pre-screen
            for kw_a, kw_b in _CONFLICT_KW_PAIRS:
                if kw_a in d["text"] and kw_b in n["text"] and _has_negation(n["text"]):
                    reasons.append("DO涉及「{}」但DON'T禁止相关操作".format(kw_a))
                    break  # one kw match is sufficient to flag candidate

            # 2b. Action+target overlap: same nouns targeted by same verbs in both
            n_verbs = _extract_verbs(n["text"])
            n_nouns = _extract_nouns(n["text"])
            common_nouns = d_nouns & n_nouns
            common_acts = d_verbs & n_verbs
            if common_nouns and common_acts and _has_negation(n["text"]):
                targets = "、".join(sorted(common_nouns)[:3])
                reasons.append("双方作用域交叠（{}）：DO要求执行但DON'T禁止".format(targets))

            # 2c. Prerequisite chain: DO needs X, DON'T blocks X
            for act in d_verbs:
                for n_act in n_verbs:
                    if _is_prerequisite(act, n_act) and _has_negation(n["text"]):
                        reasons.append("DO要求「{}」依赖前置步骤「{}」，但DON'T禁止该前置".format(act, n_act))

            if reasons:
                severity = "致命" if len(reasons) >= 2 else "严重"
                conflicts.append({
                    "pairing": "{}×{}".format(d["id"], n["id"]),
                    "type": "DO_vs_DONT",
                    "do_id": d["id"], "do_line": d["line"], "do_rule": d["text"],
                    "dont_id": n["id"], "dont_line": n["line"], "dont_rule": n["text"],
                    "description": "; ".join(reasons),
                    "severity": severity,
                    "fix_suggestion": "将DON'T添加例外条件，或DO补充替代路径（如优先使用本地数据/缓存）",
                })

    # Pattern 2: DO vs DO — upper-triangle DxD
    for i in range(len(do_rules)):
        d1 = do_rules[i]
        d1_verbs = _extract_verbs(d1["text"])
        d1_nouns = _extract_nouns(d1["text"])
        for j in range(i + 1, len(do_rules)):
            d2 = do_rules[j]
            reasons = []
            # 2a. Keyword-level contradiction
            for kw_a, kw_b in _CONFLICT_KW_PAIRS:
                if kw_a in d1["text"] and kw_b in d2["text"]:
                    reasons.append("DO要求「{}」与另一DO要求「{}」矛盾".format(kw_a, kw_b))
                    break
                if kw_b in d1["text"] and kw_a in d2["text"]:
                    reasons.append("DO要求「{}」与另一DO要求「{}」矛盾".format(kw_b, kw_a))
                    break

            # 2b. Verb-level: contradictory pair on same target
            d2_verbs = _extract_verbs(d2["text"])
            d2_nouns = _extract_nouns(d2["text"])
            common_nouns = d1_nouns & d2_nouns
            if common_nouns:
                for v1 in d1_verbs:
                    for v2 in d2_verbs:
                        if _is_contradictory_pair(v1, v2):
                            targets = "、".join(sorted(common_nouns)[:2])
                            reasons.append("同一目标「{}」要求{}同时要求{}".format(targets, v1, v2))

            if reasons:
                conflicts.append({
                    "pairing": "{}×{}".format(d1["id"], d2["id"]),
                    "type": "DO_vs_DO",
                    "do_id": d1["id"], "do_line": d1["line"], "do_rule": d1["text"],
                    "dont_id": d2["id"], "dont_line": d2["line"], "dont_rule": d2["text"],
                    "description": "; ".join(reasons),
                    "severity": "严重",
                    "fix_suggestion": "标注主DO + 例外DO，或用优先级解决顺序冲突",
                })

    # Pattern 3: DON'T vs DON'T — upper-triangle NxN
    for i in range(len(dont_rules)):
        n1 = dont_rules[i]
        for j in range(i + 1, len(dont_rules)):
            n2 = dont_rules[j]
            reasons = []

            # 3a. Deadlock: "never refuse" + "never do dangerous things"
            has_never_refuse = "拒绝" in n1["text"] or "拒绝" in n2["text"]
            has_safety_block = any(w in n1["text"] or w in n2["text"]
                                   for w in ["危险", "安全", "风险", "违规", "违法"])
            if has_never_refuse and has_safety_block:
                reasons.append("「禁止拒绝」与「禁止危险/违规操作」产生死锁：用户要求危险操作时无合法路径")

            # 3b. Same target, both exhaustively blocked
            n1_nouns = _extract_nouns(n1["text"])
            n2_nouns = _extract_nouns(n2["text"])
            common_nouns = n1_nouns & n2_nouns
            n1_exhaustive = any(w in n1["text"] for w in ["所有", "一切", "全部", "任何"])
            n2_exhaustive = any(w in n2["text"] for w in ["所有", "一切", "全部", "任何"])
            if common_nouns and (n1_exhaustive or n2_exhaustive):
                targets = "、".join(sorted(common_nouns)[:2])
                reasons.append("对「{}」两条DON'T叠加产生全覆盖禁止，无合法通道".format(targets))

            if reasons:
                conflicts.append({
                    "pairing": "{}×{}".format(n1["id"], n2["id"]),
                    "type": "DONT_vs_DONT",
                    "do_id": n1["id"], "do_line": n1["line"], "do_rule": n1["text"],
                    "dont_id": n2["id"], "dont_line": n2["line"], "dont_rule": n2["text"],
                    "description": "; ".join(reasons),
                    "severity": "致命",
                    "fix_suggestion": "添加边界规则：安全优先，危险请求引导用户确认后手动执行",
                })

    return conflicts


ANTI_PATTERN_CHECKS = [
    ("不触发条件",       "## 何时不激活",     r"^-\s+用户",  4, "list"),
    ("蒸馏红线",         "## 🚫 反例与黑名单",        None,       4, "table"),
]


def check_anti_pattern_index(text):
    lines = text.split("\n")
    results = []
    for name, anchor, item_pattern, expected, check_type in ANTI_PATTERN_CHECKS:
        in_section = False
        count = 0
        if check_type == "table":
            for line in lines:
                if anchor in line:
                    in_section = True
                    continue
                if in_section:
                    if line.startswith("## ") or (line.startswith("### ") and anchor not in line):
                        break
                    if line.strip().startswith("|") and re.match(r"\|\s*\d+\s*\|", line.strip()):
                        count += 1
        else:
            for line in lines:
                if anchor in line:
                    in_section = True
                    continue
                if in_section:
                    if line.startswith("## "):
                        break
                    if anchor.startswith("#### ") and (line.startswith("### ") or line.startswith("#### ")):
                        if anchor not in line:
                            break
                    if re.match(item_pattern, line.strip()):
                        count += 1
        match = count == expected
        results.append({"section": name, "expected": expected, "actual": count, "match": match})
        if not match:
            results[-1]["warning"] = "expected {} items, found {}".format(expected, count)
    issues = sum(1 for r in results if not r["match"])
    return {"checks": results, "issues": issues, "verdict": "PASS" if issues == 0 else "MISMATCH"}


def validate_reference_files(skill_dir):
    ref_issues = []
    all_ref_files = []
    for sub in ["references", "examples"]:
        sub_dir = os.path.join(skill_dir, sub)
        if not os.path.isdir(sub_dir):
            continue
        for fname in sorted(os.listdir(sub_dir)):
            if fname.endswith(".md"):
                all_ref_files.append(os.path.join(sub_dir, fname))
    for ref_path in all_ref_files:
        rel = os.path.relpath(ref_path, skill_dir)
        with open(ref_path, "r", encoding="utf-8") as f:
            text = f.read()
        lines = text.split("\n")
        checks = []
        fence_count = text.count("```")
        if fence_count % 2 != 0:
            fence_lines = [i + 1 for i, l in enumerate(lines) if l.strip().startswith("```")]
            checks.append({"check": "code_fence_pairing", "status": "FAIL", "detail": "odd fenced {} at lines {}".format(fence_count, fence_lines)})
        else:
            checks.append({"check": "code_fence_pairing", "status": "PASS"})
        h2_titles = {}
        for i, l in enumerate(lines):
            if re.match(r"^##\s+(?!#)", l):
                title = l.strip()[3:]
                h2_titles.setdefault(title, []).append(i + 1)
        duplicates = {t: ps for t, ps in h2_titles.items() if len(ps) > 1}
        if duplicates:
            dup_detail = "; ".join("'{}' at lines {}".format(t, ps) for t, ps in duplicates.items())
            checks.append({"check": "h2_duplicates", "status": "FAIL", "detail": dup_detail})
        else:
            checks.append({"check": "h2_duplicates", "status": "PASS"})
        if len(text.strip()) < 100:
            checks.append({"check": "content_min_length", "status": "FAIL", "detail": "only {} chars (min 100)".format(len(text.strip()))})
        else:
            checks.append({"check": "content_min_length", "status": "PASS"})
        file_failures = [c for c in checks if c["status"] != "PASS"]
        ref_issues.append({"file": rel, "lines": len(lines), "checks": checks, "verdict": "PASS" if not file_failures else "FAIL"})
    total_failures = sum(1 for r in ref_issues if r["verdict"] != "PASS")
    return {"files_scanned": len(all_ref_files), "files_failed": total_failures, "results": ref_issues, "verdict": "PASS" if total_failures == 0 else "MISMATCH"}


# ─────────────────────────────────────────────────────────
# 岗位型内容校验 - 仅当检测到岗位型章节时生效
# ─────────────────────────────────────────────────────────

JOB_SECTION_MARKER_FRAGMENT = "身份与记忆"


def _check_job_identity(sections):
    errors = []
    sec = _get_section(sections, "身份与记忆")
    if not sec:
        return ["section missing: [identity/memory]"]
    for f in ["角色", "个性", "价值观优先", "记忆", "经验"]:
        if f not in sec:
            errors.append("identity field missing: {}".format(f))
    return errors


def _check_job_mission(sections):
    errors = []
    sec = _get_section(sections, "使命", "核心使命")
    if not sec:
        return ["section missing: [mission]"]
    if "反使命" not in sec:
        errors.append("mission missing: anti-mission")
    return errors


def _check_job_answer_protocol(sections):
    errors = []
    sec = _get_section(sections, "回答工作流", "Professional Answer Protocol")
    if not sec:
        return ["section missing: [answer protocol]"]
    for step in ["Step 1", "Step 2", "Step 3"]:
        if step not in sec:
            errors.append("answer protocol missing: {}".format(step))
    if _count_table_rows(sec) < 1:
        errors.append("answer protocol: dimension table empty")
    return errors


def _check_job_factions(sections):
    errors = []
    sec = _get_section(sections, "领域流派", "Domain Factions")
    if not sec:
        return []
    if "领域共识" not in sec:
        errors.append("factions missing: domain consensus")
    return errors


def _check_job_deliverables(sections):
    errors = []
    sec = _get_section(sections, "技术交付物", "Technical Deliverables")
    if not sec:
        return ["section missing: [deliverables]"]
    if _count_code_blocks(sec) == 0 and _count_list_items(sec) == 0:
        errors.append("deliverables empty: needs at least 1 template/checklist/code")
    return errors


def _check_job_workflow_steps(sections):
    errors = []
    sec = _get_section(sections, "工作流程", "Workflow")
    if not sec:
        return ["section missing: [workflow]"]
    count = _count_numbered_steps(sec)
    if count < 3:
        errors.append("workflow steps insufficient: {}/min 3".format(count))
    elif count > 5:
        errors.append("workflow steps excessive: {}/max 5".format(count))
    return errors


def _check_job_communication(sections):
    errors = []
    sec = _get_section(sections, "沟通风格", "Communication Style")
    if not sec:
        return ["section missing: [communication style]"]
    labels = len(re.findall(r"\*\*[^*\n]+\*\*\s*(?:标签|label)", sec))
    if labels < 3:
        errors.append("communication labels insufficient: {}/min 3".format(labels))
    if "拒绝方式" not in sec:
        errors.append("communication missing: refusal style")
    return errors


def _check_job_extreme_behavior(sections):
    errors = []
    sec = _get_section(sections, "极限行为", "Extreme Behavior")
    if not sec:
        return ["section missing: [extreme behavior]"]
    count = _count_table_rows(sec)
    if count < 4:
        errors.append("extreme behavior scenarios insufficient: {}/need 4".format(count))
    return errors


def _check_job_kpi(sections):
    errors = []
    sec = _get_section(sections, "成功指标", "KPI")
    if not sec:
        return ["section missing: [KPIs]"]
    count = _count_list_items(sec)
    if count < 3:
        errors.append("KPIs insufficient: {}/min 3".format(count))
    return errors


def _check_job_knowledge_base(sections):
    errors = []
    sec = _get_section(sections, "知识库", "Knowledge Base")
    if not sec:
        return ["section missing: [knowledge base]"]
    for layer in ["核心层", "应用层", "扩展层"]:
        if layer not in sec:
            errors.append("knowledge base missing layer: {}".format(layer))
    return errors


def _check_job_honesty(sections):
    errors = []
    sec = _get_section(sections, "诚实边界", "Honesty Boundary")
    if not sec:
        return ["section missing: [honesty boundary]"]
    count = _count_list_items(sec)
    if count < 5:
        errors.append("honesty boundary items insufficient: {}/min 5".format(count))
    return errors


def validate_job_structure(text):
    sections = extract_h2_sections(text)

    is_job_type = any(JOB_SECTION_MARKER_FRAGMENT in h for h in sections)
    if not is_job_type:
        return {"active": False, "note": "non-job SKILL.md, skipping deep checks"}

    all_errors = []
    checks = [
        ("identity", _check_job_identity(sections)),
        ("mission", _check_job_mission(sections)),
        ("answer_protocol", _check_job_answer_protocol(sections)),
        ("factions", _check_job_factions(sections)),
        ("deliverables", _check_job_deliverables(sections)),
        ("workflow", _check_job_workflow_steps(sections)),
        ("communication", _check_job_communication(sections)),
        ("extreme_behavior", _check_job_extreme_behavior(sections)),
        ("kpi", _check_job_kpi(sections)),
        ("knowledge_base", _check_job_knowledge_base(sections)),
        ("honesty", _check_job_honesty(sections)),
    ]
    errors_by_section = {}
    for check_name, errors in checks:
        if errors:
            errors_by_section[check_name] = errors
            all_errors.extend(["[{}] {}".format(check_name, e) for e in errors])

    return {
        "active": True,
        "checks_run": 11,
        "errors": all_errors,
        "errors_by_section": errors_by_section,
        "error_count": len(all_errors),
        "verdict": "PASS" if not all_errors else "FAIL",
    }


# ─────────────────────────────────────────────────────────
# 主报告
# ─────────────────────────────────────────────────────────


def generate_report(skill_path):
    if not os.path.exists(skill_path):
        return {"error": "file not found: {}".format(skill_path), "file_path": skill_path}

    skill_dir = os.path.dirname(os.path.abspath(skill_path))

    with open(skill_path, "r", encoding="utf-8") as f:
        text = f.read()

    report = {
        "file_path": os.path.abspath(skill_path),
        "file_size_bytes": os.path.getsize(skill_path),
        "total_lines": len(text.split("\n")),
    }

    # 1. YAML frontmatter
    parsed, raw_fm, fm_end = extract_frontmatter(text)
    if parsed is None:
        report["frontmatter"] = {"status": "MISSING", "errors": ["no YAML frontmatter found"]}
    else:
        required_fields = ["name", "description", "version", "domain", "author"]
        missing = [f for f in required_fields if f not in parsed]
        report["frontmatter"] = {
            "status": "OK" if not missing else "INCOMPLETE",
            "fields_found": list(parsed.keys()),
            "missing_required": missing,
            "raw": raw_fm,
        }

    # 2. CHECKPOINT
    report["checkpoints"] = count_checkpoints(text)

    # 3. H2 headings
    report["h2_headings"] = count_h2_headings(text)

    # 4. Rule conflicts
    conflicts = detect_rule_conflicts(text)
    report["rule_conflicts"] = {
        "conflicts_found": len(conflicts),
        "conflicts": conflicts,
        "verdict": "PASS" if len(conflicts) == 0 else "WARNING",
    }

    # 5. Anti-pattern index
    anti = check_anti_pattern_index(text)
    report["anti_pattern_index"] = anti

    # 6. Reference files
    report["reference_files"] = validate_reference_files(skill_dir)

    # 7. Job-type deep structure check (conditional)
    report["job_structure"] = validate_job_structure(text)

    # Overall
    issues = 0
    if report["frontmatter"]["status"] != "OK":
        issues += 1
    if report["rule_conflicts"]["verdict"] != "PASS":
        issues += 1
    if report["anti_pattern_index"]["verdict"] != "PASS":
        issues += 1
    if report["reference_files"]["verdict"] != "PASS":
        issues += 1
    if report["job_structure"].get("active") and report["job_structure"]["verdict"] != "PASS":
        issues += 1

    report["overall"] = {
        "issues": issues,
        "verdict": "PASS" if issues == 0 else "NEEDS_REVIEW",
    }

    return report


def main():
    parser = argparse.ArgumentParser(description="tiangong-skill SKILL.md structural validator")
    parser.add_argument("--skill", default=None, help="path to SKILL.md")
    parser.add_argument("--quiet", action="store_true", help="JSON only, no progress")
    parser.add_argument("--json", action="store_true", help="JSON to stdout only")
    args = parser.parse_args()

    if args.skill:
        skill_path = args.skill
        # 路径白名单：仅允许 tiangong-skill 目录或用户 skills/ 目录
        import pathlib
        skill_abs = str(pathlib.Path(skill_path).resolve())
        allowed_prefixes = [
            str(pathlib.Path(__file__).resolve().parent.parent),
            str(pathlib.Path.home() / "skills"),
        ]
        if not any(skill_abs.startswith(p) for p in allowed_prefixes):
            print(f"ERROR: --skill path must be under tiangong-skill or ~/skills/ directories. Got: {skill_abs}", file=sys.stderr)
            sys.exit(1)
    else:
        script_dir = os.path.dirname(os.path.abspath(__file__))
        skill_path = os.path.join(script_dir, "..", "SKILL.md")

    report = generate_report(skill_path)

    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
        sys.exit(0 if report.get("overall", {}).get("verdict") == "PASS" else 1)

    if not args.quiet:
        print("Verify target: {}".format(report.get("file_path", skill_path)), file=sys.stderr)
        print("Lines: {}".format(report.get("total_lines", "?")), file=sys.stderr)

    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
