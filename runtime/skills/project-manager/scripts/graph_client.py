#!/usr/bin/env python3
"""
graph_client.py - MindX Project Manager graph database operations.

CLI commands for managing projects, goals, tasks, sessions, and dependencies.
All data is stored via the MindX Daemon's Graph API.
"""

import argparse
import json
import os
import sys
import uuid
from datetime import datetime, timezone

try:
    import websocket
except ImportError:
    print("Error: websocket-client package is required")
    print("   Install with: pip install websocket-client")
    sys.exit(1)


# ====== Configuration ======

DEFAULT_WS_HOST = os.environ.get("MINDX_WS_HOST", "localhost")
DEFAULT_WS_PORT = int(os.environ.get("MINDX_WS_PORT", "1314"))
DEFAULT_WS_PATH = os.environ.get("MINDX_WS_PATH", "/ws")
RPC_TIMEOUT = 30


# ====== RPC Client ======

def rpc_call(method, params=None):
    """Send a JSON-RPC 2.0 request to the Daemon via WebSocket."""
    url = f"ws://{DEFAULT_WS_HOST}:{DEFAULT_WS_PORT}{DEFAULT_WS_PATH}"
    try:
        ws = websocket.create_connection(url, timeout=RPC_TIMEOUT)
    except Exception as e:
        print(f'{{"error": "daemon connection failed: {e}"}}', file=sys.stderr)
        sys.exit(1)

    request_id = str(uuid.uuid4())
    request = {"jsonrpc": "2.0", "id": request_id, "method": method}
    if params is not None:
        request["params"] = params

    try:
        ws.send(json.dumps(request))
        response_str = ws.recv()
        ws.close()
    except Exception as e:
        ws.close()
        print(f'{{"error": "rpc call failed: {e}"}}', file=sys.stderr)
        sys.exit(1)

    response = json.loads(response_str)
    if response.get("error"):
        err = response["error"]
        msg = err.get("message", str(err))
        print(f'{{"error": "rpc error [{method}]: {msg}"}}', file=sys.stderr)
        sys.exit(1)

    result = response.get("result")
    return result


# ====== Graph Operations (via Daemon) ======

def graph_query(cypher, params_map=None):
    """Execute a read Cypher query via daemon graph.query"""
    rpc_params = {"query": cypher}
    if params_map:
        rpc_params["params"] = params_map
    result = rpc_call("graph.query", rpc_params)

    # graph.query returns {columns: [...], rows: [{col: val, ...}, ...]}
    return (result or {}).get("rows", [])


def graph_exec(cypher, params_map=None):
    """Execute a write Cypher query via daemon graph.exec"""
    rpc_params = {"query": cypher}
    if params_map:
        rpc_params["params"] = params_map
    result = rpc_call("graph.exec", rpc_params)
    return result


def upsert_nodes(nodes):
    """Batch upsert nodes via daemon graph.upsert_nodes"""
    return rpc_call("graph.upsert_nodes", {"nodes": nodes})


def upsert_edges(edges):
    """Batch upsert edges via daemon graph.upsert_edges"""
    return rpc_call("graph.upsert_edges", {"edges": edges})


# ====== Helpers ======

def generate_id(prefix):
    return f"{prefix}-{uuid.uuid4().hex[:8]}"


def timestamp():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


# ====== Project Management Commands ======

def cmd_create_project(args):
    proj_id = generate_id("proj")
    now = timestamp()
    metrics_json = json.loads(args.metrics) if args.metrics else {}
    timeline_json = json.loads(args.timeline) if args.timeline else {}

    nodes = [{
        "id": proj_id,
        "labels": ["Project"],
        "properties": {
            "name": args.name,
            "description": args.description or args.name,
            "status": "active",
            "progress": 0.0,
            "created_at": now,
            "updated_at": now,
            "metrics": json.dumps(metrics_json, ensure_ascii=False),
            "timeline": json.dumps(timeline_json, ensure_ascii=False),
        }
    }]
    upsert_nodes(nodes)

    print(f"Project created:")
    print(f"   ID: {proj_id}")
    print(f"   Name: {args.name}")
    print(f"   Status: active")


def cmd_query_project(args):
    cypher = """
        MATCH (p:Project {id: $project_id})
        OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN p,
               collect(DISTINCT g) as goals,
               collect(DISTINCT t) as tasks
    """
    result = graph_query(cypher, {"project_id": args.project_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_list_projects(args):
    cypher = """
        MATCH (p:Project)
        OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)
        RETURN p.id as id,
               p.name as name,
               p.status as status,
               p.progress as progress,
               count(g) as goal_count
        ORDER BY p.updated_at DESC
    """
    result = graph_query(cypher)
    print(json.dumps(result, indent=2, default=str))


def cmd_update_project(args):
    now = timestamp()
    set_parts = [f"p.updated_at = '{now}'"]
    if args.status:
        set_parts.append(f"p.status = '{args.status}'")
    if args.progress is not None:
        set_parts.append(f"p.progress = {args.progress}")

    set_clause = ", ".join(set_parts)
    cypher = f"""
        MATCH (p:Project {{id: $project_id}})
        SET {set_clause}
        RETURN p.id, p.name, p.status, p.progress
    """
    result = graph_query(cypher, {"project_id": args.project_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_create_goal(args):
    goal_id = generate_id("goal")
    now = timestamp()
    weight = float(args.weight) if args.weight else 0.0
    metrics_json = json.loads(args.metrics) if args.metrics else {}

    nodes = [{
        "id": goal_id,
        "labels": ["Goal"],
        "properties": {
            "title": args.title,
            "description": args.description or args.title,
            "weight": weight,
            "status": "pending",
            "progress": 0.0,
            "created_at": now,
            "updated_at": now,
            "metrics": json.dumps(metrics_json, ensure_ascii=False),
        }
    }]
    upsert_nodes(nodes)

    edges = [{
        "from_node_id": args.project_id,
        "to_node_id": goal_id,
        "type": "HAS_GOAL",
        "properties": {"order": now},
    }]
    upsert_edges(edges)


def cmd_query_goals(args):
    cypher = """
        MATCH (p:Project {id: $project_id})-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN g.id, g.title, g.status, g.progress, g.weight,
               count(t) as task_count,
               count(CASE WHEN t.status = 'completed' THEN 1 END) as completed_count
        ORDER BY g.created_at
    """
    result = graph_query(cypher, {"project_id": args.project_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_update_goal(args):
    now = timestamp()
    set_parts = [f"g.updated_at = '{now}'"]
    if args.status:
        set_parts.append(f"g.status = '{args.status}'")
        if args.status == "completed":
            set_parts.append("g.progress = 1.0")
    if args.progress is not None:
        set_parts.append(f"g.progress = {args.progress}")

    set_clause = ", ".join(set_parts)
    cypher = f"""
        MATCH (g:Goal {{id: $goal_id}})
        SET {set_clause}
        RETURN g.id, g.title, g.status, g.progress
    """
    result = graph_query(cypher, {"goal_id": args.goal_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_create_task(args):
    task_id = generate_id("task")
    now = timestamp()

    nodes = [{
        "id": task_id,
        "labels": ["Task"],
        "properties": {
            "title": args.title,
            "agent": args.agent or "assistant",
            "cron_expr": args.cron_expr or "",
            "prompt": args.prompt or args.title,
            "status": "pending",
            "priority": args.priority or "normal",
            "progress": 0.0,
            "success_count": 0,
            "failure_count": 0,
            "created_at": now,
            "updated_at": now,
        }
    }]
    upsert_nodes(nodes)

    edges = [{
        "from_node_id": args.goal_id,
        "to_node_id": task_id,
        "type": "CONTAINS",
        "properties": {"order": now},
    }]
    upsert_edges(edges)


def cmd_update_task(args):
    now = timestamp()
    set_parts = ["t.updated_at = $now"]

    params = {"task_id": args.task_id, "now": now}

    if args.status:
        set_parts.append("t.status = $status")
        params["status"] = args.status
        if args.status == "completed":
            set_parts.append("t.success_count = coalesce(t.success_count, 0) + 1")
            set_parts.append("t.progress = 1.0")
        elif args.status == "failed":
            set_parts.append("t.failure_count = coalesce(t.failure_count, 0) + 1")
    if args.result:
        set_parts.append("t.summary = $result")
        params["result"] = args.result
    if args.scheduler_id:
        set_parts.append("t.scheduler_id = $scheduler_id")
        params["scheduler_id"] = args.scheduler_id
    if args.progress is not None:
        set_parts.append("t.progress = $progress")
        params["progress"] = args.progress
    if args.session_id:
        set_parts.append("t.session_id = $session_id")
        params["session_id"] = args.session_id
    if args.interruption_type:
        set_parts.append("t.interruption_type = $interruption_type")
        params["interruption_type"] = args.interruption_type
    if args.interruption_context:
        ctx = json.loads(args.interruption_context) if args.interruption_context.startswith('{') else args.interruption_context
        set_parts.append("t.interruption_context = $interruption_context")
        params["interruption_context"] = ctx
    if args.verification_note:
        set_parts.append("t.verification_note = $verification_note")
        params["verification_note"] = args.verification_note

    set_clause = ", ".join(set_parts)
    cypher = f"""
        MATCH (t:Task {{id: $task_id}})
        SET {set_clause}
        RETURN t.id, t.title, t.status, t.progress, t.success_count, t.failure_count, t.session_id
    """
    result = graph_query(cypher, params)
    print(json.dumps(result, indent=2, default=str))


def cmd_record_execution(args):
    exec_id = generate_id("exec")
    now = timestamp()
    duration = int(args.duration) if args.duration else 0

    nodes = [{
        "id": exec_id,
        "labels": ["Execution"],
        "properties": {
            "status": args.status,
            "result": args.result or "",
            "duration_seconds": duration,
            "executed_at": now,
        }
    }]
    upsert_nodes(nodes)

    edges = [{
        "from_node_id": args.task_id,
        "to_node_id": exec_id,
        "type": "HAS_EXECUTION",
        "properties": {},
    }]
    upsert_edges(edges)

    # Also update the task status
    class UpdateArgs:
        task_id = args.task_id
        status = args.status
        result = args.result
        scheduler_id = getattr(args, 'scheduler_id', '')
        progress = getattr(args, 'progress', None)
        session_id = getattr(args, 'session_id', '')
        interruption_type = getattr(args, 'interruption_type', '')
        interruption_context = getattr(args, 'interruption_context', '')
        verification_note = getattr(args, 'verification_note', '')

    cmd_update_task(UpdateArgs())


def cmd_query_tasks(args):
    where_parts = ["TRUE"]
    params = {}

    if args.goal_id:
        where_parts.append("g.id = $goal_id")
        params["goal_id"] = args.goal_id

    if args.status:
        statuses = [s.strip() for s in args.status.split(",")]
        conditions = []
        for i, s in enumerate(statuses):
            key = f"status_{i}"
            if s == "awaiting_*":
                conditions.append(f"t.status STARTS WITH 'awaiting_'")
            else:
                conditions.append(f"t.status = ${key}")
                params[key] = s
        status_filter = f"AND ({' OR '.join(conditions)})"
    else:
        status_filter = ""

    where_clause = " AND ".join(where_parts)

    cypher = f"""
        MATCH (g:Goal)-[:CONTAINS]->(t:Task)
        WHERE {where_clause} {status_filter}
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        WITH t, g, collect(pre.id) as depends_on
        RETURN t.id, t.title, t.agent, t.cron_expr, t.status,
               t.priority, t.progress, t.success_count, t.failure_count,
               t.summary, t.scheduler_id, t.session_id,
               t.interruption_type, t.interruption_context, t.verification_note,
               depends_on, g.title as goal_title,
               t.created_at, t.updated_at
        ORDER BY t.updated_at DESC
    """
    result = graph_query(cypher, params)
    print(json.dumps(result, indent=2, default=str))


def cmd_query_by_status(args):
    cypher = """
        MATCH (t:Task {status: $status})
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        RETURN t.id, t.title, t.agent, t.status, t.progress,
               t.success_count, t.failure_count, t.updated_at,
               g.title as goal_title, p.name as project_name
        ORDER BY t.updated_at DESC
        LIMIT 50
    """
    result = graph_query(cypher, {"status": args.status})
    print(json.dumps(result, indent=2, default=str))


def cmd_get_task(args):
    cypher = """
        MATCH (t:Task {id: $task_id})
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
        WITH t, g, pre,
             collect(DISTINCT pre.id) as dependencies,
             collect(e) as executions
        RETURN t, g.title as goal_title, dependencies, executions
    """
    result = graph_query(cypher, {"task_id": args.task_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_get_task_output(args):
    cypher = """
        MATCH (t:Task {id: $task_id})
        OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        WITH t, e, pre
        ORDER BY e.executed_at DESC
        WITH t,
             collect(DISTINCT pre.id) as depends_on,
             collect(e)[0] as latest_execution
        RETURN t.id, t.title, t.agent, t.status, t.summary,
               t.session_id, t.success_count, t.failure_count,
               t.verification_note, t.interruption_type,
               depends_on,
               latest_execution.id as exec_id,
               latest_execution.status as exec_status,
               latest_execution.result as exec_result,
               latest_execution.executed_at as exec_time,
               latest_execution.duration_seconds as exec_duration
    """
    result = graph_query(cypher, {"task_id": args.task_id})
    print(json.dumps(result, indent=2, default=str))


# ====== Session Management Commands ======

def cmd_register_session(args):
    sess_id = args.session_id.strip() if args.session_id and args.session_id.strip() else generate_id("sess")
    now = timestamp()

    session_nodes = [{
        "id": sess_id,
        "labels": ["Session"],
        "properties": {
            "task_id": args.task_id,
            "agent": args.agent,
            "status": args.session_status or "initialized",
            "created_by": args.created_by or "system",
            "interruption_type": None,
            "interruption_context": None,
            "resolution": None,
            "resolved_at": None,
            "replacement_session_id": None,
            "loss_reason": None,
            "timeout_reason": None,
            "created_at": now,
            "updated_at": now,
        }
    }]
    upsert_nodes(session_nodes)

    edges = [{
        "from_node_id": args.task_id,
        "to_node_id": sess_id,
        "type": "HAS_SESSION",
        "properties": {},
    }]
    upsert_edges(edges)

    # Update task's session_id
    cypher = """
        MATCH (t:Task {id: $task_id})
        SET t.session_id = $sess_id, t.updated_at = $now
        RETURN t.id as task_id, t.title as task_title, t.session_id as session_id
    """
    result = graph_query(cypher, {"task_id": args.task_id, "sess_id": sess_id, "now": now})
    print(json.dumps(result, indent=2, default=str))


def cmd_get_session(args):
    cypher = """
        MATCH (s:Session {id: $session_id})
        OPTIONAL MATCH (t:Task)-[:HAS_SESSION]->(s)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        OPTIONAL MATCH (s)-[:HAS_EXECUTION]->(e:Execution)
        WITH s, t, g, p,
             collect(e) as executions
        RETURN s,
               t.id as task_id, t.title as task_title, t.agent as task_agent, t.status as task_status,
               g.title as goal_title, p.name as project_name,
               executions
    """
    result = graph_query(cypher, {"session_id": args.session_id})
    print(json.dumps(result, indent=2, default=str))


def cmd_update_session(args):
    now = timestamp()
    set_parts = ["s.updated_at = $now"]
    params = {"session_id": args.session_id, "now": now}

    if args.status:
        set_parts.append("s.status = $status")
        params["status"] = args.status
    if args.resolution:
        set_parts.append("s.resolution = $resolution")
        params["resolution"] = args.resolution
    if args.resolved_at:
        set_parts.append("s.resolved_at = $resolved_at")
        params["resolved_at"] = args.resolved_at
    if args.replacement_session_id:
        set_parts.append("s.replacement_session_id = $replacement_session_id")
        params["replacement_session_id"] = args.replacement_session_id
    if args.loss_reason:
        set_parts.append("s.loss_reason = $loss_reason")
        params["loss_reason"] = args.loss_reason
    if args.timeout_reason:
        set_parts.append("s.timeout_reason = $timeout_reason")
        params["timeout_reason"] = args.timeout_reason
    if args.interruption_context:
        ctx = json.loads(args.interruption_context) if args.interruption_context.startswith('{') else args.interruption_context
        set_parts.append("s.interruption_context = $interruption_context")
        params["interruption_context"] = ctx

    set_clause = ", ".join(set_parts)
    cypher = f"""
        MATCH (s:Session {{id: $session_id}})
        SET {set_clause}
        RETURN s.id, s.status, s.resolution, s.task_id
    """
    result = graph_query(cypher, params)
    print(json.dumps(result, indent=2, default=str))


def cmd_query_sessions(args):
    where_parts = []
    params = {}

    if args.status:
        if args.status == "awaiting_*":
            where_parts.append("s.status STARTS WITH 'awaiting_'")
        else:
            statuses = [s.strip() for s in args.status.split(",")]
            for i, s in enumerate(statuses):
                key = f"status_{i}"
                if s == "awaiting_*":
                    where_parts.append("s.status STARTS WITH 'awaiting_'")
                else:
                    where_parts.append(f"s.status = ${key}")
                    params[key] = s

    if args.stale_threshold:
        threshold = args.stale_threshold
        if threshold.endswith('h'):
            seconds = int(threshold[:-1]) * 3600
        elif threshold.endswith('d'):
            seconds = int(threshold[:-1]) * 86400
        else:
            seconds = int(threshold)
        where_parts.append(f"s.updated_at < datetime() - duration('P{seconds}S')")

    where_clause = ""
    if where_parts:
        where_clause = f"WHERE {' AND '.join(where_parts)}"

    cypher = f"""
        MATCH (s:Session) {where_clause}
        OPTIONAL MATCH (t:Task)-[:HAS_SESSION]->(s)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        RETURN s.id, s.status, s.interruption_type, s.resolution,
               s.created_at, s.updated_at, s.loss_reason, s.timeout_reason,
               t.id as task_id, t.title as task_title, t.agent as task_agent,
               g.title as goal_title
        ORDER BY s.updated_at ASC
        LIMIT 50
    """
    result = graph_query(cypher, params)
    print(json.dumps(result, indent=2, default=str))


# ====== Relationship Management Commands ======

def cmd_add_dependency(args):
    edges = [{
        "from_node_id": args.task_id,
        "to_node_id": args.depends_on,
        "type": "DEPENDS_ON",
        "properties": {},
    }]
    upsert_edges(edges)
    print(json.dumps({"task": args.task_id, "depends_on": args.depends_on}, indent=2))


def cmd_remove_dependency(args):
    cypher = """
        MATCH (t:Task {id: $task_id})-[dep:DEPENDS_ON]->(pre:Task {id: $depends_on})
        DELETE dep
        RETURN 'dependency removed' as result
    """
    result = graph_query(cypher, {"task_id": args.task_id, "depends_on": args.depends_on})
    print(json.dumps(result, indent=2, default=str))


# ====== Goal Query ======

def cmd_get_goal(args):
    cypher = """
        MATCH (g:Goal {id: $goal_id})
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN g, p.name as project_name,
               count(t) as total_tasks,
               count(CASE WHEN t.status = 'completed' THEN 1 END) as completed_tasks
    """
    result = graph_query(cypher, {"goal_id": args.goal_id})
    print(json.dumps(result, indent=2, default=str))


# ====== Progress Report ======

def cmd_progress_report(args):
    cypher = """
        MATCH (p:Project {id: $project_id})-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        WITH p, g,
             count(t) as total_tasks,
             count(CASE WHEN t.status = 'completed' THEN 1 END) as completed,
             count(CASE WHEN t.status = 'in_progress' THEN 1 END) as in_progress,
             count(CASE WHEN t.status = 'pending' THEN 1 END) as pending,
             count(CASE WHEN t.status = 'failed' THEN 1 END) as failed,
             sum(t.success_count) as total_success,
             sum(t.failure_count) as total_failures
        RETURN p.id, p.name, p.status, p.progress,
               collect({
                 goal_id: g.id,
                 goal_title: g.title,
                 goal_weight: g.weight,
                 goal_status: g.status,
                 goal_progress: g.progress,
                 tasks: total_tasks,
                 completed: completed,
                 in_progress: in_progress,
                 pending: pending,
                 failed: failed,
                 successes: total_success,
                 failures: total_failures
               }) as goals_data
    """
    result = graph_query(cypher, {"project_id": args.project_id})
    print(json.dumps(result, indent=2, default=str))


# ====== Argument Parser ======

def build_parser():
    parser = argparse.ArgumentParser(
        description="MindX Project Manager - Graph Database Operations",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s create-project --name "Community Ops" --description "Increase engagement"
  %(prog)s create-goal --project-id proj001 --title "Content Creation" --weight 0.4
  %(prog)s create-task --goal-id goal001 --agent writer --cron-expr "0 0 9 * * 1" --prompt "Write article"
  %(prog)s update-task --task-id task001 --status completed --scheduler-id sched123 --session-id sess456
  %(prog)s register-session --task-id task001 --agent writer --session-id 550e8400-e29b-41d4-a716-446655440000 --session-status initialized
  %(prog)s progress-report --project-id proj001
        """)

    subparsers = parser.add_subparsers(dest='command', help='Available commands')

    # Project commands
    p_proj = subparsers.add_parser('create-project', help='Create a project node')
    p_proj.add_argument('--name', required=True)
    p_proj.add_argument('--description', default='')
    p_proj.add_argument('--metrics', default='{}')
    p_proj.add_argument('--timeline', default='{}')

    p_qproj = subparsers.add_parser('query-project', help='Query project details')
    p_qproj.add_argument('--project-id', required=True)

    p_lproj = subparsers.add_parser('list-projects', help='List all projects')

    p_uproj = subparsers.add_parser('update-project', help='Update project properties')
    p_uproj.add_argument('--project-id', required=True)
    p_uproj.add_argument('--status', choices=['active', 'completed', 'paused', 'archived'])
    p_uproj.add_argument('--progress', type=float)

    # Goal commands
    p_cgoal = subparsers.add_parser('create-goal', help='Create a goal node under a project')
    p_cgoal.add_argument('--project-id', required=True)
    p_cgoal.add_argument('--title', required=True)
    p_cgoal.add_argument('--description', default='')
    p_cgoal.add_argument('--weight', type=float, default=0.0)
    p_cgoal.add_argument('--metrics', default='{}')

    p_qgoal = subparsers.add_parser('query-goals', help='Query goals for a project')
    p_qgoal.add_argument('--project-id', required=True)

    p_ugoal = subparsers.add_parser('update-goal', help='Update goal progress/status')
    p_ugoal.add_argument('--goal-id', required=True)
    p_ugoal.add_argument('--status', choices=['pending', 'in_progress', 'completed', 'paused'])
    p_ugoal.add_argument('--progress', type=float)

    # Task commands
    p_ctask = subparsers.add_parser('create-task', help='Create a task node under a goal')
    p_ctask.add_argument('--goal-id', required=True)
    p_ctask.add_argument('--title', required=True)
    p_ctask.add_argument('--agent', default='')
    p_ctask.add_argument('--cron-expr', default='')
    p_ctask.add_argument('--prompt', default='')
    p_ctask.add_argument('--priority', default='normal')

    p_utask = subparsers.add_parser('update-task', help='Update task status/results')
    p_utask.add_argument('--task-id', required=True)
    p_utask.add_argument('--status', choices=['pending', 'scheduled', 'in_progress', 'completed', 'failed',
                                                    'awaiting_authorization', 'awaiting_clarification', 'awaiting_resource'])
    p_utask.add_argument('--result', default='')
    p_utask.add_argument('--scheduler-id', default='')
    p_utask.add_argument('--progress', type=float)
    p_utask.add_argument('--session-id', default='')
    p_utask.add_argument('--interruption-type', default='')
    p_utask.add_argument('--interruption-context', default='')
    p_utask.add_argument('--verification-note', default='')

    p_rexec = subparsers.add_parser('record-execution', help='Record a task execution result')
    p_rexec.add_argument('--task-id', required=True)
    p_rexec.add_argument('--status', required=True)
    p_rexec.add_argument('--result', default='')
    p_rexec.add_argument('--duration', type=int, default=0)

    p_qtask = subparsers.add_parser('query-tasks', help='Query tasks by goal or status filter')
    p_qtask.add_argument('--goal-id', default='')
    p_qtask.add_argument('--status', default='')

    p_qbs = subparsers.add_parser('query-by-status', help='Query tasks by single status')
    p_qbs.add_argument('--status', required=True)

    p_gtask = subparsers.add_parser('get-task', help='Get a single task with full details')
    p_gtask.add_argument('--task-id', required=True)

    p_gtout = subparsers.add_parser('get-task-output', help='Get task output for quality verification')
    p_gtout.add_argument('--task-id', required=True)

    # Session commands
    p_rsess = subparsers.add_parser('register-session', help='Register a new session for a task')
    p_rsess.add_argument('--task-id', required=True)
    p_rsess.add_argument('--agent', required=True)
    p_rsess.add_argument('--session-id', default='',
                           help='Client-generated UUID v4 (omit to auto-generate)')
    p_rsess.add_argument('--session-status', default='initialized')
    p_rsess.add_argument('--created-by', default='')

    p_gsess = subparsers.add_parser('get-session', help='Get session details including context')
    p_gsess.add_argument('--session-id', required=True)

    p_usess = subparsers.add_parser('update-session', help='Update session state after recovery')
    p_usess.add_argument('--session-id', required=True)
    p_usess.add_argument('--status', default='')
    p_usess.add_argument('--resolution', default='')
    p_usess.add_argument('--resolved-at', default='')
    p_usess.add_argument('--replacement-session-id', default='')
    p_usess.add_argument('--loss-reason', default='')
    p_usess.add_argument('--timeout-reason', default='')
    p_usess.add_argument('--interruption-context', default='')

    p_qsess = subparsers.add_parser('query-sessions', help='Find stale/interrupted sessions')
    p_qsess.add_argument('--status', default='')
    p_qsess.add_argument('--stale-threshold', default='')

    # Relationship commands
    p_adep = subparsers.add_parser('add-dependency', help='Add a task dependency')
    p_adep.add_argument('--task-id', required=True)
    p_adep.add_argument('--depends-on', required=True)

    p_rdep = subparsers.add_parser('remove-dependency', help='Remove a dependency')
    p_rdep.add_argument('--task-id', required=True)
    p_rdep.add_argument('--depends-on', required=True)

    # Query tool commands
    p_ggoal = subparsers.add_parser('get-goal', help='Get a single goal with full details')
    p_ggoal.add_argument('--goal-id', required=True)

    p_prept = subparsers.add_parser('progress-report', help='Generate progress report dataset')
    p_prept.add_argument('--project-id', required=True)

    return parser


# ====== Command Dispatch ======

COMMAND_MAP = {
    'create-project': cmd_create_project,
    'query-project': cmd_query_project,
    'list-projects': cmd_list_projects,
    'update-project': cmd_update_project,
    'create-goal': cmd_create_goal,
    'query-goals': cmd_query_goals,
    'update-goal': cmd_update_goal,
    'create-task': cmd_create_task,
    'update-task': cmd_update_task,
    'record-execution': cmd_record_execution,
    'query-tasks': cmd_query_tasks,
    'query-by-status': cmd_query_by_status,
    'get-task': cmd_get_task,
    'get-task-output': cmd_get_task_output,
    'register-session': cmd_register_session,
    'get-session': cmd_get_session,
    'update-session': cmd_update_session,
    'query-sessions': cmd_query_sessions,
    'add-dependency': cmd_add_dependency,
    'remove-dependency': cmd_remove_dependency,
    'get-goal': cmd_get_goal,
    'progress-report': cmd_progress_report,
}


def main():
    parser = build_parser()
    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(1)

    handler = COMMAND_MAP.get(args.command)
    if not handler:
        print(f"Unknown command: {args.command}")
        sys.exit(1)

    try:
        handler(args)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
