#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook (GLOBAL, 非阻断): 新建/追加踩坑册条目前查重，防近重复。

补 errors_recall_guard 显式跳过记忆区写留下的去重缝：
recall guard 在写 ERR 单条/索引行时 self-exclude，故新建坑那一刻无人提醒"可能已有"。

- 仅当写目标为 `errors/ERR-*.md` 单条 或 `error_index.md` 时介入；其余写直接 exit 0。
- 从本次写内容抽取候选触发词，与现有活跃索引逐条比对：整词/短语级共享 >=1 或
  中文 bigram 共享 >=3 即判疑似重复（bigram 单独计权，防「弹窗/编辑」类高频二字误报）。
- 含空格的 ASCII 短语（ERROR 1046 等）按折叠空白后的原文做子串匹配，不走分词交集。
- 始终 exit 0（非阻断）：仅提醒，由 Agent 先核对再建（用户确认）。

记忆根目录解析同 errors_recall_guard.resolve_index_path（`AI_MEMORY_DIR` →
`PROJECT_ROOT/.ai-memory` → cwd/脚本位置上溯）。
"""
import sys
import os
import re
import json
import argparse

STRONG_MIN = 1  # 整词/短语级共享达此数即判疑似重复
WEAK_MIN = 3    # 仅中文 bigram 共享时须达此数（防「弹窗/编辑」类高频二字误报）

ERR_FILE_RE = re.compile(r"(?:^|/)ERR-\d+\.md$", re.IGNORECASE)
INDEX_RE = re.compile(r"(?:^|/)error_index\.md$", re.IGNORECASE)
MAX_HITS = 3


SEP_RE = re.compile(r"[/，、,；;]")


def resolve_index_path():
    """解析 `error_index.md` 路径；无法确定时返回 None（静默）。"""
    env_dir = os.environ.get("AI_MEMORY_DIR", "")
    if env_dir:
        return os.path.join(env_dir, "error_index.md")
    roots = []
    proj = os.environ.get("PROJECT_ROOT", "")
    if proj:
        roots.append(os.path.abspath(proj))
    for start in (os.getcwd(), os.path.dirname(os.path.abspath(__file__))):
        d = os.path.abspath(start)
        while True:
            roots.append(d)
            nd = os.path.dirname(d)
            if nd == d:
                break
            d = nd
    seen = set()
    for r in roots:
        if r in seen:
            continue
        seen.add(r)
        cand = os.path.join(r, ".ai-memory", "error_index.md")
        if os.path.isfile(cand):
            return cand
    return None


def _looks_like_keyword(s):
    """触发词判据：含分隔符或为长描述串。"""
    return bool(SEP_RE.search(s)) or len(s) > 16


def _split_cells(line):
    """去掉首尾管线后按 `|` 全拆分并 strip（保留列序，供右侧定列取值）。"""
    s = line.strip()
    if s.startswith("|"):
        s = s[1:]
    if s.endswith("|"):
        s = s[:-1]
    return [c.strip() for c in s.split("|")]


def parse_row(line):
    """解析一行索引 → (err_id, keyword, desc)；非数据行返回 None。

    兼容两种列序：
    - 本技能模板（≥6 列）：`ERR-ID | 一句话描述 | 根因分类 | 触发关键词 | 日期 | Recurrence-Count`
      → 触发词按**右侧定列**取（keyword=cells[-3]），描述用 `|` 回拼 cells[1:-4]；
        故描述内含 `|` 时列序不会左移、触发词不会被顶掉。
    - 精简 3 段：`ERR-ID | 触发关键词 | 描述` → keyword 取第 2 段。
    """
    cells = _split_cells(line)
    if len(cells) < 2 or not cells[0]:
        return None
    if len(cells) >= 6:
        return cells[0], cells[-3], "|".join(cells[1:-4]).strip()
    err_id, second = cells[0], cells[1]
    third = "|".join(cells[2:]) if len(cells) > 2 else ""
    if third and "|" in third:
        rest = cells[2:]
        keyword = rest[1] if len(rest) > 1 else ""
        if keyword:
            return err_id, keyword, second
    return err_id, second, third


def load_triggers(index_path):
    """解析 index 文件，返回 [(err_id, keyword, desc)]；遇「## 归档区」停止。"""
    if not index_path or not os.path.isfile(index_path):
        return []
    out = []
    try:
        with open(index_path, encoding="utf-8", errors="ignore") as f:
            for line in f:
                s = line.strip()
                if not s:
                    continue
                if s.startswith("##"):
                    if "归档" in s:
                        break
                    continue
                if s.startswith("#") or s.startswith(">"):
                    continue
                if s.startswith("|") and ("ERR-ID" in s or set(s) <= set("|-: ")):
                    continue
                row = parse_row(s.lstrip("|"))
                if row and row[1]:
                    out.append(row)
    except OSError:
        return []
    return out


def tokenize(keyword):
    """按分隔符切词（不按空格，保多词短语整体），返回 (whole, bigram)。

    - ASCII：保留 >=6（短词子串噪声大）。
    - 中文：2 字保留；>2 字补 bigram，使嵌入长句里的短 keyword 能与索引短词交集命中。
    - bigram 单独返回：它与整词不同权（见 STRONG_MIN/WEAK_MIN）。
    """
    whole, bigram = [], []
    for t in re.split(r"[/\，、,；;]+", keyword):
        t = t.strip().lower()
        if not t:
            continue
        if t.isascii():
            if len(t) < 6:
                continue
            whole.append(t)
        else:
            if len(t) == 1:
                continue
            whole.append(t)            # 整词（缓存击穿）
            # 仅对纯中文整词补 bigram；混入数字/字母的混合词不切，避免 ASCII 噪声 bigram。
            if len(t) >= 3 and re.fullmatch(r"[\u4e00-\u9fff]+", t):
                for i in range(len(t) - 1):
                    bigram.append(t[i:i + 2])
    return whole, bigram


def content_tokens(text):
    """候选侧分词，返回 (cand_whole, cand_bigram, norm_text)。

    - 先按空白切词再对每词走 tokenize，保证正文里 'ehref whhref 后台404'
      被拆成独立词。
    - norm_text：空白折叠为单空格的小写原文，供**含空格**的 ASCII 触发词
      （ERROR 1046 / CREATE TABLE …）做子串匹配。
    """
    cand_whole, cand_bigram = set(), set()
    for word in re.split(r"\s+", text or ""):
        w, b = tokenize(word)
        cand_whole.update(w)
        cand_bigram.update(b)
    norm_text = re.sub(r"\s+", " ", text or "").lower()
    return cand_whole, cand_bigram, norm_text


def read_event():
    try:
        raw = sys.stdin.buffer.read().decode("utf-8", errors="ignore")
    except Exception:
        raw = ""
    raw = raw.lstrip("\ufeff")
    if not raw.strip():
        return {}
    try:
        return json.loads(raw)
    except Exception:
        return {}


def emit(lines, event="PostToolUse"):
    """JSON 契约输出：`hookSpecificOutput.additionalContext` 在 PostToolUse 可达 Agent。
    **只输出这一个 JSON**，不与裸 print 混排。"""
    msg = "\n".join(lines)
    print(json.dumps({
        "systemMessage": msg,
        "hookSpecificOutput": {"hookEventName": event, "additionalContext": msg},
    }, ensure_ascii=False))


def main():
    parser = argparse.ArgumentParser(description="踩坑册新建条目查重守卫")
    parser.add_argument("--event-file", default="", help="测试用：从文件读 hook 事件 JSON")
    args = parser.parse_args()

    if args.event_file:
        try:
            with open(args.event_file, encoding="utf-8") as f:
                data = json.load(f)
        except Exception:
            return 0
    else:
        data = read_event()
    if not data:
        return 0

    tool_input = data.get("tool_input", {}) or {}
    path = (tool_input.get("filePath") or tool_input.get("file_path")
            or tool_input.get("path") or "")
    if not path:
        return 0
    low = os.path.abspath(path).replace("\\", "/").lower()
    if not (ERR_FILE_RE.search(low) or INDEX_RE.search(low)):
        return 0

    index_path = resolve_index_path()
    if not index_path:
        return 0

    # 排除自身编号，避免更新现有条目时自匹配。
    self_id = None
    m = re.search(r"err-(\d+)\.md$", low)
    if m:
        self_id = "ERR-" + m.group(1)
    content = (tool_input.get("content") or tool_input.get("new_str") or "")
    cm = re.search(r"ERR-(\d+)", content, re.IGNORECASE)
    if cm and not self_id:
        self_id = "ERR-" + cm.group(1)

    cand_whole, cand_bigram, norm_text = content_tokens(content)
    if not cand_whole and not cand_bigram:
        return 0

    triggers = load_triggers(index_path)
    if not triggers:
        return 0

    hits = []
    for err_id, keyword, desc in triggers:
        # 索引 err_id 可能带 年/月/ 前缀（如 2026/08/ERR-007），与自排除的
        # ERR-XXX 对齐后再比，否则编辑旧坑会自匹配误报。
        if err_id.split("/")[-1] == self_id:
            continue
        kw_whole, kw_bigram = tokenize(keyword)
        strong = set()
        for t in kw_whole:
            if " " in t:
                if t in norm_text:
                    strong.add(t)
            elif t in cand_whole:
                strong.add(t)
        weak = cand_bigram & set(kw_bigram)
        # 强（整词/短语）命中 1 个即疑似重复；仅 bigram 命中则需 >=3 个。
        if len(strong) >= STRONG_MIN or len(weak) >= WEAK_MIN:
            hits.append((len(strong), len(weak), err_id, desc or keyword, strong, weak))
    if not hits:
        return 0

    hits.sort(key=lambda x: (x[0], x[1]), reverse=True)
    lines = ["[ERR-DUP] 新建/追加踩坑条目与现有条目高度相似，先核对是否重复（Rules §20 #4）："]
    lines += ["  疑似重复 %s（强匹配：%s；弱匹配：%s）：%s"
              % (eid, "/".join(sorted(st)) or "无", "/".join(sorted(wk)) or "无", desc)
              for _ns, _nw, eid, desc, st, wk in hits[:MAX_HITS]]
    lines.append("  若确为新坑，请在描述中明确差异点后再建；若重复，更新既有 ERR 的 Recurrence-Count。")
    emit(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main())
