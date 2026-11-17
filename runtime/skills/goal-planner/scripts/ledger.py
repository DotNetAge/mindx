#!/usr/bin/env python3
"""goal-ledger.md 解析与核对脚本（确定性操作不裸算）。

用法:
    ledger.py <goal-ledger.md> summary          # 全量目标进度汇总（JSON）
    ledger.py <goal-ledger.md> tasks            # 任务清单：状态/预算/负责人/无预算标记（JSON）
    ledger.py <goal-ledger.md> validate         # 格式校验：缺字段/坏编号/状态非法

输出: JSON 走 stdout；错误与告警走 stderr；退出码 0 成功 / 1 用法错误 / 2 解析失败。
"""
import json
import re
import sys

STATUS = {"待批准", "已批准", "已驳回"}
GOAL_RE = re.compile(r"^## (G-\d{8}-\d+)\s+(.+)$")
FIELD_RE = re.compile(r"^- (状态|原话|意图|成功判据|不做)：\s*(.*)$")
TASK_RE = re.compile(
    r"^  - \[( |x)\] (T\S+)\s+(.+?)"
    r"(?:\s+→\s*([^\s（]+))?"
    r"\s*(?:（(?:cron|执行)：(.*?)）)?"
    r"\s*(?:预算：(\d+)\s*轮)?"
    r"\s*(?:验收：(.*))?$"
)
PROGRESS_RE = re.compile(r"^  - (\d{4}-\d{2}-\d{2})\s+(.*)$")


def parse(path):
    goals, cur, warn = [], None, []
    with open(path, encoding="utf-8") as f:
        for lineno, line in enumerate(f, 1):
            line = line.rstrip("\n")
            m = GOAL_RE.match(line)
            if m:
                cur = {"id": m.group(1), "title": m.group(2), "status": None,
                       "tasks": [], "progress": []}
                goals.append(cur)
                continue
            if cur is None:
                continue
            m = FIELD_RE.match(line)
            if m and m.group(1) == "状态":
                cur["status"] = m.group(2).strip()
                if cur["status"] not in STATUS:
                    warn.append(f"{lineno}: 非法状态 [{cur['status']}]")
                continue
            m = TASK_RE.match(line)
            if m:
                cur["tasks"].append({"done": m.group(1) == "x", "id": m.group(2),
                                     "title": m.group(3).strip(), "owner": m.group(4),
                                     "exec": m.group(5), "budget_rounds": int(m.group(6)) if m.group(6) else None,
                                     "acceptance": m.group(7)})
                continue
            m = PROGRESS_RE.match(line)
            if m:
                cur["progress"].append({"date": m.group(1), "note": m.group(2)})
    return goals, warn


def main():
    if any(a in {"-h", "--help"} for a in sys.argv[1:]):
        print(__doc__)
        sys.exit(0)
    if len(sys.argv) != 3 or sys.argv[2] not in {"summary", "tasks", "validate"}:
        print(__doc__, file=sys.stderr)
        sys.exit(1)
    try:
        goals, warn = parse(sys.argv[1])
    except OSError as e:
        print(f"error: {e}", file=sys.stderr)
        sys.exit(2)
    if "validate" == sys.argv[2]:
        problems = list(warn)
        for g in goals:
            for k in ("status",):
                if not g.get(k):
                    problems.append(f"{g['id']}: 缺 {k}")
            for t in g["tasks"]:
                if t["budget_rounds"] is None:
                    problems.append(f"{g['id']} {t['id']}: 无预算轮数")
                if not t["acceptance"]:
                    problems.append(f"{g['id']} {t['id']}: 无验收标准")
        print(json.dumps({"goals": len(goals), "problems": problems}, ensure_ascii=False, indent=2))
        sys.exit(0)
    if "tasks" == sys.argv[2]:
        rows = [{"goal": g["id"], **t} for g in goals for t in g["tasks"]]
        print(json.dumps({"tasks": rows,
                          "no_budget": [f"{r['goal']}/{r['id']}" for r in rows if r["budget_rounds"] is None]},
                         ensure_ascii=False, indent=2))
        sys.exit(0)
    print(json.dumps({
        "goals": [{"id": g["id"], "title": g["title"], "status": g["status"],
                   "tasks_total": len(g["tasks"]),
                   "tasks_done": sum(1 for t in g["tasks"] if t["done"]),
                   "no_budget": [t["id"] for t in g["tasks"] if t["budget_rounds"] is None],
                   "last_progress": g["progress"][-1]["date"] if g["progress"] else None}
                  for g in goals],
        "counts": {"total": len(goals),
                   "pending_approval": sum(1 for g in goals if g["status"] == "待批准"),
                   "approved": sum(1 for g in goals if g["status"] == "已批准"),
                   "rejected": sum(1 for g in goals if g["status"] == "已驳回")}
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
