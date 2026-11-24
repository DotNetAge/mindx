#!/usr/bin/env python3
"""自省清点脚本：盘会话债、盘素材存量、判阈值、体检命名（确定性操作不裸算）。

用法:
    scan.py status  [--agents-dir .agents] [--threshold 100000] [--notes-threshold 30]
        一次给出全景：会话债务 + 素材存量 + 是否触发 + 建议模式（JSON）
    scan.py debt    [--threshold 100000] [--project-dir .] [--current-session-id ID]
        只列超阈会话：session_id / agent / title / window_tokens（JSON）
    scan.py stock   [--agents-dir .agents] [--notes-threshold 30]
        只盘素材：条数、按分类分组、命名与分类词表体检（JSON）；notes-threshold 只是项目总量提醒，不参与本 Agent 提纯判据

输出: JSON 走 stdout；进度与告警走 stderr；退出码 0 成功 / 1 用法错误 / 2 数据源不可用。
依赖: python3 标准库；debt 子命令需 mindx CLI（解析顺序 MINDX_BIN > PATH > ~/.mindx/bin）与 daemon（不可用时该部分返回 error，stock 照常）。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from collections import Counter
from datetime import date, datetime
from pathlib import Path

CAT_RE = None  # 由 --categories 初始化
DATE_RE = re.compile(r"^(\d{4}-\d{2}-\d{2}|\d{8})")
SUMMARY_RE = re.compile(r"^(\d{8})-\d{6}-")  # 压缩摘要自动落盘的命名
IN_USE_MINUTES = 30  # 只用来判断「是否正在使用」，不用于推测缓存有效期


def mindx_exe():
    """解析 mindx CLI：环境变量 > PATH > ~/.mindx/bin。"""
    if os.environ.get("MINDX_BIN"):
        return os.environ["MINDX_BIN"]
    found = shutil.which("mindx")
    if found:
        return found
    fallback = Path.home() / ".mindx" / "bin" / "mindx"
    return str(fallback) if fallback.exists() else "mindx"


MINDX = mindx_exe()


def run_json(cmd):
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.TimeoutExpired) as e:
        return None, str(e)
    if out.returncode != 0:
        return None, (out.stderr or out.stdout).strip()[:200]
    for line in reversed(out.stdout.strip().splitlines()):
        line = line.strip()
        if line.startswith("[") or line.startswith("{"):
            try:
                return json.loads(line), None
            except json.JSONDecodeError:
                continue
    return None, "无法解析 CLI 输出"


def idle_hours(ts):
    """距最后活动多少小时；解析失败返回 None，调用方必须按「未知活动」defer。"""
    if not ts:
        return None
    try:
        last = datetime.fromisoformat(ts)
    except (TypeError, ValueError):
        return None
    now = datetime.now(last.tzinfo) if last.tzinfo else datetime.now()
    return round((now - last).total_seconds() / 3600, 1)


def same_project(path, project_dir):
    """比较会话 project_dir 与当前项目根；空值或解析失败一律不匹配。"""
    if not path or not project_dir:
        return False
    try:
        return os.path.realpath(os.path.expanduser(path)) == os.path.realpath(
            os.path.expanduser(project_dir))
    except OSError:
        return False


def scan_debt(threshold, project_dir=None, current_session_id=None):
    sessions, err = run_json([MINDX, "session", "list", "--json"])
    if sessions is None:
        return {"error": f"会话清单不可用：{err}", "threshold": threshold,
                "checked": 0, "over": [], "max_tokens": None}
    items = sessions if isinstance(sessions, list) else sessions.get("sessions", [])
    all_items = list(items)
    if project_dir:
        items = [s for s in items if same_project(s.get("project_dir"), project_dir)]
    over, errors, peak = [], [], 0
    for s in items:
        sid = s.get("session_id")
        if not sid:
            continue
        ctx, cerr = run_json([MINDX, "session", "context", "--session-id", sid, "--json"])
        if ctx is None:
            errors.append({"session_id": sid, "error": cerr})
            continue
        tokens = ctx.get("window_tokens") or 0
        peak = max(peak, tokens)
        if tokens > threshold:
            idle = idle_hours(s.get("last_activity_at"))
            if current_session_id and sid == current_session_id:
                in_use, reason = True, "当前会话：轮内压会丢掉本轮后续上下文"
            elif idle is None:
                in_use, reason = True, "最后活动未知：无法确认空闲，保守不压"
            elif idle < IN_USE_MINUTES / 60:
                in_use, reason = True, "最近有活动，可能正在使用；本轮不压，登记为待压"
            else:
                in_use, reason = False, None
            over.append({"session_id": sid, "agent": s.get("agent_name"),
                         "title": (s.get("title") or "")[:40],
                         "window_tokens": tokens,
                         "usage_ratio": ctx.get("usage_ratio"),
                         "cursor": ctx.get("cursor"),
                         "last_activity": s.get("last_activity_at"),
                         "idle_hours": idle,
                         "in_use": in_use,
                         "action": "defer" if in_use else "compact",
                         "compact_command": f"{MINDX} session compact --session-id {sid} --yes",
                         "skip_reason": reason})
    over.sort(key=lambda x: -(x["window_tokens"] - threshold))
    for x in over:
        x["over_by"] = x["window_tokens"] - threshold
    result = {"threshold": threshold, "project_dir": project_dir,
              "session_total": len(all_items),
              "current_session_id": current_session_id,
              "checked": len(items) - len(errors),
              "over": over, "over_count": len(over),
              "actionable_count": sum(1 for x in over if not x["in_use"]),
              "deferred_count": sum(1 for x in over if x["in_use"]),
              "deferred_batch": [{"session_id": x["session_id"],
                                  "window_tokens": x["window_tokens"],
                                  "compact_command": x["compact_command"]}
                                 for x in over if x["in_use"]],
              "over_total": sum(x["over_by"] for x in over),
              "max_tokens": peak}
    if errors:
        result["errors"] = errors[:5]
    return result


def init_categories(words):
    global CAT_RE
    body = "|".join(sorted({re.escape(w) for w in words}, key=len, reverse=True))
    CAT_RE = re.compile(rf"-({body}|缺口-[^-]+)-")


def classify(name):
    m = CAT_RE.search(name)
    return m.group(1) if m else None




def scan_dir(d, kind, is_threshold=None):
    if not d.is_dir():
        return {"count": 0, "by_category": {}, "unclassified": [], "issues": [f"目录不存在：{d}"],
                "issue_total": 0, "oldest": None, "newest": None}
    files = sorted(d.glob("*.md"))
    issues, cats, dates = [], Counter(), []
    for p in files:
        cat = classify(p.name)
        if cat:
            cats[cat] += 1
        else:
            issues.append(p.name)
        m = DATE_RE.match(p.name)
        if m:
            raw = m.group(1)
            dates.append(raw if "-" in raw else f"{raw[:4]}-{raw[4:6]}-{raw[6:]}")
    return {"count": len(files), "by_category": dict(cats.most_common()),
            "unclassified": issues[:8], "issue_total": len(issues),
            "oldest": min(dates) if dates else None,
            "newest": max(dates) if dates else None}


def scan_memory(d):
    """memory 层有两类文件：压缩摘要（YYYYMMDD-HHMMSS- 前缀，系统自动落盘）与提炼条目（人工/自省写入）。"""
    if not d.is_dir():
        return {"total": 0, "curated": {"count": 0, "by_category": {}, "unclassified": [],
                "issue_total": 0, "oldest": None, "newest": None},
                "summaries": {"count": 0, "oldest": None, "newest": None},
                "issues": [f"目录不存在：{d}"]}
    files = sorted(d.glob("*.md"))
    curated = [p for p in files if not SUMMARY_RE.match(p.name)]
    sums = [p for p in files if SUMMARY_RE.match(p.name)]

    cats, issues, dates = Counter(), [], []
    for p in curated:
        cat = classify(p.name)
        if cat:
            cats[cat] += 1
        else:
            issues.append(p.name)
        m = DATE_RE.match(p.name)
        if m:
            raw = m.group(1)
            dates.append(raw if "-" in raw else f"{raw[:4]}-{raw[4:6]}-{raw[6:]}")
    sd = [f"{m.group(1)[:4]}-{m.group(1)[4:6]}-{m.group(1)[6:]}" for p in sums if (m := SUMMARY_RE.match(p.name))]
    return {"total": len(files),
            "curated": {"count": len(curated), "by_category": dict(cats.most_common()),
                        "unclassified": issues[:8], "issue_total": len(issues),
                        "oldest": min(dates) if dates else None,
                        "newest": max(dates) if dates else None},
            "summaries": {"count": len(sums),
                          "oldest": min(sd) if sd else None,
                          "newest": max(sd) if sd else None}}


def scan_stock(agents_dir, notes_threshold):
    root = Path(agents_dir)
    notes = scan_dir(root / "notes", "notes")
    memory = scan_memory(root / "memory")
    return {"notes": notes, "memory": memory,
            "notes_threshold": notes_threshold,
            "notes_over_threshold": notes["count"] >= notes_threshold}








def cmd_status(args):
    debt = scan_debt(args.threshold, args.project_dir, args.current_session_id)
    stock = scan_stock(args.agents_dir, args.notes_threshold)
    own = count_own(args.agents_dir, args.agent)
    over = debt.get("over") or []
    actionable = [x for x in over if not x.get("in_use")]
    deferred = [x for x in over if x.get("in_use")]
    batch = actionable[:args.max_compact]

    reasons, mode = [], "无需自省"
    if over:
        reasons.append(f"{len(over)} 个会话超 {args.threshold} tokens"
                       f"（总超出 {sum(x['over_by'] for x in over):,}；"
                       f"本轮可压 {len(batch)} 个，待会话空闲 {len(deferred)} 个）")
    curate = own >= args.max_notes
    if curate:
        reasons.append(f"本 Agent notes {own} 条 ≥ {args.max_notes}")
    if over and curate:
        mode = "Full" if actionable else "Full（只提纯，债务待压）"
    elif over:
        mode = "Light（只减债）" if actionable else "Light（无空闲会话，债务待压）"
    elif curate:
        mode = "Full（只提纯）"

    out = {
        "trigger": {"triggered": bool(reasons), "reasons": reasons},
        "mode_suggest": mode,
        "compact_batch": [x["session_id"] for x in batch],
        "deferred_batch": [{"session_id": x["session_id"], "window_tokens": x["window_tokens"],
                            "compact_command": x["compact_command"]} for x in deferred],
        "own_notes": own,
        "debt": debt,
        "stock": {k: v for k, v in stock.items()
                  if k not in ("notes_threshold", "notes_over_threshold")},
        "naming_issues": stock["notes"].get("issue_total", 0) + stock["memory"]["curated"].get("issue_total", 0),
        "today": date.today().isoformat(),
    }
    print(json.dumps(out, ensure_ascii=False, indent=2))
    return 0


def is_own_note(name, agent):
    """文件名是否为该 Agent 的 notes：<日期>-<agent>-<分类>-<标题>.md。

    兼容无日期前缀的旧命名；有日期时必须按「日期-agent-分类」整段匹配，
    避免 agent 是另一个 agent 名的前缀时误计（如 go 误吞 go-programmer）。
    """
    if name.startswith(agent + "-"):
        return bool(CAT_RE.match("-" + name[len(agent) + 1:]))
    m = DATE_RE.match(name)
    if not m:
        return False
    tail = name[m.end():]
    if not tail.startswith("-" + agent + "-"):
        return False
    after_agent = tail[len(agent) + 2:]
    return bool(CAT_RE.match("-" + after_agent))


def count_own(agents_dir, agent):
    d = Path(agents_dir) / "notes"
    if not d.is_dir():
        return 0
    files = list(d.glob("*.md"))
    if not agent:
        return len(files)
    return sum(1 for p in files if is_own_note(p.name, agent))


def cmd_count(args):
    own = count_own(args.agents_dir, args.agent)
    trig = own >= args.max_notes
    print(json.dumps({"agent": args.agent or "(未指定，按全部计)", "own_notes": own,
                      "max_notes": args.max_notes, "trigger": trig,
                      "hint": (f"本 Agent {own} 条 ≥ {args.max_notes}，该自省了" if trig
                               else f"本 Agent {own}/{args.max_notes}，继续攒")},
                     ensure_ascii=False, indent=2))
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True, description="自省清点：盘会话债、盘素材存量、判阈值、体检命名")
    sub = ap.add_subparsers(dest="cmd")

    for name, fn in (("status", cmd_status), ("debt", None), ("stock", None),
                     ("count", cmd_count)):
        p = sub.add_parser(name)
        p.add_argument("--agents-dir", default=".agents", help="项目 .agents 目录（默认 .agents）")
        p.add_argument("--threshold", type=int, default=100000, help="会话窗口 token 阈值（默认 100000）")
        p.add_argument("--notes-threshold", type=int, default=30,
                       help="项目 notes 总量提醒阈值（默认 30，不参与本 Agent 提纯判据）")
        p.add_argument("--categories", default="决策,踩坑,经验,事实,待办,偏好",
                       help="合法分类词表，逗号分隔（默认 决策,踩坑,经验,事实,待办,偏好）")
        p.add_argument("--max-notes", type=int, default=20, help="本 Agent notes 条数上限（默认 20）")
        p.add_argument("--agent", default="", help="本 Agent 名，用于只数自己的 notes（默认按全部计）")
        p.add_argument("--max-compact", type=int, default=3, help="单轮最多压几个会话（默认 3，按超出量降序）")
        p.add_argument("--project-dir", default=os.getcwd(),
                       help="项目根目录（默认当前目录）；debt/status 只扫该目录下的会话")
        p.add_argument("--current-session-id", default=os.environ.get("MINDX_SESSION_ID", ""),
                       help="当前会话 id（默认读 MINDX_SESSION_ID）；该会话永远 defer")
        p.set_defaults(run=fn)

    args = ap.parse_args()
    if not args.cmd:
        ap.print_help()
        return 1
    init_categories(c.strip() for c in args.categories.split(",") if c.strip())

    if args.cmd == "debt":
        d = scan_debt(args.threshold, args.project_dir, args.current_session_id)
        print(json.dumps(d, ensure_ascii=False, indent=2))
        return 2 if d.get("error") else 0
    if args.cmd == "count":
        return cmd_count(args)
    if args.cmd == "stock":
        print(json.dumps(scan_stock(args.agents_dir, args.notes_threshold), ensure_ascii=False, indent=2))
        return 0
    return cmd_status(args)


if __name__ == "__main__":
    sys.exit(main())
