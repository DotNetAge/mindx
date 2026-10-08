#!/usr/bin/env python3
"""
luban-skill 共享模块 —— frontmatter 解析 / 触发词提取 / TSV 解析公共函数。
供 scripts/ 下所有脚本导入使用。
"""
import re
from pathlib import Path


def parse_frontmatter(text: str) -> dict[str, str]:
    """从 SKILL.md 文本中解析 YAML frontmatter，返回字段字典。
    返回空 dict 表示不存在或格式异常。
    """
    if not text.startswith("---"):
        return {}
    parts = text.split("---", 2)
    if len(parts) < 3:
        return {}
    yb = parts[1].strip()
    result: dict[str, str] = {}
    for field in ["name", "description"]:
        m = re.search(rf"^{field}\s*:\s*(.+)", yb, re.MULTILINE)
        if m:
            result[field] = m.group(1).strip()
    # 也尝试提取其他常见字段
    for field in ["version", "author", "license"]:
        m = re.search(rf"^{field}\s*:\s*(.+)", yb, re.MULTILINE)
        if m:
            result[field] = m.group(1).strip()
    return result


def extract_triggers(text: str, max_len: int = 20) -> set[str]:
    """从 SKILL.md frontmatter description 及正文触发词章节提取触发词集合。

    - text: SKILL.md 完整文本
    - max_len: 触发词最大字符数，超过此长度的不做触发词
    """
    triggers: set[str] = set()

    # 方式1: frontmatter description 引号内和逗号分隔
    desc_match = re.search(r"^description\s*:\s*(.+)", text, re.MULTILINE)
    if desc_match:
        desc = desc_match.group(1)
        # 引号内的词
        for m in re.finditer(r"['\"「『]([^'\"」』]+)['\"」』]", desc):
            word = m.group(1).strip()
            if word and len(word) <= max_len:
                triggers.add(word)
        # 逗号分隔的词
        for part in desc.split(","):
            part = part.strip().strip("'\"\\").strip()
            if part and len(part) <= max_len and not part.startswith(("触发", "如", "例", "当")):
                triggers.add(part)

    # 方式2: "触发"/"关键词" 章节显式声明的触发词
    for m in re.finditer(r"(?:触发|关键词)[：:]\s*(.+)", text):
        for word in re.split(r"[,，、/]", m.group(1)):
            word = word.strip().strip("'\"").strip()
            if word and len(word) <= 30:
                triggers.add(word)

    return triggers


def parse_tsv(tsv_path: Path) -> list[dict]:
    """解析 TSV 文件（含表头行），返回 list[dict]。
    文件不存在或无数据行时返回空列表。
    """
    records: list[dict] = []
    if not tsv_path.exists():
        return records
    lines = tsv_path.read_text(encoding="utf-8").strip().split("\n")
    if len(lines) < 2:
        return records
    headers = lines[0].split("\t")
    for line in lines[1:]:
        fields = line.split("\t")
        if len(fields) >= len(headers):
            rec = dict(zip(headers, fields))
            records.append(rec)
    return records
