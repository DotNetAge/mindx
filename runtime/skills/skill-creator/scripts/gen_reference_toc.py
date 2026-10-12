#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""为超过 300 行的 references/*.md 生成「## 目录」段（树状缩进无序列表）。

用法: python3 gen_reference_toc.py <文件>...   或   python3 gen_reference_toc.py <目录>
条目格式: - 标题 [L始行-L末行]，子级缩进两空格，末行为章节最后一行。
规则:
- 仅处理 >= 300 行的 md 文件（<300 行不强制目录，已剥离的不再生成）
- 目录段插在 frontmatter 之后、正文第一个标题之前，段头为「## 目录」
- 索引 h2-h4；h1 视为文档标题不进目录；代码块围栏内的标题不算
- L末行 = 下一个级别不高于自身的标题的前一行；末条目到文件末行
- 行号含 frontmatter 与目录段自身（生成时预补偿偏移），供 Read 按 offset 定位
- 重建式幂等：已存在「## 目录」段或旧式 <!-- TOC-START --> 标记段则先剥离，
  再按当前正文重算，正文修改后重跑即可
"""
import re, sys, os

MIN_LINES = 300
TOC_HEADING = "## 目录"
TOC_START_MARK = "<!-- TOC-START -->"
TOC_END_MARK = "<!-- TOC-END -->"
HEADING_RE = re.compile(r'^(#{2,4})\s+(.+)$')
ANY_HEADING_RE = re.compile(r'^#{1,6}\s+')


def fence_run(s):
    """行首连续反引号数，>=3 视为围栏（四反引号也是合法围栏），否则 0。"""
    n = 0
    while n < len(s) and s[n] == '`':
        n += 1
    return n if n >= 3 else 0


def strip_toc(lines):
    """剥离已存在的「## 目录」段与旧式 <!-- TOC-START --> 标记段，返回纯正文行。

    围栏内的目录段头与标记是示例，不剥离；剥离时连带吸收紧邻的前导空行，
    保证重复执行不累积空行。围栏跟踪按 CommonMark 语义：闭合围栏须 >=
    开启围栏长度且行内无 info 串，使 ```` 示例块中嵌套的 ``` 不致错位。
    """
    out = []
    fence = 0
    in_toc = False
    in_legacy = False
    for ln in lines:
        s = ln.strip()
        if in_legacy:
            if s == TOC_END_MARK:
                in_legacy = False
            continue
        n = fence_run(s)
        if fence:
            if n >= fence and not s[n:]:
                fence = 0
            out.append(ln)
            continue
        if n:
            fence = n
            out.append(ln)
            continue
        if in_toc:
            if s == TOC_START_MARK:
                # 新目录段后紧跟旧标记段（v1 遗留），转入旧段剥离
                in_toc = False
                in_legacy = True
                continue
            m = ANY_HEADING_RE.match(s)
            if m:
                title = s.lstrip('#').strip()
                if title.startswith('目录') or title.startswith('索引'):
                    continue  # 旧式目录段头（## 目录索引…），仍属目录段
                in_toc = False
                out.append(ln)
            continue
        if s == TOC_HEADING:
            in_toc = True
            if out and not out[-1].strip():
                out.pop()
            continue
        if s == TOC_START_MARK:
            in_legacy = True
            if out and not out[-1].strip():
                out.pop()
            continue
        out.append(ln)
    return out


def locate_frontmatter(lines):
    """返回 frontmatter 结束行下标（0 起），无 frontmatter 返回 -1。"""
    if not lines or lines[0].strip() != '---':
        return -1
    for i, ln in enumerate(lines[1:], 1):
        if ln.strip() == '---':
            return i
    return -1


def extract_headings(lines, fm_end):
    """返回 [(level, title, lineno)]（1 起行号），跳过 frontmatter 与代码块内标题。"""
    headings = []
    fence = 0
    for i, ln in enumerate(lines):
        if i <= fm_end:
            continue
        s = ln.strip()
        n = fence_run(s)
        if fence:
            if n >= fence and not s[n:]:
                fence = 0
            continue
        if n:
            fence = n
            continue
        if s == TOC_HEADING or s == TOC_START_MARK:
            continue
        m = HEADING_RE.match(s)
        if m:
            headings.append((len(m.group(1)), m.group(2).strip(), i + 1))
    return headings


def process(path):
    with open(path, encoding='utf-8') as f:
        original = f.read().split('\n')
    lines = strip_toc(original)
    total = len(lines)
    if total < MIN_LINES:
        if lines != original:
            with open(path, 'w', encoding='utf-8') as f:
                f.write('\n'.join(lines))
            return f"TRIM {path} (剥离旧目录段后 {total} 行 < {MIN_LINES}，不再生成)"
        return f"SKIP {path} ({total} 行 < {MIN_LINES})"
    fm_end = locate_frontmatter(lines)
    headings = extract_headings(lines, fm_end)
    if not headings:
        return f"SKIP {path} (无标题可索引)"

    # 每章节末行 = 下一个级别不高于自身的标题前一行；末条目到文件末行
    ranges = []
    for idx, (lvl, _, lineno) in enumerate(headings):
        end = total
        for l2, _, ln2 in headings[idx + 1:]:
            if l2 <= lvl:
                end = ln2 - 1
                break
        ranges.append((lineno, end))

    # 目录段自身占 2 + N 行（标题 + 空行 + 条目），行号预补偿
    offset = 2 + len(headings)
    entries = [TOC_HEADING, '']
    for (lvl, title, lineno), (_, end) in zip(headings, ranges):
        indent = '  ' * (lvl - 2)
        entries.append(f"{indent}- {title} [L{lineno + offset}-L{end + offset}]")

    insert_at = fm_end + 1
    new_lines = lines[:insert_at] + entries + lines[insert_at:]
    with open(path, 'w', encoding='utf-8') as f:
        f.write('\n'.join(new_lines))
    return f"OK   {path} ({total} 行, {len(headings)} 个标题, 目录段插于行 {insert_at + 1})"


if __name__ == '__main__':
    targets = []
    for a in sys.argv[1:]:
        if os.path.isdir(a):
            # 递归收集 references/ 目录下的 .md（排除 archive 与 node_modules）
            for root, dirs, files in os.walk(a):
                dirs[:] = [d for d in dirs if d not in ('archive', 'node_modules', '.git')]
                # 用 normpath 去掉尾斜杠，避免 basename 返回空串导致目录模式漏扫
                if os.path.basename(os.path.normpath(root)) != 'references':
                    continue
                targets += [os.path.join(root, f) for f in sorted(files) if f.endswith('.md')]
        else:
            targets.append(a)
    ok = 0
    for t in targets:
        r = process(t)
        if r.startswith('OK'):
            ok += 1
        print(r)
    print(f'完成：{ok} 个文件已生成/更新目录段')
