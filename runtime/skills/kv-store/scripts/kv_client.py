#!/usr/bin/env python3
"""
kv_client.py — Thin RPC wrapper for kv-store skill.

Maps CLI arguments to kvstore.* JSON-RPC methods.
"""

import argparse
import json
import os
import sys
import uuid

try:
    import websocket
except ImportError:
    print("ERROR: websocket-client required. pip install websocket-client", file=sys.stderr)
    sys.exit(1)

DEFAULT_HOST = os.environ.get("MINDX_WS_HOST", "localhost")
DEFAULT_PORT = int(os.environ.get("MINDX_WS_PORT", "1314"))
DEFAULT_PATH = os.environ.get("MINDX_WS_PATH", "/ws")
TIMEOUT = 30


def rpc_call(method, params=None):
    url = f"ws://{DEFAULT_HOST}:{DEFAULT_PORT}{DEFAULT_PATH}"
    try:
        ws = websocket.create_connection(url, timeout=TIMEOUT)
    except Exception as e:
        print(f'{{"error": "connection failed: {e}"}}', file=sys.stderr)
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
    return response.get("result")


def _parse_value(raw):
    """Parse a CLI value arg: try JSON first, fall back to raw string."""
    if raw is None:
        return None
    stripped = raw.strip()
    if not stripped:
        return ""
    # Try JSON parse for objects, arrays, numbers, bools, null
    if stripped.startswith(("{", "[", '"')) or stripped in ("true", "false", "null"):
        try:
            return json.loads(stripped)
        except json.JSONDecodeError:
            pass
    # Try as number
    try:
        if "." in stripped:
            return float(stripped)
        return int(stripped)
    except ValueError:
        pass
    # Fall back to string
    return stripped


def cmd_get(args):
    result = rpc_call("kvstore.get", {"key": args.key})
    print(json.dumps(result, indent=2))


def cmd_set(args):
    value = _parse_value(args.value)
    params = {"key": args.key, "value": value}
    if args.ttl and args.ttl > 0:
        params["ttl"] = args.ttl
    result = rpc_call("kvstore.set", params)
    print(json.dumps(result, indent=2))


def cmd_delete(args):
    result = rpc_call("kvstore.delete", {"key": args.key})
    print(json.dumps(result, indent=2))


def cmd_list(args):
    params = {"prefix": args.prefix or "", "limit": args.limit}
    if args.with_values:
        params["with_values"] = True
    result = rpc_call("kvstore.list", params)
    print(json.dumps(result, indent=2))


def cmd_batch_set(args):
    try:
        entries = json.loads(args.entries)
    except json.JSONDecodeError:
        print('{"error": "--entries must be valid JSON array"}', file=sys.stderr)
        sys.exit(1)
    if not isinstance(entries, list):
        print('{"error": "--entries must be a JSON array"}', file=sys.stderr)
        sys.exit(1)
    # Parse any string-typed values in entries
    for e in entries:
        if "value" in e and isinstance(e["value"], str):
            e["value"] = _parse_value(e["value"])
    result = rpc_call("kvstore.batch_set", {"entries": entries})
    print(json.dumps(result, indent=2))


def cmd_clear(args):
    result = rpc_call("kvstore.clear", {"prefix": args.prefix or ""})
    print(json.dumps(result, indent=2))


def main():
    parser = argparse.ArgumentParser(
        description="KV Store Client — RPC wrapper for bbolt-backed kvstore",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = parser.add_subparsers(dest="command")

    # get
    p_get = sub.add_parser("get", help="Read a key")
    p_get.add_argument("--key", required=True)

    # set
    p_set = sub.add_parser("set", help="Write a key (with optional TTL)")
    p_set.add_argument("--key", required=True)
    p_set.add_argument("--value", required=True)
    p_set.add_argument("--ttl", type=int, default=0, help="Seconds until expiry (0=no expiry)")

    # delete
    p_del = sub.add_parser("delete", help="Delete a key")
    p_del.add_argument("--key", required=True)

    # list
    p_list = sub.add_parser("list", help="Prefix scan keys")
    p_list.add_argument("--prefix", default="")
    p_list.add_argument("--limit", type=int, default=100)
    p_list.add_argument("--with-values", action="store_true")

    # batch-set
    p_batch = sub.add_parser("batch-set", help="Atomic batch write")
    p_batch.add_argument("--entries", required=True, help="JSON array of {key,value,ttl?}")

    # clear
    p_clear = sub.add_parser("clear", help="Delete all keys matching prefix")
    p_clear.add_argument("--prefix", required=True)

    args = parser.parse_args()
    cmds = {
        "get": cmd_get,
        "set": cmd_set,
        "delete": cmd_delete,
        "list": cmd_list,
        "batch-set": cmd_batch_set,
        "clear": cmd_clear,
    }
    fn = cmds.get(args.command)
    if fn:
        fn(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
