#!/usr/bin/env python3
"""
project-tool.py — local data management helper for project-tracker.

It calls the mindx kv CLI for persistence.
"""

import argparse
import json
import os
import re
import subprocess
import sys
import uuid
from datetime import datetime
from typing import Optional

KV_PREFIX = "project-tracker"
DATE_FMT = "%Y-%m-%d"
DT_FMT = "%Y-%m-%d %H:%M:%S"


def now() -> str:
    return datetime.now().strftime(DT_FMT)


def today() -> str:
    return datetime.now().strftime(DATE_FMT)


# ---------------------------------------------------------------------------
# mindx kv CLI helpers
#
# Requires a mindx version that supports --json (v2.3.14+).
# The MINDX_BIN environment variable can be used to specify the mindx binary.
# ---------------------------------------------------------------------------

MINDX_BIN = os.environ.get("MINDX_BIN", "mindx")


def run_kv(args: list[str]) -> subprocess.CompletedProcess:
    return subprocess.run([MINDX_BIN, "kv", *args], capture_output=True, text=True)


def _find_json_line(text: str) -> Optional[dict]:
    """Find a JSON object line in output mixed with logs or panics."""
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("{") and line.endswith("}"):
            try:
                return json.loads(line)
            except json.JSONDecodeError:
                continue
    return None


def kv_get(key: str) -> Optional[dict]:
    result = run_kv(["get", "--json", "--key", key])
    data = _find_json_line(result.stdout)
    if not isinstance(data, dict):
        return None
    if not data.get("found"):
        return None
    return data.get("item", {}).get("value")


def kv_set(key: str, value: dict) -> bool:
    result = run_kv(["set", "--key", key, "--value", json.dumps(value, ensure_ascii=False)])
    data = _find_json_line(result.stdout)
    return isinstance(data, dict) and data.get("status") == "ok"


def kv_delete(key: str) -> bool:
    result = run_kv(["delete", "--key", key])
    data = _find_json_line(result.stdout)
    return isinstance(data, dict) and data.get("status") == "ok"


def kv_list(prefix: str, with_values: bool = True) -> list[dict]:
    args = ["list", "--json", "--prefix", prefix]
    if with_values:
        args.append("--with-values")
    result = run_kv(args)
    data = _find_json_line(result.stdout)
    if not isinstance(data, dict):
        return []
    if with_values:
        return data.get("items", []) or []
    return [{"key": k} for k in (data.get("keys", []) or [])]


def kv_clear(prefix: str) -> bool:
    result = run_kv(["clear", "--prefix", prefix])
    data = _find_json_line(result.stdout)
    return isinstance(data, dict) and data.get("status") == "ok"


# ---------------------------------------------------------------------------
# ID / key helpers
# ---------------------------------------------------------------------------

def make_slug(name: str) -> str:
    s = re.sub(r"[^\w\s-]", "", name.lower())
    s = re.sub(r"[-\s]+", "-", s).strip("-")
    return s[:64] or "project"


def uniq_project_id(name: str) -> str:
    slug = make_slug(name)
    existing = {item["key"].split(":")[2] for item in kv_list(f"{KV_PREFIX}:project:", False)}
    if slug not in existing:
        return slug
    return f"{slug}-{uuid.uuid4().hex[:6]}"


def uniq_id(prefix: str) -> str:
    return f"{prefix}-{uuid.uuid4().hex[:8]}"


def project_meta_key(project_id: str) -> str:
    return f"{KV_PREFIX}:project:{project_id}:meta"


def goal_key(project_id: str, goal_id: str) -> str:
    return f"{KV_PREFIX}:project:{project_id}:goal:{goal_id}"


def task_key(project_id: str, task_id: str) -> str:
    return f"{KV_PREFIX}:project:{project_id}:task:{task_id}"


def log_key(project_id: str, date: str) -> str:
    return f"{KV_PREFIX}:project:{project_id}:log:{date}"


# ---------------------------------------------------------------------------
# Project
# ---------------------------------------------------------------------------

def cmd_project_create(args: argparse.Namespace) -> int:
    project_id = args.id or uniq_project_id(args.name)
    meta = {
        "id": project_id,
        "name": args.name,
        "description": args.description or "",
        "start_date": args.start_date or today(),
        "end_date": args.end_date or "",
        "acceptance_criteria": args.acceptance_criteria or "",
        "is_recurring": args.recurring,
        "status": "active",
        "created_at": now(),
        "updated_at": now(),
    }
    if kv_set(project_meta_key(project_id), meta):
        print(json.dumps(meta, ensure_ascii=False, indent=2))
        return 0
    print("error: failed to create project", file=sys.stderr)
    return 1


def cmd_project_list(args: argparse.Namespace) -> int:
    items = kv_list(f"{KV_PREFIX}:project:", with_values=True)
    projects = []
    for item in items:
        parts = item["key"].split(":")
        if len(parts) >= 4 and parts[3] == "meta":
            projects.append(item["value"])
    print(json.dumps(projects, ensure_ascii=False, indent=2))
    return 0


def cmd_project_get(args: argparse.Namespace) -> int:
    meta = kv_get(project_meta_key(args.id))
    if meta is None:
        print(f"error: project '{args.id}' not found", file=sys.stderr)
        return 1
    print(json.dumps(meta, ensure_ascii=False, indent=2))
    return 0


def cmd_project_update(args: argparse.Namespace) -> int:
    meta = kv_get(project_meta_key(args.id))
    if meta is None:
        print(f"error: project '{args.id}' not found", file=sys.stderr)
        return 1
    allowed = {"name", "description", "end_date", "acceptance_criteria", "status"}
    if args.field not in allowed:
        print(f"error: field must be one of {allowed}", file=sys.stderr)
        return 1
    meta[args.field] = args.value
    meta["updated_at"] = now()
    if kv_set(project_meta_key(args.id), meta):
        print(json.dumps(meta, ensure_ascii=False, indent=2))
        return 0
    return 1


def cmd_project_delete(args: argparse.Namespace) -> int:
    prefix = f"{KV_PREFIX}:project:{args.id}:"
    if kv_clear(prefix):
        print(json.dumps({"project_id": args.id, "status": "deleted"}, ensure_ascii=False))
        return 0
    print(f"error: failed to delete project '{args.id}'", file=sys.stderr)
    return 1


# ---------------------------------------------------------------------------
# Goal
# ---------------------------------------------------------------------------

def cmd_goal_add(args: argparse.Namespace) -> int:
    if kv_get(project_meta_key(args.project)) is None:
        print(f"error: project '{args.project}' not found", file=sys.stderr)
        return 1
    goal_id = args.id or uniq_id("goal")
    goal = {
        "id": goal_id,
        "project_id": args.project,
        "title": args.title,
        "acceptance_criteria": args.acceptance_criteria or "",
        "status": "pending",
        "created_at": now(),
    }
    if kv_set(goal_key(args.project, goal_id), goal):
        print(json.dumps(goal, ensure_ascii=False, indent=2))
        return 0
    return 1


def cmd_goal_list(args: argparse.Namespace) -> int:
    items = kv_list(f"{KV_PREFIX}:project:{args.project}:goal:", with_values=True)
    goals = [item["value"] for item in items]
    print(json.dumps(goals, ensure_ascii=False, indent=2))
    return 0


def cmd_goal_update(args: argparse.Namespace) -> int:
    goal = kv_get(goal_key(args.project, args.id))
    if goal is None:
        print(f"error: goal '{args.id}' not found", file=sys.stderr)
        return 1
    if args.title:
        goal["title"] = args.title
    if args.acceptance_criteria:
        goal["acceptance_criteria"] = args.acceptance_criteria
    if args.status:
        goal["status"] = args.status
    goal["updated_at"] = now()
    if kv_set(goal_key(args.project, args.id), goal):
        print(json.dumps(goal, ensure_ascii=False, indent=2))
        return 0
    return 1


# ---------------------------------------------------------------------------
# Task
# ---------------------------------------------------------------------------

def cmd_task_add(args: argparse.Namespace) -> int:
    if kv_get(project_meta_key(args.project)) is None:
        print(f"error: project '{args.project}' not found", file=sys.stderr)
        return 1
    task_id = args.id or uniq_id("task")
    task = {
        "id": task_id,
        "project_id": args.project,
        "description": args.description,
        "agent": args.agent or "",
        "cron": args.cron or "",
        "schedule_id": args.schedule_id or "",
        "due_date": args.due_date or "",
        "check_criteria": args.check_criteria or "",
        "depends_on": args.depends_on.split(",") if args.depends_on else [],
        "status": "pending",
        "max_retries": args.max_retries,
        "retry_count": 0,
        "result_summary": "",
        "failure_reason": "",
        "created_at": now(),
    }
    if kv_set(task_key(args.project, task_id), task):
        print(json.dumps(task, ensure_ascii=False, indent=2))
        return 0
    return 1


def cmd_task_list(args: argparse.Namespace) -> int:
    items = kv_list(f"{KV_PREFIX}:project:{args.project}:task:", with_values=True)
    tasks = [item["value"] for item in items]
    if args.status:
        tasks = [t for t in tasks if t.get("status") == args.status]
    if args.due_date:
        tasks = [t for t in tasks if t.get("due_date") == args.due_date]
    print(json.dumps(tasks, ensure_ascii=False, indent=2))
    return 0


def cmd_task_get(args: argparse.Namespace) -> int:
    task = kv_get(task_key(args.project, args.id))
    if task is None:
        print(f"error: task '{args.id}' not found", file=sys.stderr)
        return 1
    print(json.dumps(task, ensure_ascii=False, indent=2))
    return 0


def cmd_task_update(args: argparse.Namespace) -> int:
    task = kv_get(task_key(args.project, args.id))
    if task is None:
        print(f"error: task '{args.id}' not found", file=sys.stderr)
        return 1
    if args.status:
        task["status"] = args.status
    if args.result_summary is not None:
        task["result_summary"] = args.result_summary
    if args.failure_reason is not None:
        task["failure_reason"] = args.failure_reason
    if args.schedule_id:
        task["schedule_id"] = args.schedule_id
    if args.retry_count is not None:
        task["retry_count"] = args.retry_count
    task["updated_at"] = now()
    if kv_set(task_key(args.project, args.id), task):
        print(json.dumps(task, ensure_ascii=False, indent=2))
        return 0
    return 1


def cmd_task_today(args: argparse.Namespace) -> int:
    items = kv_list(f"{KV_PREFIX}:project:{args.project}:task:", with_values=True)
    d = args.date or today()
    tasks = []
    for item in items:
        t = item["value"]
        if t.get("due_date") == d or t.get("cron"):
            tasks.append(t)
    print(json.dumps(tasks, ensure_ascii=False, indent=2))
    return 0


def cmd_task_next(args: argparse.Namespace) -> int:
    """List the next executable tasks (dependencies completed and status pending)."""
    items = kv_list(f"{KV_PREFIX}:project:{args.project}:task:", with_values=True)
    tasks = {item["value"]["id"]: item["value"] for item in items}
    ready = []
    for t in tasks.values():
        if t.get("status") != "pending":
            continue
        deps = t.get("depends_on", [])
        if all(tasks.get(dep, {}).get("status") == "completed" for dep in deps):
            ready.append(t)
    print(json.dumps(ready, ensure_ascii=False, indent=2))
    return 0


# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------

def cmd_report_daily(args: argparse.Namespace) -> int:
    d = args.date or today()
    items = kv_list(f"{KV_PREFIX}:project:{args.project}:task:", with_values=True)
    completed = []
    failed = []
    pending = []
    for item in items:
        t = item["value"]
        if t.get("status") == "completed" and t.get("updated_at", "").startswith(d):
            completed.append(t)
        elif t.get("status") == "failed":
            failed.append(t)
        elif t.get("status") in ("pending", "in_progress"):
            pending.append(t)

    report = {
        "project_id": args.project,
        "date": d,
        "generated_at": now(),
        "summary": {
            "completed": len(completed),
            "failed": len(failed),
            "pending": len(pending),
        },
        "completed_tasks": completed,
        "failed_tasks": failed,
        "pending_tasks": pending,
    }
    if kv_set(log_key(args.project, d), report):
        print(json.dumps(report, ensure_ascii=False, indent=2))
        return 0
    return 1


def cmd_report_progress(args: argparse.Namespace) -> int:
    meta = kv_get(project_meta_key(args.project))
    if meta is None:
        print(f"error: project '{args.project}' not found", file=sys.stderr)
        return 1

    goal_items = kv_list(f"{KV_PREFIX}:project:{args.project}:goal:", with_values=True)
    goals = [item["value"] for item in goal_items]
    task_items = kv_list(f"{KV_PREFIX}:project:{args.project}:task:", with_values=True)
    tasks = [item["value"] for item in task_items]

    total_tasks = len(tasks)
    completed_tasks = sum(1 for t in tasks if t.get("status") == "completed")
    total_goals = len(goals)
    completed_goals = sum(1 for g in goals if g.get("status") == "completed")

    report = {
        "project_id": args.project,
        "generated_at": now(),
        "project": meta,
        "progress": {
            "tasks": {"total": total_tasks, "completed": completed_tasks},
            "goals": {"total": total_goals, "completed": completed_goals},
        },
        "goals": goals,
        "tasks": tasks,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="project-tool.py", description="project-tracker data management tool")
    sub = parser.add_subparsers(dest="cmd", required=True)

    # project
    p_proj = sub.add_parser("project", help="Project management")
    sp_proj = p_proj.add_subparsers(dest="subcmd", required=True)

    c = sp_proj.add_parser("create", help="Create a project")
    c.add_argument("--name", required=True)
    c.add_argument("--description")
    c.add_argument("--start-date")
    c.add_argument("--end-date")
    c.add_argument("--acceptance-criteria")
    c.add_argument("--recurring", action="store_true")
    c.add_argument("--id")
    c.set_defaults(func=cmd_project_create)

    l = sp_proj.add_parser("list", help="List projects")
    l.set_defaults(func=cmd_project_list)

    g = sp_proj.add_parser("get", help="View a project")
    g.add_argument("--id", required=True)
    g.set_defaults(func=cmd_project_get)

    u = sp_proj.add_parser("update", help="Update a project field")
    u.add_argument("--id", required=True)
    u.add_argument("--field", required=True)
    u.add_argument("--value", required=True)
    u.set_defaults(func=cmd_project_update)

    d = sp_proj.add_parser("delete", help="Delete a project and all related data")
    d.add_argument("--id", required=True)
    d.set_defaults(func=cmd_project_delete)

    # goal
    p_goal = sub.add_parser("goal", help="Goal management")
    sp_goal = p_goal.add_subparsers(dest="subcmd", required=True)

    a = sp_goal.add_parser("add", help="Add a goal")
    a.add_argument("--project", required=True)
    a.add_argument("--title", required=True)
    a.add_argument("--acceptance-criteria")
    a.add_argument("--id")
    a.set_defaults(func=cmd_goal_add)

    l = sp_goal.add_parser("list", help="List goals")
    l.add_argument("--project", required=True)
    l.set_defaults(func=cmd_goal_list)

    u = sp_goal.add_parser("update", help="Update a goal")
    u.add_argument("--project", required=True)
    u.add_argument("--id", required=True)
    u.add_argument("--title")
    u.add_argument("--acceptance-criteria")
    u.add_argument("--status")
    u.set_defaults(func=cmd_goal_update)

    # task
    p_task = sub.add_parser("task", help="Task management")
    sp_task = p_task.add_subparsers(dest="subcmd", required=True)

    a = sp_task.add_parser("add", help="Add a task")
    a.add_argument("--project", required=True)
    a.add_argument("--description", required=True)
    a.add_argument("--agent")
    a.add_argument("--cron")
    a.add_argument("--due-date")
    a.add_argument("--check-criteria")
    a.add_argument("--depends-on")
    a.add_argument("--schedule-id")
    a.add_argument("--max-retries", type=int, default=5)
    a.add_argument("--id")
    a.set_defaults(func=cmd_task_add)

    l = sp_task.add_parser("list", help="List tasks")
    l.add_argument("--project", required=True)
    l.add_argument("--status")
    l.add_argument("--due-date")
    l.set_defaults(func=cmd_task_list)

    g = sp_task.add_parser("get", help="View a task")
    g.add_argument("--project", required=True)
    g.add_argument("--id", required=True)
    g.set_defaults(func=cmd_task_get)

    u = sp_task.add_parser("update", help="Update a task")
    u.add_argument("--project", required=True)
    u.add_argument("--id", required=True)
    u.add_argument("--status", choices=["pending", "in_progress", "completed", "failed"])
    u.add_argument("--result-summary")
    u.add_argument("--failure-reason")
    u.add_argument("--schedule-id")
    u.add_argument("--retry-count", type=int)
    u.set_defaults(func=cmd_task_update)

    t = sp_task.add_parser("today", help="List tasks for today")
    t.add_argument("--project", required=True)
    t.add_argument("--date")
    t.set_defaults(func=cmd_task_today)

    n = sp_task.add_parser("next", help="List the next executable tasks")
    n.add_argument("--project", required=True)
    n.set_defaults(func=cmd_task_next)

    # report
    p_report = sub.add_parser("report", help="Reports")
    sp_report = p_report.add_subparsers(dest="subcmd", required=True)

    d = sp_report.add_parser("daily", help="Generate a daily report")
    d.add_argument("--project", required=True)
    d.add_argument("--date")
    d.set_defaults(func=cmd_report_daily)

    pr = sp_report.add_parser("progress", help="Generate an overall progress report")
    pr.add_argument("--project", required=True)
    pr.set_defaults(func=cmd_report_progress)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
