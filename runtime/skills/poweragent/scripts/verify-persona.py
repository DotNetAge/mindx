#!/usr/bin/env python3
"""
verify-persona.py — 人格蒸馏产物结构验证脚本

校验维度：
  1. YAML frontmatter 完整性（name / description / color）
  2. 必选 H2 章节全覆盖
  3. 心智模型数量 [3, 7]
  4. 决策启发式数量 [5, 10]
  5. 表达DNA 维度完整（7 行）
  6. 诚实边界条款 ≥5
  7. 退化行为场景 = 4
  8. 成功指标 ≥3
  9. 工作流程步骤 [3, 5]
 10. 技术交付物 ≥1
 11. 调研来源：一手 + 二手 ≥1
 12. CHECKPOINT 计数（SKILL.md 内嵌 CHECKPOINT 统一定义）

用法：
  python verify-persona.py <path/to/SKILL.md> [--json]

退出码：
  0 = PASS, 1 = FAIL
"""

import sys
import json
import re
from pathlib import Path

# 共享工具函数
from _common import (
    count_table_rows,
    count_list_items,
    count_numbered_steps,
    count_code_blocks,
    extract_h2_sections,
)


def parse_frontmatter(text: str) -> dict:
    """提取 YAML frontmatter，返回键值对。"""
    m = re.match(r'^---\s*\n(.*?)\n---', text, re.DOTALL)
    if not m:
        return {}
    fm = {}
    for line in m.group(1).split('\n'):
        kv = re.match(r'^(\w[\w-]*)\s*:\s*(.+?)\s*$', line)
        if kv:
            fm[kv.group(1)] = kv.group(2).strip('"\' ')
    return fm


# ─── 检查函数 ──────────────────────────────────────────

def check_yaml(fm: dict) -> list[str]:
    errors = []
    if 'name' not in fm:
        errors.append("YAML frontmatter 缺少必填字段: name")
    if 'description' not in fm:
        errors.append("YAML frontmatter 缺少必填字段: description")
    if 'color' not in fm:
        errors.append("YAML frontmatter 缺少必填字段: color")
    return errors


def check_required_sections(sections: dict[str, str]) -> list[str]:
    """检查所有必选章节是否存在。"""
    required = [
        '🎭 角色扮演规则',
        '🪪 身份卡',
        '⚖️ 价值观与反模式',
        '🧬 心智模型',
        '🔄 决策启发式',
        '⚙️ 回答工作流',
        '💬 表达 DNA',
        '📅 人物时间线',
        '🧬 智识谱系',
        '🛡️ 诚实边界',
        '📋 技术交付物',
        '🔄 工作流程',
        '📊 成功指标',
        '💭 沟通风格',
        '🧱 退化行为设计',
        '📎 附录：调研来源',
    ]
    errors = []
    for req in required:
        # 模糊匹配：去掉 emoji 前缀后比较
        found = any(req.lstrip('🎭🪪⚖️🧬🔄⚙️💬📅🛡️📋📊💭🧱📎 ') in k for k in sections)
        if not found:
            errors.append(f"缺少必选章节: {req}")
    return errors


def check_identity_card(sections: dict[str, str]) -> list[str]:
    """身份卡 5 字段检查。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '身份卡' in k:
            section = v
            break
    if not section:
        return ["未找到「身份卡」章节"]

    fields = ['我是谁', '我的起点', '我现在在做什么', '风格标签', '公开形象']
    for f in fields:
        if f not in section:
            errors.append(f"身份卡缺少字段: {f}")
    return errors


def check_mental_models(sections: dict[str, str]) -> list[str]:
    """心智模型：数量 [3, 7]，每个需有三重验证痕迹。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '心智模型' in k:
            section = v
            break
    if not section:
        return ["未找到「心智模型」章节"]

    # 计数：每个「模型 N」或「模型N」算一个
    count = len(re.findall(r'模型\s*\d+', section))
    if count < 3:
        errors.append(f"心智模型不足: {count}/最少3个")
    elif count > 7:
        errors.append(f"心智模型超标: {count}/最多7个")

    # 三重验证痕迹
    has_cross_domain = bool(re.search(r'跨域|≥\s*2|领域', section))
    has_generative = bool(re.search(r'生成力|预测|推断|新问题', section))
    has_exclusive = bool(re.search(r'排他性|独特|不是所有', section))
    if not has_cross_domain:
        errors.append("心智模型缺少「跨域复现」验证痕迹")
    if not has_generative:
        errors.append("心智模型缺少「生成力」验证痕迹")
    if not has_exclusive:
        errors.append("心智模型缺少「排他性」验证痕迹")

    return errors


def check_heuristics(sections: dict[str, str]) -> list[str]:
    """决策启发式：[5, 10] 条。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '决策启发式' in k:
            section = v
            break
    if not section:
        return ["未找到「决策启发式」章节"]

    count = count_table_rows(section)
    if count < 5:
        errors.append(f"决策启发式不足: {count}/最少5条")
    elif count > 10:
        errors.append(f"决策启发式超标: {count}/最多10条")
    return errors


def check_expression_dna(sections: dict[str, str]) -> list[str]:
    """表达DNA：必须 7 个维度。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '表达 DNA' in k or '表达DNA' in k:
            section = v
            break
    if not section:
        return ["未找到「表达 DNA」章节"]

    count = count_table_rows(section)
    if count < 7:
        errors.append(f"表达DNA维度不足: {count}/7")
    elif count > 7:
        errors.append(f"表达DNA维度超标: {count}/7")

    # 关键维度检查
    dims = ['句式偏好', '语气停顿标记', '高频词汇', '确定性风格', '幽默方式', '情绪基线', '默认姿态']
    for d in dims:
        if d not in section:
            errors.append(f"表达DNA缺少维度: {d}")

    return errors


def check_honesty_boundary(sections: dict[str, str]) -> list[str]:
    """诚实边界：≥5 条。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '诚实边界' in k:
            section = v
            break
    if not section:
        return ["未找到「诚实边界」章节"]

    count = count_list_items(section)
    if count < 5:
        errors.append(f"诚实边界条款不足: {count}/最少5条")

    # 关键项
    key_items = ['不能预测', '不能替代', '公开表达', '信息截止', '一手来源']
    for ki in key_items:
        if ki not in section:
            errors.append(f"诚实边界缺少关键项: {ki}")

    return errors


def check_degradation(sections: dict[str, str]) -> list[str]:
    """退化行为：4 场景。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '退化行为' in k:
            section = v
            break
    if not section:
        return ["未找到「退化行为设计」章节"]

    count = count_table_rows(section)
    if count != 4:
        errors.append(f"退化行为场景数异常: {count}/需要4个")

    return errors


def check_workflow(sections: dict[str, str]) -> list[str]:
    """工作流程：[3, 5] 步。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '工作流程' in k and '回答工作流' not in k:
            section = v
            break
    if not section:
        return ["未找到「工作流程」章节"]

    count = count_numbered_steps(section)
    if count < 3:
        errors.append(f"工作流程步骤不足: {count}/最少3步")
    elif count > 5:
        errors.append(f"工作流程步骤超标: {count}/最多5步")

    return errors


def check_success_metrics(sections: dict[str, str]) -> list[str]:
    """成功指标：≥3 条。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '成功指标' in k:
            section = v
            break
    if not section:
        return ["未找到「成功指标」章节"]

    count = count_list_items(section)
    if count < 3:
        errors.append(f"成功指标不足: {count}/最少3条")

    return errors


def check_deliverables(sections: dict[str, str]) -> list[str]:
    """技术交付物：≥1 个。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '技术交付物' in k:
            section = v
            break
    if not section:
        return ["未找到「技术交付物」章节"]

    code_blocks = count_code_blocks(section)
    lists = count_list_items(section)
    if code_blocks == 0 and lists == 0:
        errors.append("技术交付物为空：需要至少1个模板/代码/清单")

    return errors


def check_sources(sections: dict[str, str]) -> list[str]:
    """调研来源：一手 ≥1，二手 ≥1，关键引用 ≥1。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '附录' in k and '来源' in k:
            section = v
            break
    if not section:
        return ["未找到「附录：调研来源」章节"]

    if '一手来源' not in section and '一手' not in section:
        errors.append("缺少一手来源段落")
    if '二手来源' not in section and '二手' not in section:
        errors.append("缺少二手来源段落")
    if '关键引用' not in section:
        errors.append("缺少关键引用段落")

    return errors


def check_timeline(sections: dict[str, str]) -> list[str]:
    """人物时间线：至少 1 条。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '时间线' in k:
            section = v
            break
    if not section:
        return ["未找到「人物时间线」章节"]

    count = count_table_rows(section)
    if count < 1:
        errors.append("人物时间线为空：至少需要1条事件")

    # 最新动态段落
    if '最新动态' not in section:
        errors.append("缺少「最新动态」段落")

    return errors


def check_answer_protocol(sections: dict[str, str]) -> list[str]:
    """回答工作流：Step 1/2/3 完整。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '回答工作流' in k:
            section = v
            break
    if not section:
        return ["未找到「回答工作流」章节"]

    for step in ['Step 1', 'Step 2', 'Step 3']:
        if step not in section:
            errors.append(f"回答工作流缺少: {step}")

    # 研究维度表至少有 1 个维度
    dim_count = count_table_rows(section)
    if dim_count < 1:
        errors.append("回答工作流研究维度表为空")

    return errors


def check_communication_style(sections: dict[str, str]) -> list[str]:
    """沟通风格：≥3 个风格标签 + 拒绝方式。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '沟通风格' in k:
            section = v
            break
    if not section:
        return ["未找到「沟通风格」章节"]

    # 风格标签：**标签**
    labels = len(re.findall(r'\*\*[^*]+标签[^*]*\*\*', section))
    if labels < 3:
        errors.append(f"沟通风格标签不足: {labels}/最少3个")

    if '拒绝方式' not in section:
        errors.append("缺少「拒绝方式」")

    return errors


def check_intellectual_genealogy(sections: dict[str, str]) -> list[str]:
    """智识谱系：影响我/我影响。"""
    errors = []
    section = ''
    for k, v in sections.items():
        if '智识谱系' in k:
            section = v
            break
    if not section:
        return ["未找到「智识谱系」章节"]

    if '影响过我' not in section:
        errors.append("缺少「影响过我的人」")
    if '我影响了' not in section:
        errors.append("缺少「我影响了谁」")

    return errors


# ─── 主流程 ────────────────────────────────────────────

def verify(filepath: str) -> dict:
    path = Path(filepath)
    if not path.exists():
        return {"result": "FAIL", "errors": [f"文件不存在: {filepath}"], "warnings": []}

    text = path.read_text(encoding='utf-8')
    fm = parse_frontmatter(text)
    sections = extract_h2_sections(text)

    all_errors = []
    all_warnings = []

    checks = [
        ("YAML frontmatter", check_yaml(fm)),
        ("必选章节", check_required_sections(sections)),
        ("身份卡", check_identity_card(sections)),
        ("心智模型", check_mental_models(sections)),
        ("决策启发式", check_heuristics(sections)),
        ("表达DNA", check_expression_dna(sections)),
        ("回答工作流", check_answer_protocol(sections)),
        ("诚实边界", check_honesty_boundary(sections)),
        ("退化行为", check_degradation(sections)),
        ("工作流程", check_workflow(sections)),
        ("成功指标", check_success_metrics(sections)),
        ("技术交付物", check_deliverables(sections)),
        ("调研来源", check_sources(sections)),
        ("人物时间线", check_timeline(sections)),
        ("沟通风格", check_communication_style(sections)),
        ("智识谱系", check_intellectual_genealogy(sections)),
    ]

    for check_name, errors in checks:
        for e in errors:
            all_errors.append(f"[{check_name}] {e}")

    # 额外: 一手来源占比检查
    for k, v in sections.items():
        if '诚实边界' in k:
            m = re.search(r'一手来源[占比]*\s*(\d+)', v)
            if m:
                pct = int(m.group(1))
                if pct < 30:
                    all_warnings.append(f"一手来源占比 {pct}% < 30%（低可信度）")
            else:
                all_warnings.append("诚实边界未声明一手来源占比")
            break

    result = "PASS" if not all_errors else "FAIL"
    return {
        "result": result,
        "errors": all_errors,
        "warnings": all_warnings,
        "file": str(path.absolute()),
    }


def main():
    if len(sys.argv) < 2:
        print("用法: python verify-persona.py <SKILL.md路径> [--json]", file=sys.stderr)
        sys.exit(2)

    filepath = sys.argv[1]
    use_json = '--json' in sys.argv

    report = verify(filepath)

    if use_json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print(f"验证文件: {report['file']}")
        print(f"结果: {report['result']}")
        print(f"错误: {len(report['errors'])} 条")
        for e in report['errors']:
            print(f"  ❌ {e}")
        if report['warnings']:
            print(f"警告: {len(report['warnings'])} 条")
            for w in report['warnings']:
                print(f"  ⚠️  {w}")
        print()

    sys.exit(0 if report['result'] == 'PASS' else 1)


if __name__ == '__main__':
    main()
