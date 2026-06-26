#!/usr/bin/env python3
"""Debate state machine for multi-agent meetings.

Usage:
  # generate a session ID (ULID)
  python scripts/debate.py gen-ulid

  # initialize a debate session
  python scripts/debate.py init --work-dir <dir> --topic "<topic>" --agents A B C [--max-rounds 3]

  # per-round operations
  python scripts/debate.py --work-dir <dir> prepare --round N
  python scripts/debate.py --work-dir <dir> record --round N --agent <name> --response '<json>'
  python scripts/debate.py --work-dir <dir> check
  python scripts/debate.py --work-dir <dir> summary
"""

import argparse
import json
import os
import secrets
import sys
import time


# ---- ULID ----

CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"


def _ulid_time() -> str:
    ms = int(time.time() * 1000)
    chars = []
    for _ in range(10):
        chars.append(CROCKFORD[ms % 32])
        ms //= 32
    return "".join(reversed(chars))


def _ulid_random() -> str:
    return "".join(CROCKFORD[secrets.randbelow(32)] for _ in range(16))


def generate_ulid() -> str:
    """Generate a ULID (26-char Crockford Base32, time-sortable unique ID)."""
    return _ulid_time() + _ulid_random()


# ---- state management ----


def _state_path(work_dir):
    return os.path.join(work_dir, "state.json")


def _load_state(work_dir):
    with open(_state_path(work_dir)) as f:
        return json.load(f)


def _save_state(work_dir, state):
    os.makedirs(work_dir, exist_ok=True)
    with open(_state_path(work_dir), "w") as f:
        json.dump(state, f, indent=2, ensure_ascii=False)


# ---- commands ----


def cmd_gen_ulid(args):
    print(generate_ulid())


def cmd_init(args):
    agents = {}
    for name in args.agents:
        agents[name] = {"initial_position": {}, "rounds": {}}
    state = {
        "topic": args.topic,
        "agents": agents,
        "max_rounds": args.max_rounds,
        "current_round": 0,
        "status": "in_progress",
    }
    _save_state(args.work_dir, state)
    print(f"init ok: topic={args.topic!r} agents={list(agents.keys())}")


def cmd_prepare(args):
    state = _load_state(args.work_dir)
    r = args.round
    ctx_dir = os.path.join(args.work_dir, f"round-{r}")
    os.makedirs(ctx_dir, exist_ok=True)

    prev = {}
    for name in state["agents"]:
        rounds = []
        for i in range(1, r):
            s = str(i)
            if s in state["agents"][name]["rounds"]:
                rounds.append({s: state["agents"][name]["rounds"][s]})
        prev[name] = rounds

    for name in state["agents"]:
        others = {
            n: {"initial": d["initial_position"], "previous": prev[n]}
            for n, d in state["agents"].items()
            if n != name
        }
        ctx = {
            "topic": state["topic"],
            "round": r,
            "max_rounds": state["max_rounds"],
            "other_agents": others,
        }
        with open(os.path.join(ctx_dir, f"{name}.json"), "w") as f:
            json.dump(ctx, f, indent=2, ensure_ascii=False)

    state["current_round"] = r
    _save_state(args.work_dir, state)
    print(f"prepare round {r} ok: {len(state['agents'])} context files")


def cmd_record(args):
    state = _load_state(args.work_dir)
    if args.agent not in state["agents"]:
        print(f"error: unknown agent {args.agent!r}")
        sys.exit(1)
    try:
        resp = json.loads(args.response)
    except json.JSONDecodeError:
        resp = {"position": args.response}
    s = str(args.round)
    state["agents"][args.agent]["rounds"][s] = resp
    _save_state(args.work_dir, state)
    print(f"record round {args.round} / {args.agent} ok")


def cmd_check(args):
    state = _load_state(args.work_dir)
    if state["current_round"] == 0:
        print("diverged")
        return
    r = str(state["current_round"])
    positions = {}
    for name, d in state["agents"].items():
        if r in d["rounds"]:
            positions[name] = d["rounds"][r].get("position", "").lower().strip()
    if len(positions) < len(state["agents"]):
        print("diverged")
        return

    uniq = set(positions.values())
    if len(uniq) <= 1:
        state["status"] = "converged"
        _save_state(args.work_dir, state)
        print("converged")
        return

    prev_r = str(state["current_round"] - 1)
    if prev_r != "0":
        changed = False
        for name, d in state["agents"].items():
            if prev_r in d["rounds"] and r in d["rounds"]:
                old = d["rounds"][prev_r].get("position", "").lower().strip()
                new = d["rounds"][r].get("position", "").lower().strip()
                if old != new:
                    changed = True
                    break
        if not changed:
            state["status"] = "stalled"
            _save_state(args.work_dir, state)
            print("stalled")
            return

    if state["current_round"] >= state["max_rounds"]:
        state["status"] = "max_rounds_reached"
        _save_state(args.work_dir, state)
        print("max_rounds_reached")
        return

    print("diverged")


def cmd_summary(args):
    state = _load_state(args.work_dir)
    out = {
        "topic": state["topic"],
        "status": state["status"],
        "total_rounds": state["current_round"],
        "agents": {},
    }
    for name, d in state["agents"].items():
        out["agents"][name] = {
            "initial_position": d["initial_position"],
            "rounds": d["rounds"],
        }
    if state["current_round"] > 0:
        r = str(state["current_round"])
        out["final_positions"] = {
            name: d["rounds"].get(r, {}).get("position", "")
            for name, d in state["agents"].items()
            if r in d["rounds"]
        }
        uniq = set(v.lower().strip() for v in out["final_positions"].values())
        out["consensus"] = len(uniq) <= 1 if uniq else False
    print(json.dumps(out, indent=2, ensure_ascii=False))


# ---- main ----


def _add_global_args(parser):
    parser.add_argument("--work-dir", required=True, help="Debate session working directory")


def main():
    p = argparse.ArgumentParser(description="Debate state machine")
    sub = p.add_subparsers(dest="command", required=True)

    # gen-ulid
    sub.add_parser("gen-ulid")

    # init
    pi = sub.add_parser("init")
    _add_global_args(pi)
    pi.add_argument("--topic", required=True)
    pi.add_argument("--agents", nargs="+", required=True)
    pi.add_argument("--max-rounds", type=int, default=3)

    # prepare
    pp = sub.add_parser("prepare")
    _add_global_args(pp)
    pp.add_argument("--round", type=int, required=True)

    # record
    pr = sub.add_parser("record")
    _add_global_args(pr)
    pr.add_argument("--round", type=int, required=True)
    pr.add_argument("--agent", required=True)
    pr.add_argument("--response", required=True)

    # check
    pc = sub.add_parser("check")
    _add_global_args(pc)

    # summary
    ps = sub.add_parser("summary")
    _add_global_args(ps)

    args = p.parse_args()

    if args.command == "gen-ulid":
        cmd_gen_ulid(args)
    else:
        {
            "init": cmd_init,
            "prepare": cmd_prepare,
            "record": cmd_record,
            "check": cmd_check,
            "summary": cmd_summary,
        }[args.command](args)


if __name__ == "__main__":
    main()
