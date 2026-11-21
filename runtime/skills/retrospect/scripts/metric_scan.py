#!/usr/bin/env python3
"""决策量表读侧：把 fail / rework 判定捞出来，供自省聚类成技能候选。

用法:
    metric_scan.py failures [--data ~/.mindx/data] [--window-days 90] [--json]
    metric_scan.py stats    [--data ~/.mindx/data]

为什么单独写这个读侧：
    量表写入侧（metric.record）已通，但判定正文 JudgmentRecord.Note 在 Go 代码里
    没有任何消费方——聚合层只取 Value 算成功率，Note 永远没人读。技能蒸馏需要的
    恰恰是 Note 里的四段式原料（问题/否决/修正/验证），不是那个比率数字。
    本脚本补上这个缺口：不改 Go 代码，直接读量表的 YAML 文件。

输出约定：
    JSON 走 stdout，进度与告警走 stderr。
    退出码 0 成功 / 1 用法错误 / 2 数据源不可用。
依赖：
    仅标准库。量表 YAML 是本项目自产（全量读写、无锚点无别名、缩进仅 2/4/6 层），
    因此用行级状态机解析，不引入 pyyaml。
"""
import argparse
import json
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timedelta
from pathlib import Path

# 四段式字段：技能履历要求「问题/否决/修正/验证」，缺任一段的 note 无法直接成稿
FOUR_PARTS = {
    "问题": re.compile(r"问题[：:]"),
    "否决": re.compile(r"否决[：:]"),
    "修正": re.compile(r"修正[：:]"),
    "验证": re.compile(r"验证[：:]"),
}
JUDGED_TO_FIX = {"fail", "rework"}


def unquote(v):
    """YAML 标量去引号。只处理首尾同种引号，不做转义还原（本表自产、无转义）。"""
    v = v.strip()
    if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
        return v[1:-1]
    return v


def split_kv(line):
    """按第一个冒号切成 key / value。

    必须只切第一个：task_hint 的值里含中英文冒号（实测 216 条里有 36 条），
    用 rsplit 或全切都会把值截断。
    """
    k, _, v = line.partition(":")
    return k.strip(), unquote(v)


# 块标量起始符：| / |- / |+ / > / >- / >+
BLOCK_RE = re.compile(r"^[|>][+-]?$")


def parse_events(path):
    """行级状态机解析 metric_events.yml。

    需要处理三种结构（均由本项目 Go 侧 yaml.v3 序列化产生，已实测）：
        1. 标量：        kind: round
        2. 嵌套列表：    skills_used:  \\n    - agent-creator
        3. 块标量：      note: |-      \\n        问题：…   （多行 note 的默认形态）

    块标量是重点：四段式 note 必然多行，yaml.v3 一律以 `|-` 输出。
    不处理它会把最有价值的素材读成字面量 "|-"。

    返回事件 dict 列表；judgments / skills_used 为 list。
    """
    events, cur = [], None
    # 当前处于哪个多行容器：None / "judgments" / "skills" / "block"
    section = None
    block_lines, block_key, block_target = [], None, None

    def flush_block():
        nonlocal block_lines, block_key, block_target
        if block_key is not None and block_target is not None:
            block_target[block_key] = "\n".join(block_lines)
        block_lines, block_key, block_target = [], None, None

    for raw in path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = raw.rstrip()
        if not line.strip() or line.lstrip().startswith("#"):
            # 空行属于块标量内容（段落分隔），只在块模式下保留
            if section == "block" and block_key is not None:
                block_lines.append("")
            continue
        stripped = line.lstrip()
        indent = len(line) - len(stripped)

        # 块标量续行：缩进比声明行深，且不是新的 key
        if section == "block":
            # 声明块的行 indent 记在 block_indent；续行必须更深
            if indent > block_indent and ":" not in stripped.partition(" ")[0]:
                block_lines.append(stripped)
                continue
            if indent > block_indent:
                block_lines.append(stripped)
                continue
            flush_block()
            section = None

        if indent == 0 and stripped.startswith("- "):
            if cur:
                events.append(cur)
            cur = {"judgments": [], "skills_used": []}
            section = None
            k, v = split_kv(stripped[2:])
            cur[k] = v
            continue
        if cur is None:
            continue

        # 事件级键（indent == 2）永远优先于任何未闭合的列表段：
        # 列表段只能靠「回到 indent 2」或「遇到新顶层项」结束。
        if indent == 2:
            section = None
            k, v = split_kv(stripped)
            if k == "judgments" and v == "":
                section = "judgments"
            elif k == "skills_used" and v == "":
                section = "skills"
            elif BLOCK_RE.match(v):
                section = "block"
                block_key, block_target, block_indent, block_lines = k, cur, indent, []
            else:
                cur[k] = v
            continue

        # indent >= 4：只可能在列表段或块标量内
        if section == "judgments":
            if stripped.startswith("- "):
                cur["judgments"].append({})
                k, v = split_kv(stripped[2:])
                cur["judgments"][-1][k] = v
            elif cur["judgments"]:
                k, v = split_kv(stripped)
                if BLOCK_RE.match(v):
                    section = "block"
                    block_key, block_target = k, cur["judgments"][-1]
                    block_indent, block_lines = indent, []
                else:
                    cur["judgments"][-1][k] = v
        elif section == "skills":
            if stripped.startswith("- "):
                cur["skills_used"].append(unquote(stripped[2:]))
            else:
                k, v = split_kv(stripped)
                if BLOCK_RE.match(v):
                    section = "block"
                    block_key, block_target, block_indent, block_lines = k, cur, indent, []
                else:
                    cur[k] = v
        continue

    if section == "block":
        flush_block()
    if cur:
        events.append(cur)
    return events


# 判定来源层级（与 Go 侧 OutcomeSource.rank 同口径，白皮书 7.4）：
# 用户结构化反馈 > 用户文本感受 > Agent 上报 / 返工回溯。
# 层内按 at 取最新：追溯修正天然在后，故能覆盖 Agent 早先的乐观 success。
SOURCE_RANK = {
    "user_feedback": 3,
    "user_text": 2,
    "agent_report": 1,
    "rework_trace": 1,
}


def latest_judgment(ev):
    """取生效判定：先比来源层级，同级再比 at 最新（与 Go 侧 LatestJudgment 同口径）。

    只按 at 取最新是错的——一条晚到的 Agent 上报会抹掉用户当时的明确判定，
    「判定优先级」就成了空话。读侧必须与写侧同规则，否则复盘看到的
    「哪次失败了」和量表里真正生效的判定会不一致。
    """
    js = ev.get("judgments") or []
    if not js:
        return None

    def at_key(j):
        try:
            return datetime.fromisoformat(j.get("at", ""))
        except (TypeError, ValueError):
            return datetime.min

    def rank(j):
        return SOURCE_RANK.get(str(j.get("source", "")), 0)

    return max(js, key=lambda j: (rank(j), at_key(j)))


def parts_present(note):
    """返回 note 里命中的四段式字段名列表。"""
    return [name for name, pat in FOUR_PARTS.items() if pat.search(note or "")]


def collect_failures(events, window_days):
    """捞出判定为 fail/rework 的事件，附四段式完整度。

    只取 round 事件：overall 是对 Agent/Team/Skill 的整体评价，不含任务级 note。
    同一会话可能有多次判定（版本链），只取生效的那条。
    """
    since = datetime.now().astimezone() - timedelta(days=window_days) if window_days > 0 else None
    out = []
    for ev in events:
        if ev.get("kind") != "round":
            continue
        j = latest_judgment(ev)
        if not j or j.get("value") not in JUDGED_TO_FIX:
            continue
        if since:
            try:
                ts = datetime.fromisoformat(ev.get("ts", ""))
                if ts.tzinfo is None:
                    ts = ts.astimezone()
                if ts < since:
                    continue
            except (TypeError, ValueError):
                pass  # 时间解析失败不因此丢弃素材
        note = j.get("note", "") or ""
        out.append({
            "session_id": ev.get("session_id", ""),
            "agent": ev.get("agent_name", ""),
            "ts": ev.get("ts", ""),
            "task": (ev.get("task_hint") or "")[:80],
            "outcome": j.get("value"),
            "source": j.get("source", ""),
            "note": note,
            "parts": parts_present(note),
            "ready": len(parts_present(note)) == 4,
        })
    out.sort(key=lambda x: x["ts"], reverse=True)
    return out


def cmd_failures(args):
    path = Path(args.data).expanduser() / "metric_events.yml"
    if not path.is_file():
        print(json.dumps({"error": f"量表文件不存在：{path}"}, ensure_ascii=False), file=sys.stderr)
        return 2
    try:
        events = parse_events(path)
    except OSError as e:
        print(json.dumps({"error": f"量表读取失败：{e}"}, ensure_ascii=False), file=sys.stderr)
        return 2

    fails = collect_failures(events, args.window_days)
    by_parts = Counter(len(f["parts"]) for f in fails)
    result = {
        "source": str(path),
        "events_total": len(events),
        "round_total": sum(1 for e in events if e.get("kind") == "round"),
        "judged_total": sum(1 for e in events if e.get("kind") == "round" and latest_judgment(e)),
        "fail_count": len(fails),
        "ready_count": sum(1 for f in fails if f["ready"]),
        "parts_distribution": {f"{k}段": v for k, v in sorted(by_parts.items())},
        "items": fails,
    }
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0

    print(f"量表：{result['events_total']} 事件 / round {result['round_total']} / 有判定 {result['judged_total']}")
    print(f"失败判定：{result['fail_count']} 条（四段齐全 {result['ready_count']} 条）")
    if by_parts:
        print("四段式完整度：" + "  ".join(f"{k}段={v}" for k, v in sorted(by_parts.items())))
    if not fails:
        print("\n无可提炼素材：量表里没有 fail/rework 判定，或判定没带 note。")
        return 0
    for f in fails:
        mark = "✓" if f["ready"] else "·"
        missing = [p for p in FOUR_PARTS if p not in f["parts"]]
        print(f"\n{mark} [{f['outcome']}/{f['source']}] {f['ts'][:19]} {f['agent']}")
        print(f"  任务：{f['task']}")
        print(f"  note：{f['note'][:200] or '(空)'}")
        if missing:
            print(f"  缺段：{'/'.join(missing)}")
    return 0


def cmd_stats(args):
    path = Path(args.data).expanduser() / "metric_events.yml"
    if not path.is_file():
        print(json.dumps({"error": f"量表文件不存在：{path}"}, ensure_ascii=False), file=sys.stderr)
        return 2
    events = parse_events(path)
    kinds = Counter(e.get("kind", "?") for e in events)
    outcome = Counter()
    source = Counter()
    for ev in events:
        j = latest_judgment(ev)
        if j:
            outcome[j.get("value", "?")] += 1
            source[j.get("source", "?")] += 1
    print(json.dumps({
        "source": str(path),
        "total": len(events),
        "by_kind": dict(kinds),
        "by_outcome": dict(outcome),
        "by_source": dict(source),
        "skills_used_events": sum(1 for e in events if e.get("skills_used") != ""),
    }, ensure_ascii=False, indent=2))
    return 0


def main():
    ap = argparse.ArgumentParser(description="决策量表读侧：捞失败判定供技能蒸馏")
    sub = ap.add_subparsers(dest="cmd")
    for name in ("failures", "stats"):
        p = sub.add_parser(name)
        p.add_argument("--data", default="~/.mindx/data", help="量表数据目录（默认 ~/.mindx/data）")
        p.add_argument("--window-days", type=int, default=90, help="统计窗口天数（0=不限，默认 90）")
        p.add_argument("--json", action="store_true", help="输出 JSON")
    args = ap.parse_args()
    if not args.cmd:
        ap.print_help()
        return 1
    return cmd_failures(args) if args.cmd == "failures" else cmd_stats(args)


if __name__ == "__main__":
    sys.exit(main())