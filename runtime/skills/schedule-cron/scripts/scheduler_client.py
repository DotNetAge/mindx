#!/usr/bin/env python3
"""
MindX Scheduler WebSocket Client (JSON-RPC 2.0)

Communicates with MindX Gateway via WebSocket using JSON-RPC 2.0 protocol
to send scheduling commands: job-add, job-list, job-del.

Protocol:
    Request:  {"jsonrpc":"2.0","id":"<uuid>","method":"<command>","params":{"args":"..."}}
    Response: {"jsonrpc":"2.0","id":"<uuid>","result":"..."} or {"error":{"code":...,"message":"..."}}

Usage Examples:
    # Add a scheduled task
    python scheduler_client.py add-job --agent @writer --content "Daily blog post" --cron "0 0 9 * * 1"

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
    print("❌ Error: websocket-client library is required")
    print("   Install with: pip install websocket-client")
    sys.exit(1)


# ====== Configuration ======

DEFAULT_GATEWAY_HOST = "localhost"
DEFAULT_GATEWAY_PORT = 8081
DEFAULT_GATEWAY_PATH = "/ws"
DEFAULT_TIMEOUT = 30  # seconds


# ====== Data Models ======

@dataclass
class ScheduledJobResult:
    """Result of a Scheduler operation"""
    success: bool
    command: str
    data: Optional[str] = None
    error: Optional[str] = None
    raw_response: Optional[str] = None
    task_id: Optional[str] = None


@dataclass
class JobInfo:
    """Information about a scheduled task"""
    id: str
    agent: str
    content: str
    cron_expr: str
    status: str
    success_count: int = 0
    failure_count: int = 0


@dataclass
class JobAddParams:
    """Parameters for adding a scheduled task"""
    agent: str          # Target agent (e.g., @writer)
    content: str        # Message content to send
    cron_expr: str       # 6-field Cron expression


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
            print(f"❌ Connection failed: {e}")
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

    def _send_jsonrpc_request(self, method: str, params: dict = None) -> dict:
        """
        Send a JSON-RPC 2.0 request and return the response.

        Protocol format:
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

    def _send_command(self, command: str, args: str = "") -> dict:
        """
        Send a command using JSON-RPC 2.0 protocol.

        The command is sent as a method call with args wrapped in params.
        """
        params = {"args": args}
        return self._send_jsonrpc_request(command, params)

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.disconnect()

    # ====== High-Level Methods ======

    def add_job(self, params: JobAddParams) -> ScheduledJobResult:
        """
        Add a scheduled task.

        Args:
            params: Task parameters (agent, content, cron_expr)

        Returns:
            Operation result containing task_id and other info
        """
        agent = params.agent if params.agent.startswith("@") else f"@{params.agent}"
        args_str = f"{agent} {params.content} expr=\"{params.cron_expr}\""

        result_dict = self._send_command("job-add", args_str)

        result = ScheduledJobResult(
            success=result_dict["success"],
            command="job-add",
            raw_response=result_dict.get("raw")
        )

        if result.success:
            result.data = result_dict.get("data", "")
            if isinstance(result.data, str) and result.data:
                for line in result.data.split("\n"):
                    if "ID:" in line:
                        parts = line.split(":")
                        if len(parts) >= 2:
                            result.task_id = parts[-1].strip()
                            break
            elif isinstance(result.data, dict):
                result.task_id = result.data.get("id")
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def list_jobs(self) -> ScheduledJobResult:
        """
        List all scheduled tasks.

        Returns:
            Operation result containing the task list
        """
        result_dict = self._send_command("job-list", "")

        result = ScheduledJobResult(
            success=result_dict["success"],
            command="job-list",
            raw_response=result_dict.get("raw")
        )

        if result.success:
            result.data = result_dict.get("data", "")
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def delete_job(self, task_id: str) -> ScheduledJobResult:
        """
        Delete a scheduled task.

        Args:
            task_id: The task ID to delete

        Returns:
            Operation result
        """
        args_str = f"id={task_id}"
        result_dict = self._send_command("job-del", args_str)

        result = ScheduledJobResult(
            success=result_dict["success"],
            command="job-del",
            raw_response=result_dict.get("raw")
        )

        if result.success:
            result.data = result_dict.get("data", "")
        else:
            result.error = result_dict.get("error", "Unknown error")

        return result

    def batch_add_jobs(self, jobs: list[JobAddParams]) -> list[ScheduledJobResult]:
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
                print(f"  ✅ Success: {result.task_id or 'N/A'}")
                if result.data:
                    if isinstance(result.data, str):
                        for line in str(result.data).split("\n")[:5]:
                            if line.strip():
                                print(f"     {line}")
                    else:
                        print(f"     {result.data}")
            else:
                print(f"  ❌ Failed: {result.error}")

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
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST, help="Gateway host address")
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT, help="Gateway port")

    opts = parser.parse_args(args)

    params = JobAddParams(
        agent=opts.agent,
        content=opts.content,
        cron_expr=opts.cron
    )

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.add_job(params)

        if result.success:
            print("\n" + "=" * 60)
            print(result.data)
            print("=" * 60)
            return 0
        else:
            print(f"\n❌ Error: {result.error}")
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
                print(json.dumps({
                    "success": True,
                    "data": result.data,
                    "raw": result.raw_response
                }, indent=2, ensure_ascii=False))
            else:
                print("\n" + result.data)
            return 0
        else:
            print(f"❌ Error: {result.error}")
            return 1


def cmd_del_job(args):
    """Delete a scheduled task"""
    parser = argparse.ArgumentParser(description="Delete a scheduled task")
    parser.add_argument("--id", required=True, help="Task ID to delete")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, DEFAULT=DEFAULT_GATEWAY_PORT)

    opts = parser.parse_args(args)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        result = client.delete_job(opts.id)

        if result.success:
            print("\n" + result.data)
            return 0
        else:
            print(f"\n❌ Error: {result.error}")
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
        "cron_expr": "0 0 9 * * 1"
    },
    {
        "agent": "@analyst",
        "content": "Every Friday: Analyze data and generate report",
        "cron_expr": "0 0 16 * * 5"
    }
]
        """)
    parser.add_argument("--file", required=True, help="Path to JSON file")
    parser.add_argument("--host", default=DEFAULT_GATEWAY_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_GATEWAY_PORT)

    opts = parser.parse_args(args)

    try:
        with open(opts.file, 'r', encoding='utf-8') as f:
            jobs_data = json.load(f)
    except FileNotFoundError:
        print(f"❌ File not found: {opts.file}")
        return 1
    except json.JSONDecodeError as e:
        print(f"❌ JSON format error: {e}")
        return 1

    jobs = []
    for item in jobs_data:
        job = JobAddParams(
            agent=item.get("agent", ""),
            content=item.get("content", ""),
            cron_expr=item.get("cron_expr", "")
        )
        jobs.append(job)

    print(f"\n📋 Preparing to batch register {len(jobs)} tasks...")
    print("=" * 60)

    with MindXSchedulerClient(host=opts.host, port=opts.port) as client:
        results = client.batch_add_jobs(jobs)

    success_count = sum(1 for r in results if r.success)
    fail_count = len(results) - success_count

    print("\n" + "=" * 60)
    print(f"\n📊 Batch operation complete:")
    print(f"   ✅ Success: {success_count}/{len(results)}")
    print(f"   ❌ Failed: {fail_count}/{len(results)}")

    if fail_count > 0:
        print("\n⚠️ Failed tasks:")
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
            print("✅ Connection successful! Gateway is running")
            data_str = result.data if isinstance(result.data, str) else json.dumps(result.data)
            print(f"\nCurrent registered tasks: {len(data_str) if data_str else 0}")
            return 0
        else:
            print(f"❌ Connection failed: {result.error}")
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

  # List tasks
  %(prog)s list-jobs

  # Batch add
  %(prog)s batch-add --file tasks.json

  # Test connection
  %(prog)s test-conn

Protocol: JSON-RPC 2.0 over WebSocket
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
