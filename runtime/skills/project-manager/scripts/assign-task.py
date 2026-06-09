#!/usr/bin/env python3
"""
Assign a recurring task to an agent — who, when, what.

Links schedule entries to graph tasks: pass the graph task_id as --session-id
so the agent can report back to the correct conversation thread.

Usage:
  python3 scripts/assign-task.py assign \\
      --agent @writer --task "Write blog post" --cron "0 0 9 * * 1"

  # Link to an existing graph task (recommended):
  python3 scripts/assign-task.py assign \\
      --agent @writer --task "Write blog post" --cron "0 0 9 * * 1" \\
      --session-id "task-a1b2c3d4" --project-dir /path/to/project

  python3 scripts/assign-task.py list
"""

import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from scheduler_client import MindXSchedulerClient, JobAddParams, DEFAULT_GATEWAY_HOST, DEFAULT_GATEWAY_PORT


def cmd_assign(args):
    params = JobAddParams(
        agent=args.agent,
        content=args.task,
        cron_expr=args.cron,
        session_id=args.session_id,
        project_dir=args.project_dir or "",
    )
    with MindXSchedulerClient(host=args.host, port=args.port) as client:
        result = client.add_job(params)
    if result.success:
        print(json.dumps({
            "status": "assigned",
            "agent": args.agent,
            "task": args.task,
            "cron": args.cron,
            "task_id": result.task_id,
            "session_id": result.session_id,
        }, indent=2, ensure_ascii=False))
        return 0
    else:
        print(f"Failed: {result.error}", file=sys.stderr)
        return 1


def cmd_list(args):
    with MindXSchedulerClient(host=args.host, port=args.port) as client:
        result = client.list_jobs()
    if result.success:
        entries = result.data
        if isinstance(entries, list):
            if args.json:
                print(json.dumps(entries, indent=2, ensure_ascii=False))
            elif not entries:
                print("No assignments.")
            else:
                print(f"\nScheduled tasks ({len(entries)}):")
                print("=" * 60)
                for entry in entries:
                    status = "active" if entry.get("enabled", False) else "paused"
                    print(f"  ID:      {entry.get('id', '')}")
                    print(f"  Agent:   {entry.get('agent', '')}")
                    print(f"  Content: {entry.get('content', '')[:80]}")
                    print(f"  Cron:    {entry.get('cron_expr', '')}")
                    print(f"  Status:  {status}")
                    print(f"  Session: {entry.get('session_id', '') or 'N/A'}")
                    print("-" * 60)
        else:
            print(entries)
        return 0
    else:
        print(f"Failed: {result.error}", file=sys.stderr)
        return 1


def main():
    parser = argparse.ArgumentParser(description="Assign recurring tasks to agents")
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("assign", help="Assign a recurring task")
    p.add_argument("--agent", required=True, help="Target agent (e.g. @writer)")
    p.add_argument("--task", required=True, help="Task description / prompt")
    p.add_argument("--cron", required=True, help="6-field cron expression")
    p.add_argument("--session-id", default="",
                   help="Graph task_id to use as session (recommended; omit to auto-generate)")
    p.add_argument("--project-dir", default="", help="Project directory")
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    p = sub.add_parser("list", help="List all assignments")
    p.add_argument("--json", action="store_true", help="JSON output")
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    args = parser.parse_args()
    if args.command == "assign":
        exit(cmd_assign(args))
    elif args.command == "list":
        exit(cmd_list(args))


if __name__ == "__main__":
    main()
