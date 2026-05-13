"""
Simple YAML extractor for MindX subagent scripts.
Handles the specific formats used in agent .md files, skill SKILL.md files, and models.yml.
No external dependencies — stdlib only.
"""

import json
import re
import sys
import os


def parse_yaml_frontmatter(content):
    """Parse YAML frontmatter from a markdown string. Returns dict or None."""
    content = content.strip()
    if not content.startswith("---"):
        return None
    parts = content.split("\n---", 1)
    if len(parts) < 2:
        return None
    yaml_text = parts[0][3:].strip()
    if not yaml_text:
        return None
    return _parse_fields(yaml_text.split("\n"))


def parse_yaml_file(filepath):
    """Parse a standalone YAML file (e.g. models.yml). Returns dict."""
    with open(filepath, "r", encoding="utf-8") as f:
        lines = f.read().split("\n")
    return _parse_fields(lines)


def _parse_fields(lines):
    """Parse YAML lines into a dict. Handles scalars, block scalars (>, |), and nested lists."""
    result = {}
    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        if stripped == "" or stripped.startswith("#"):
            i += 1
            continue

        indent = len(line) - len(line.lstrip())

        m = _kv_match(stripped)
        if m is None:
            i += 1
            continue

        key, value = m.group(1), m.group(2).strip()

        if value in (">", "|"):
            result[key], i = _read_block_scalar(lines, i + 1)
            continue

        if value != "":
            result[key] = _unquote(value)
            i += 1
            continue

        i += 1
        if i >= len(lines):
            result[key] = ""
            continue

        next_line = lines[i]
        ns = next_line.strip()

        if ns in (">", "|"):
            result[key], i = _read_block_scalar(lines, i + 1)
            continue

        if ns.startswith("-"):
            ni = len(next_line) - len(next_line.lstrip())
            result[key] = _parse_list(lines, i, ni)
            continue

        result[key] = ""

    return result


def _read_block_scalar(lines, start_i):
    """Read folded/literal block scalar text. Returns (joined_text, next_line_index)."""
    scalar_lines = []
    base_indent = None
    i = start_i
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if stripped == "" or stripped.startswith("#"):
            if base_indent is not None:
                scalar_lines.append("")
            i += 1
            continue

        if base_indent is None:
            base_indent = indent

        if indent >= base_indent:
            scalar_lines.append(stripped)
            i += 1
        else:
            break

    return (" ".join(scalar_lines), i)


def _parse_list(lines, start_i, list_indent):
    """Parse a YAML list. Each item can be a scalar or a multi-field dict."""
    items = []
    i = start_i

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if stripped == "" or stripped.startswith("#"):
            i += 1
            continue

        if indent < list_indent:
            break

        if stripped.startswith("-"):
            item_val = stripped[1:].strip()
            m = _kv_match(item_val)
            if m:
                item = {m.group(1): _unquote(m.group(2).strip())}
                i += 1
                i = _collect_item_fields(lines, i, indent, item)
                items.append(item)
            else:
                items.append(_unquote(item_val))
                i += 1
        else:
            i += 1

    return items


def _collect_item_fields(lines, i, parent_indent, item):
    """Collect additional key-value fields belonging to a dictionary-style list item.
    Handles block scalars (>, |) as field values. Returns the next line index."""
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if stripped == "" or stripped.startswith("#"):
            i += 1
            continue

        if indent <= parent_indent:
            break

        if stripped.startswith("-") and indent == parent_indent:
            break

        m = _kv_match(stripped)
        if m is None:
            i += 1
            continue

        key, value = m.group(1), m.group(2).strip()

        if value in (">", "|"):
            item[key], i = _read_block_scalar(lines, i + 1)
            continue

        if value != "":
            item[key] = _unquote(value)
            i += 1
            continue

        i += 1
        if i < len(lines):
            nl = lines[i]
            ns = nl.strip()
            if ns in (">", "|"):
                item[key], i = _read_block_scalar(lines, i + 1)
            elif ns.startswith("-"):
                ni = len(nl) - len(nl.lstrip())
                item[key] = _parse_list(lines, i, ni)
            else:
                item[key] = ""

    return i


def _kv_match(line):
    """Match 'key: value' or 'key:' pattern. Returns re.Match or None."""
    return re.match(r'^(\w[\w\d_.]*)\s*:\s*(.*)', line)


def _unquote(value):
    """Remove surrounding single or double quotes from a YAML value."""
    value = value.strip()
    if len(value) >= 2:
        if (value.startswith('"') and value.endswith('"')) or \
           (value.startswith("'") and value.endswith("'")):
            return value[1:-1]
    return value


def fail(message):
    """Print error message to stderr and exit with code 1."""
    print(message, file=sys.stderr)
    sys.exit(1)


def output_json(data):
    """Print data as formatted JSON to stdout."""
    print(json.dumps(data, indent=2, ensure_ascii=False))