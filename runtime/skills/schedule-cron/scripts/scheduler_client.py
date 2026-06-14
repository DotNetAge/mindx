#!/usr/bin/env python3
"""
MindX Scheduler Client

Manages recurring tasks via schedule.add/list/del.

Usage:
    python scheduler_client.py add-job --agent writer --content "..." --cron "0 0 9 * * 1"
    python scheduler_client.py list-jobs
    python scheduler_client.py del-job --id a1b2c3d4
"""

import argparse
import json
import sys
import uuid
from dataclasses import dataclass, field, asdict
from typing import Optional

try:
    import websocket
except ImportError:
    print("Error: websocket-client library is required", file=sys.stderr)
    print("   Install with: pip install websocket-client", file=sys.stderr)
    sys.exit(1)


DEFAULT_GATEWAY_HOST = "localhost"
DEFAULT_GATEWAY_PORT = 1314
DEFAULT_GATEWAY_PATH = "/ws"
DEFAULT_TIMEOUT = 30


@dataclass
class JobAddParams:
    agent: str
    content: str
    cron_expr: str
    session_id: str = ""
    project_dir: str = ""


class MindXSchedulerClient:
    """Client for MindX Scheduler"""

    def __init__(self, host=DEFAULT_GATEWAY_HOST, port=DEFAULT_GATEWAY_PORT,
                 path=DEFAULT_GATEWAY_PATH, timeout=DEFAULT_TIMEOUT):
        self.host = host
        self.port = port
        self.path = path
        self.timeout = timeout
        self.ws: Optional[websocket.WebSocket] = None
        self._connected = False

    def _ws_url(self):
        return f"ws://{self.host}:{self.port}{self.path}"

    def connect(self):
        if self._connected:
            return True
        try:
            self.ws = websocket.create_connection(
                self._ws_url(), timeout=self.timeout,
                header={"Origin": f"ws://{self.host}:{self.port}"})
            self._connected = True
            return True
        except Exception as e:
            print(f"Connection failed: {e}", file=sys.stderr)
            return False

    def disconnect(self):
        if self.ws and self._connected:
            try:
                self.ws.close()
            except Exception:
                pass
            self._connected = False

    def _call(self, method: str, params: dict = None) -> dict:
        if not self.connect():
            return {"success": False, "error": "Could not connect to Gateway"}
        try:
            req = {"jsonrpc": "2.0", "id": str(uuid.uuid4()), "method": method}
            if params:
                req["params"] = params
            self.ws.send(json.dumps(req))
            resp = json.loads(self.ws.recv())
            if resp.get("error"):
                return {"success": False, "error": resp["error"].get("message", str(resp["error"])), "raw": resp}
            return {"success": True, "data": resp.get("result"), "raw": resp}
        except Exception as e:
            return {"success": False, "error": str(e)}

    def add_job(self, params: JobAddParams) -> dict:
        agent = params.agent
        session_id = params.session_id or str(uuid.uuid4())
        r = self._call("schedule.add", {
            "agent": agent,
            "content": params.content,
            "cron_expr": params.cron_expr,
            "session_id": session_id,
            "project_dir": params.project_dir,
        })
        if r["success"]:
            r["task_id"] = r["data"].get("id") if isinstance(r["data"], dict) else None
            r["session_id"] = session_id
        return r

    def list_jobs(self) -> dict:
        return self._call("schedule.list")

    def delete_job(self, task_id: str) -> dict:
        return self._call("schedule.del", {"id": task_id})

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, *args):
        self.disconnect()


def cmd_add_job(args):
    p = argparse.ArgumentParser(description="Add a scheduled task")
    p.add_argument("--agent", required=True)
    p.add_argument("--content", required=True)
    p.add_argument("--cron", required=True)
    p.add_argument("--session-id", default="")
    p.add_argument("--project-dir", default="")
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)
    opts = p.parse_args(args)
    params = JobAddParams(opts.agent, opts.content, opts.cron, opts.session_id, opts.project_dir)
    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        r = client.add_job(params)
        if r["success"]:
            print(json.dumps(r["data"], indent=2, ensure_ascii=False))
            return 0
        print(f"Error: {r['error']}", file=sys.stderr)
        return 1


def cmd_list_jobs(args):
    p = argparse.ArgumentParser(description="List scheduled tasks")
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)
    p.add_argument("--json", action="store_true")
    opts = p.parse_args(args)
    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        r = client.list_jobs()
        if r["success"]:
            if opts.json:
                print(json.dumps(r["data"], indent=2, ensure_ascii=False))
            else:
                items = r["data"] if isinstance(r["data"], list) else []
                if not items:
                    print("No scheduled tasks.")
                    return 0
                for item in items:
                    print(f"  {item.get('id','?'):10s}  agent={item.get('agent','')}  cron={item.get('cron_expr','')}  "
                          f"enabled={item.get('enabled',False)}  ok={item.get('success_count',0)}/{item.get('failure_count',0)}")
            return 0
        print(f"Error: {r['error']}", file=sys.stderr)
        return 1


def cmd_del_job(args):
    p = argparse.ArgumentParser(description="Delete a scheduled task")
    p.add_argument("--id", required=True)
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)
    opts = p.parse_args(args)
    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        r = client.delete_job(opts.id)
        if r["success"]:
            print(f"Deleted: {opts.id}")
            return 0
        print(f"Error: {r['error']}", file=sys.stderr)
        return 1


def cmd_test_conn(args):
    p = argparse.ArgumentParser(description="Test Gateway connection")
    p.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    p.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)
    opts = p.parse_args(args)
    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        if client.connect():
            print(f"OK: Connected to {opts.host}:{opts.port}")
            return 0
        print(f"FAILED: Could not connect to {opts.host}:{opts.port}", file=sys.stderr)
        return 1


def main():
    parser = argparse.ArgumentParser(
        description="MindX Scheduler Client")
    sub = parser.add_subparsers(dest="command")
    for name, help_text, func in [
        ("add-job", "Add a scheduled task", cmd_add_job),
        ("list-jobs", "List scheduled tasks", cmd_list_jobs),
        ("del-job", "Delete a scheduled task", cmd_del_job),
        ("test-conn", "Test Gateway connection", cmd_test_conn),
    ]:
        p = sub.add_parser(name, help=help_text)
        p.set_defaults(func=func)

    args = parser.parse_args()
    if hasattr(args, "func"):
        exit(args.func(sys.argv[2:]))
    parser.print_help()
    exit(1)


if __name__ == "__main__":
    main()
