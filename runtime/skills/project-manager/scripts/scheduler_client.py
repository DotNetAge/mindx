#!/usr/bin/env python3
"""
MindX Scheduler WebSocket Client (JSON-RPC 2.0)

Communicates with MindX Daemon via WebSocket using JSON-RPC 2.0 protocol.

JSON-RPC Methods:
    schedule.list   List all scheduled tasks
    schedule.add    Add a new scheduled task
    schedule.del    Delete a scheduled task

Protocol:
    Request:  {"jsonrpc":"2.0","id":"<uuid>","method":"schedule.*","params":{...}}
    Response: {"jsonrpc":"2.0","id":"<uuid>","result":...} or {"error":{"code":...,"message":"..."}}

Usage Examples:
    # Add a scheduled task
    python scheduler_client.py add-job --agent @writer --content "Daily blog post" --cron "0 0 9 * * 1"

    # Add a scheduled task with session and project directory
    python scheduler_client.py add-job --agent @writer --content "Daily blog post" \
        --cron "0 0 9 * * 1" --session-id "550e8400-e29b-41d4-a716-446655440000" \
        --project-dir /Users/ray/workspaces/my-project

    # List all tasks
    python scheduler_client.py list-jobs

    # Delete a task
    python scheduler_client.py del-job --id a1b2c3d4

    # Batch register tasks (from JSON file)
    python scheduler_client.py batch-add --file tasks.json
"""

import argparse
import json
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Optional

try:
    import websocket
except ImportError:
    print("Error: websocket-client library is required")
    print("   Install with: pip install websocket-client")
    sys.exit(1)


# ====== Configuration ======

DEFAULT_GATEWAY_HOST = "localhost"
DEFAULT_GATEWAY_PORT = 1314
DEFAULT_GATEWAY_PATH = "/ws"
DEFAULT_TIMEOUT = 30  # seconds


# ====== Data Models ======

@dataclass
class SchedulerResult:
    """Result of a Scheduler operation"""
    success: bool
    command: str
    data: Optional[any] = None
    error: Optional[str] = None
    raw_response: Optional[str] = None
    task_id: Optional[str] = None
    session_id: Optional[str] = None


@dataclass
class JobAddParams:
    """Parameters for adding a scheduled task"""
    agent: str          # Target agent (e.g., @writer)
    content: str        # Message content to send
    cron_expr: str       # 6-field Cron expression
    session_id: str = ""    # Optional session UUID to resume
    project_dir: str = ""   # Optional project directory


# ====== Core Client Class ======

class MindXSchedulerClient:
    """WebSocket client for MindX Scheduler using JSON-RPC 2.0 protocol"""

    def __init__(
        self,
        host: str = DEFAULT_GATEWAY_HOST,
        port: int = DEFAULT_GATEWAY_PORT,
        path: str = DEFAULT_GATEWAY_PATH,
        timeout: int = DEFAULT_TIMEOUT,
    ):
        self.host = host
        self.port = port
        self.path = path
        self.timeout = timeout
        self.ws: Optional[websocket.WebSocket] = None
        self._connected = False

    def _get_ws_url(self) -> str:
        """Build the WebSocket connection URL"""
        return f"ws://{self.host}:{self.port}{self.path}"

    def connect(self) -> bool:
        """Connect to the Gateway"""
        if self._connected:
            return True

        try:
            url = self._get_ws_url()
            self.ws = websocket.create_connection(
                url,
                timeout=self.timeout,
                header={"Origin": f"ws://{self.host}:{self.port}"}
            )
            self._connected = True
            return True
        except Exception as e:
            print(f"Connection failed: {e}")
            return False

    def disconnect(self):
        """Disconnect from the Gateway"""
        if self.ws and self._connected:
            try:
                self.ws.close()
            except Exception:
                pass
            finally:
                self._connected = False

    def _call(self, method: str, params: dict = None) -> dict:
        """
        Send a JSON-RPC 2.0 request and return the response.

        Request:  {"jsonrpc":"2.0","id":"<uuid>","method":"...","params":{...}}
        Response: {"jsonrpc":"2.0","id":"<uuid>","result":...} or {"error":{...}}
        """
        if not self.connect():
            return {"success": False, "error": "Could not connect to Gateway"}

        try:
            request_id = str(uuid.uuid4())
            request = {
                "jsonrpc": "2.0",
                "id": request_id,
                "method": method,
            }

            if params is not None:
                request["params"] = params

            self.ws.send(json.dumps(request))

            response_str = self.ws.recv()
            response = json.loads(response_str)

            if "error" in response and response["error"] is not None:
                error = response["error"]
                error_msg = error.get("message", str(error))
                return {
                    "success": False,
                    "command": method,
                    "error": error_msg,
                    "raw": response_str,
                    "error_code": error.get("code")
                }

            result_data = response.get("result")

            return {
                "success": True,
                "command": method,
                "data": result_data,
                "raw": response_str
            }

        except json.JSONDecodeError as e:
            return {"success": False, "command": method, "error": f"JSON parse error: {e}"}
        except websocket.WebSocketTimeoutException:
            return {"success": False, "command": method, "error": "Request timed out"}
        except Exception as e:
            return {"success": False, "command": method, "error": str(e)}

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.disconnect()

    # ====== High-Level Methods ======

    def add_job(self, params: JobAddParams) -> SchedulerResult:
        """
        Add a scheduled task via schedule.add JSON-RPC.

        When session_id is empty or "new", the client auto-generates a UUID v4
        as the session identifier. To link this schedule to an existing task,
        pass the graph task ID as session_id.

        Args:
            params: Task parameters (agent, content, cron_expr, session_id, project_dir)

        Returns:
            Operation result containing the created schedule entry
        """
        agent = params.agent[1:] if params.agent.startswith("@") else params.agent

        # Client-managed session ID: generate UUID v4 when "new" or empty
        raw_session_id = params.session_id if params.session_id else "new"
        if raw_session_id == "new":
            resolved_session_id = str(uuid.uuid4())
        else:
            resolved_session_id = raw_session_id

        rpc_params = {
            "agent": agent,
            "content": params.content,
            "cron_expr": params.cron_expr,
            "session_id": resolved_session_id,
        }
        if params.project_dir:
            rpc_params["project_dir"] = params.project_dir

        result_dict = self._call("schedule.add", rpc_params)

        result = SchedulerResult(
            success=result_dict["success"],
            command="schedule.add",
            raw_response=result_dict.get("raw"),
            session_id=resolved_session_id,
        )

        if result.success:
            result.data = result_dict.get("data")
            if isinstance(result.data, dict):
                result.task_id = result.data.get("id")
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def list_jobs(self) -> SchedulerResult:
        """
        List all scheduled tasks via schedule.list JSON-RPC.

        Returns:
            Operation result containing the task list as an array of schedule entries
        """
        result_dict = self._call("schedule.list")

        result = SchedulerResult(
            success=result_dict["success"],
            command="schedule.list",
            raw_response=result_dict.get("raw")
        )

        if result.success:
            result.data = result_dict.get("data", [])
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def delete_job(self, task_id: str) -> SchedulerResult:
        """
        Delete a scheduled task via schedule.del JSON-RPC.

        Args:
            task_id: The task ID to delete

        Returns:
            Operation result
        """
        result_dict = self._call("schedule.del", {"id": task_id})

        result = SchedulerResult(
            success=result_dict["success"],
            command="schedule.del",
            raw_response=result_dict.get("raw")
        )

        if result.success:
            result.data = result_dict.get("data")
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def batch_add_jobs(self, jobs: list[JobAddParams]) -> list[SchedulerResult]:
        """
        Batch add multiple scheduled tasks.

        Args:
            jobs: List of task parameters

        Returns:
            List of results for each task operation
        """
        results = []

        for i, job_params in enumerate(jobs, 1):
            print(f"\n[{i}/{len(jobs)}] Registering task: @{job_params.agent}")

            result = self.add_job(job_params)
            results.append(result)

            if result.success:
                print(f"  OK: task={result.task_id or 'N/A'}  session={result.session_id or 'N/A'}")
            else:
                print(f"  FAILED: {result.error}")

            if i < len(jobs):
                time.sleep(0.5)

        return results


# ====== CLI Interface ======

def cmd_add_job(args):
    """Add a single scheduled task"""
    parser = argparse.ArgumentParser(description="Add a scheduled task to MindX Scheduler")
    parser.add_argument("--agent", required=True, help="Target agent (e.g., @writer)")
    parser.add_argument("--content", required=True, help="Message content to send")
    parser.add_argument("--cron", required=True, help="Cron expression (6 fields)")
    parser.add_argument("--session-id", default="",
                        help="Session UUID to resume (optional)")
    parser.add_argument("--project-dir", default="",
                        help="Project directory for execution context (optional)")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST, help="Gateway host address")
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT, help="Gateway port")

    opts = parser.parse_args(args)

    params = JobAddParams(
        agent=opts.agent,
        content=opts.content,
        cron_expr=opts.cron,
        session_id=opts.session_id,
        project_dir=opts.project_dir
    )

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.add_job(params)

        if result.success:
            print(json.dumps(result.data, indent=2, ensure_ascii=False))
            return 0
        else:
            print(f"Error: {result.error}")
            return 1


def cmd_list_jobs(args):
    """List all scheduled tasks"""
    parser = argparse.ArgumentParser(description="List all tasks in MindX Scheduler")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)
    parser.add_argument("--json", action="store_true", help="Output in JSON format")

    opts = parser.parse_args(args)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.list_jobs()

        if result.success:
            if opts.json:
                print(json.dumps(result.data, indent=2, ensure_ascii=False))
            else:
                entries = result.data
                if isinstance(entries, list):
                    if not entries:
                        print("No scheduled tasks")
                    else:
                        print(f"\nScheduled tasks ({len(entries)}):")
                        print("=" * 60)
                        for entry in entries:
                            status = "active" if entry.get("enabled", False) else "paused"
                            last_run = entry.get("last_run_at", "") or ""
                            last_status = entry.get("last_status", "") or ""
                            runs = f"ok:{entry.get('success_count', 0)} fail:{entry.get('failure_count', 0)}"
                            print(f"  ID:      {entry.get('id', '')}")
                            print(f"  Agent:   {entry.get('agent', '')}")
                            print(f"  Content: {entry.get('content', '')[:80]}")
                            print(f"  Cron:    {entry.get('cron_expr', '')}")
                            print(f"  Status:  {status}")
                            print(f"  Runs:    {runs}")
                            if last_run:
                                print(f"  Last:    {last_run} ({last_status})")
                            print("-" * 60)
                else:
                    print(entries)
            return 0
        else:
            print(f"Error: {result.error}")
            return 1


def cmd_del_job(args):
    """Delete a scheduled task"""
    parser = argparse.ArgumentParser(description="Delete a scheduled task")
    parser.add_argument("--id", required=True, help="Task ID to delete")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    opts = parser.parse_args(args)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.delete_job(opts.id)

        if result.success:
            print(json.dumps(result.data, indent=2, ensure_ascii=False))
            return 0
        else:
            print(f"Error: {result.error}")
            return 1


def cmd_batch_add(args):
    """Batch add tasks from a JSON file"""
    parser = argparse.ArgumentParser(
        description="Batch add scheduled tasks from a JSON file",
        epilog="""
JSON file format example:
[
    {
        "agent": "@writer",
        "content": "Every Monday: Write a technical blog post",
        "cron_expr": "0 0 9 * * 1",
        "session_id": "",
        "project_dir": "/Users/ray/workspaces/my-blog"
    },
    {
        "agent": "@analyst",
        "content": "Every Friday: Analyze data and generate report",
        "cron_expr": "0 0 16 * * 5",
        "session_id": "550e8400-e29b-41d4-a716-446655440000",
        "project_dir": "/Users/ray/workspaces/data-project"
    }
]

JSON-RPC: schedule.add
        """)
    parser.add_argument("--file", required=True, help="Path to JSON file")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    opts = parser.parse_args(args)

    try:
        with open(opts.file, 'r', encoding='utf-8') as f:
            jobs_data = json.load(f)
    except FileNotFoundError:
        print(f"File not found: {opts.file}")
        return 1
    except json.JSONDecodeError as e:
        print(f"JSON format error: {e}")
        return 1

    jobs = []
    for item in jobs_data:
        job = JobAddParams(
            agent=item.get("agent", ""),
            content=item.get("content", ""),
            cron_expr=item.get("cron_expr", ""),
            session_id=item.get("session_id", ""),
            project_dir=item.get("project_dir", "")
        )
        jobs.append(job)

    print(f"\nPreparing to batch register {len(jobs)} tasks...")
    print("=" * 60)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        results = client.batch_add_jobs(jobs)

    success_count = sum(1 for r in results if r.success)
    fail_count = len(results) - success_count

    print("\n" + "=" * 60)
    print(f"\nBatch operation complete:")
    print(f"   OK: {success_count}/{len(results)}")
    print(f"   FAILED: {fail_count}/{len(results)}")

    if fail_count > 0:
        print("\nFailed tasks:")
        for i, r in enumerate(results, 1):
            if not r.success:
                print(f"   {i}. {r.error}")
        return 1

    return 0


def cmd_test_connection(args):
    """Test connection to the Gateway"""
    parser = argparse.ArgumentParser(description="Test connection to MindX Gateway")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    opts = parser.parse_args(args)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.list_jobs()

        if result.success:
            entries = result.data
            count = len(entries) if isinstance(entries, list) else 0
            print("Connection successful! Gateway is running")
            print(f"Current registered tasks: {count}")
            return 0
        else:
            print(f"Connection failed: {result.error}")
            return 1


# ====== Main Entry Point ======

def main():
    parser = argparse.ArgumentParser(
        description="MindX Scheduler WebSocket Client Tool (JSON-RPC 2.0)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Available Commands:
  add-job      Add a single scheduled task
  list-jobs    List all scheduled tasks
  del-job      Delete a specific task
  batch-add    Batch add tasks from a JSON file
  test-conn    Test Gateway connection

Examples:
  # Add a task
  %(prog)s add-job --agent @writer --content "Daily reminder" --cron "0 0 9 * * *"

  # Add a task with project directory
  %(prog)s add-job --agent @writer --content "Project task" --cron "0 0 9 * * *" \\
      --project-dir /Users/ray/workspaces/my-project

  # List tasks
  %(prog)s list-jobs

  # List tasks as JSON
  %(prog)s list-jobs --json

  # Batch add
  %(prog)s batch-add --file tasks.json

  # Test connection
  %(prog)s test-conn

Protocol: JSON-RPC 2.0 over WebSocket
Methods:  schedule.list, schedule.add, schedule.del
        """)

    subparsers = parser.add_subparsers(dest="command", help="Available commands")

    p_add = subparsers.add_parser("add-job", help="Add a scheduled task")
    p_add.set_defaults(func=cmd_add_job)

    p_list = subparsers.add_parser("list-jobs", help="List all tasks")
    p_list.set_defaults(func=cmd_list_jobs)

    p_del = subparsers.add_parser("del-job", help="Delete a task")
    p_del.set_defaults(func=cmd_del_job)

    p_batch = subparsers.add_parser("batch-add", help="Batch add tasks")
    p_batch.set_defaults(func=cmd_batch_add)

    p_test = subparsers.add_parser("test-conn", help="Test connection")
    p_test.set_defaults(func=cmd_test_connection)

    args = parser.parse_args()

    if hasattr(args, 'func'):
        exit(args.func(sys.argv[2:]))
    else:
        parser.print_help()
        exit(1)


if __name__ == "__main__":
    main()
