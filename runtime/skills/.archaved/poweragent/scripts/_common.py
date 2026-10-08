"""Verify 脚本共享工具函数"""

import re


def count_table_rows(section: str) -> int:
    """统计 markdown 表格数据行数（不含表头和分隔线）。"""
    lines = section.strip().split("\n")
    count = 0
    in_table = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("|") and not re.match(r"^\|[\s:\-|]+\|$", stripped):
            if not in_table:
                in_table = True
                continue  # skip header row
            count += 1
        elif not stripped.startswith("|"):
            in_table = False
    return count


def count_list_items(section: str) -> int:
    """统计无序列表条目数。"""
    return len(re.findall(r"^\s*[-*]\s+", section, re.MULTILINE))


def count_numbered_steps(section: str) -> int:
    """统计编号步骤数（第一步/第二步/Step 1/Step 2）。"""
    return len(re.findall(r"(?:第[一二三四五六七八九]步|Step\s+\d+)", section))


def count_code_blocks(section: str) -> int:
    """统计代码块数（```）。"""
    return section.count("```") // 2


def extract_h2_sections(text: str) -> dict[str, str]:
    """提取所有 ## 标题及其内容（到下一个 ## 或文件尾）。"""
    sections = {}
    pattern = re.compile(r"^##\s+(.+?)$", re.MULTILINE)
    matches = list(pattern.finditer(text))
    for i, m in enumerate(matches):
        start = m.end()
        end = matches[i + 1].start() if i + 1 < len(matches) else len(text)
        sections[m.group(1).strip()] = text[start:end]
    return sections


def get_section(sections: dict[str, str], *keywords: str) -> str:
    """从 sections 字典中按关键词模糊匹配查找对应 section。"""
    for k, v in sections.items():
        for kw in keywords:
            if kw in k:
                return v
    return ""


def extract_table_column(section: str, col_idx: int) -> list[str]:
    """提取表格指定列的所有值。"""
    rows = []
    lines = section.strip().split("\n")
    in_table = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("|") and not re.match(r"^\|[\s:\-|]+\|$", stripped):
            if not in_table:
                in_table = True
                continue  # header
            cells = [c.strip() for c in stripped.split("|")[1:-1]]
            if col_idx < len(cells):
                rows.append(cells[col_idx])
        elif not stripped.startswith("|"):
            in_table = False
    return rows
