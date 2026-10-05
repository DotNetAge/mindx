#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook: 将记忆根目录下超过 KEEP_DAYS 天的日迹归档到 `backup/YYYY/MM/`。

- 非阻塞：任何异常均 exit 0，绝不阻断原工具执行。
- 以**目录名日期** `YYYYMMDD` 为准计算年龄，与目录 mtime 无关。
- 仅匹配记忆根目录下 `^\\d{8}$` 的日迹目录（本技能 `daily.md` 按日目录存放）；
  `backup/` / `errors/` / `knowledge-graph/` / `project_memory.md` / `handoff.md`
  等一律跳过。
- 幂等：已归档目录不在根目录，自然不再处理；目标已存在则跳过并告警（保留源与目标，禁删源）。
- 只移动、不删除原始内容，保留历史可回溯。

记忆根目录解析同 errors_recall_guard：`AI_MEMORY_DIR` → `PROJECT_ROOT/.ai-memory`
→ cwd 上溯 → 脚本位置上溯；未命中则静默 exit 0。
支持 `--dry-run`（只报告不移动，供反例验证）。
"""
import sys
import os
import re
import json
import shutil
import argparse
from datetime import date, datetime

KEEP_DAYS = 14
DATE_DIR_RE = re.compile(r"^(\d{4})(\d{2})(\d{2})$")
SKIP_NAMES = {"backup", "errors", "knowledge-graph", "__pycache__"}


def resolve_memory_root():
    """解析记忆根目录；无法确定时返回 None（静默）。"""
    env_dir = os.environ.get("AI_MEMORY_DIR", "")
    if env_dir and os.path.isdir(env_dir):
        return os.path.abspath(env_dir)
    roots = []
    proj = os.environ.get("PROJECT_ROOT", "")
    if proj:
        roots.append(os.path.join(os.path.abspath(proj), ".ai-memory"))
    for start in (os.getcwd(), os.path.dirname(os.path.abspath(__file__))):
        d = os.path.abspath(start)
        while True:
            roots.append(os.path.join(d, ".ai-memory"))
            nd = os.path.dirname(d)
            if nd == d:
                break
            d = nd
    seen = set()
    for r in roots:
        if r in seen:
            continue
        seen.add(r)
        if os.path.isdir(r):
            return r
    return None


def emit(lines, event="PostToolUse", detail=None):
    """JSON 契约输出：`detail` 给出时进 `systemMessage`（IDE UI 可见），
    Agent 侧只收 `lines`（避免噪声）。"""
    msg = "\n".join(lines)
    print(json.dumps({
        "systemMessage": "\n".join(list(detail) + list(lines)) if detail else msg,
        "hookSpecificOutput": {"hookEventName": event, "additionalContext": msg},
    }, ensure_ascii=False))


def main():
    parser = argparse.ArgumentParser(description="日迹归档（>KEEP_DAYS 天）")
    parser.add_argument("--dry-run", action="store_true", help="只报告不移动（反例验证用）")
    args = parser.parse_args()

    # stdin 负载仅用于兼容性读取，不参与逻辑
    try:
        sys.stdin.buffer.read().decode("utf-8", "replace")
    except Exception:
        pass

    mem_root = resolve_memory_root()
    if not mem_root:
        return 0

    today = date.today()
    archived = []
    skipped = []
    try:
        for name in os.listdir(mem_root):
            if name in SKIP_NAMES:
                continue
            m = DATE_DIR_RE.match(name)
            if not m:
                continue
            src = os.path.join(mem_root, name)
            if not os.path.isdir(src):
                continue
            try:
                fdate = datetime.strptime(name, "%Y%m%d").date()
            except Exception:
                continue
            if (today - fdate).days <= KEEP_DAYS:
                continue
            dest_dir = os.path.join(mem_root, "backup", m.group(1), m.group(2))
            dst = os.path.join(dest_dir, name)
            if os.path.exists(dst):
                # 禁止删除源记忆（无版本控制时删除=永久）。保留源与目标，告警后跳过。
                skipped.append("%s -> %s" % (src, dst))
                continue
            if args.dry_run:
                archived.append(name)
                continue
            os.makedirs(dest_dir, exist_ok=True)
            shutil.move(src, dst)
            archived.append(name)
    except Exception:
        return 0

    if archived or skipped:
        lines = []
        if archived:
            lines.append("memory_prune: archived %d daily dir(s) >%d days -> backup/%s/%s/ (e.g. %s)"
                         % (len(archived), KEEP_DAYS, archived[0][:4], archived[0][4:6], archived[0]))
        if skipped:
            lines.append("memory_prune: 目标已存在，跳过归档（源保留，请人工核对）：" + "；".join(skipped))
        emit(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main())
