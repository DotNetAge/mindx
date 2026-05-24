"""
WebSocket JSON-RPC 2.0 client for MindX Daemon.

Thin wrapper for find-experts scripts to call daemon RPC methods.
Replaces direct file operations with daemon-managed RPC calls.

Usage:
    from rpc_client import rpc_call
    result = rpc_call("agent.list")
    result = rpc_call("agent.get", {"name": "some-agent"})
"""

import json
import os
import sys
import uuid

try:
    import websocket
except ImportError:
    print("websocket-client library is required", file=sys.stderr)
    print("Install with: pip install websocket-client", file=sys.stderr)
    sys.exit(1)

DEFAULT_HOST = "localhost"
DEFAULT_PORT = 1314
DEFAULT_PATH = "/ws"
TIMEOUT = 30


def _detect_endpoint():
    host = os.environ.get("MINDX_WS_HOST", DEFAULT_HOST)
    port = os.environ.get("MINDX_WS_PORT", str(DEFAULT_PORT))
    path = os.environ.get("MINDX_WS_PATH", DEFAULT_PATH)
    return host, int(port), path


def rpc_call(method, params=None):
    host, port, path = _detect_endpoint()
    url = f"ws://{host}:{port}{path}"

    try:
        ws = websocket.create_connection(url, timeout=TIMEOUT)
    except Exception as e:
        print(f'{{"error": "connection failed: {e}"}}', file=sys.stderr)
        sys.exit(1)

    request_id = str(uuid.uuid4())
    request = {
        "jsonrpc": "2.0",
        "id": request_id,
        "method": method,
    }
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

    try:
        response = json.loads(response_str)
    except json.JSONDecodeError:
        print(f'{{"error": "invalid JSON response"}}', file=sys.stderr)
        sys.exit(1)

    if "error" in response and response["error"] is not None:
        err = response["error"]
        msg = err.get("message", str(err))
        print(f'{{"error": "rpc error: {msg}"}}', file=sys.stderr)
        sys.exit(1)

    return response.get("result")
