#!/usr/bin/env python3
"""
graph_client.py — Thin RPC wrapper for graph-db skill.

Maps CLI arguments to graph.* JSON-RPC methods.
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


def cmd_query(args):
    params = {"query": args.cypher}
    if args.params:
        try:
            params["params"] = json.loads(args.params)
        except json.JSONDecodeError:
            print('{"error": "--params must be valid JSON"}', file=sys.stderr)
            sys.exit(1)
    result = rpc_call("graph.query", params)
    print(json.dumps(result, indent=2))


def cmd_exec(args):
    params = {"query": args.cypher}
    if args.params:
        try:
            params["params"] = json.loads(args.params)
        except json.JSONDecodeError:
            print('{"error": "--params must be valid JSON"}', file=sys.stderr)
            sys.exit(1)
    result = rpc_call("graph.exec", params)
    print(json.dumps(result, indent=2))


def cmd_upsert_nodes(args):
    try:
        nodes = json.loads(args.nodes)
    except json.JSONDecodeError:
        print('{"error": "--nodes must be valid JSON array"}', file=sys.stderr)
        sys.exit(1)
    result = rpc_call("graph.upsert_nodes", {"nodes": nodes})
    print(json.dumps(result, indent=2))


def cmd_upsert_edges(args):
    try:
        edges = json.loads(args.edges)
    except json.JSONDecodeError:
        print('{"error": "--edges must be valid JSON array"}', file=sys.stderr)
        sys.exit(1)
    result = rpc_call("graph.upsert_edges", {"edges": edges})
    print(json.dumps(result, indent=2))


def cmd_get_node(args):
    result = rpc_call("graph.get_node", {"id": args.id})
    print(json.dumps(result, indent=2))


def cmd_neighbors(args):
    params = {"id": args.id, "depth": args.depth, "limit": args.limit}
    if args.types:
        params["types"] = [t.strip() for t in args.types.split(",")]
    result = rpc_call("graph.get_neighbors", params)
    print(json.dumps(result, indent=2))


def main():
    parser = argparse.ArgumentParser(
        description="Graph Database Client — RPC wrapper for gograph",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = parser.add_subparsers(dest="command")

    # query
    p_q = sub.add_parser("query", help="Cypher READ query")
    p_q.add_argument("--cypher", required=True)
    p_q.add_argument("--params", default="")

    # exec
    p_e = sub.add_parser("exec", help="Cypher WRITE query")
    p_e.add_argument("--cypher", required=True)
    p_e.add_argument("--params", default="")

    # upsert-nodes
    p_n = sub.add_parser("upsert-nodes", help="Batch create/update nodes")
    p_n.add_argument("--nodes", required=True, help="JSON array of node objects")

    # upsert-edges
    p_ed = sub.add_parser("upsert-edges", help="Batch create/update edges")
    p_ed.add_argument("--edges", required=True, help="JSON array of edge objects")

    # get-node
    p_g = sub.add_parser("get-node", help="Get single node by ID")
    p_g.add_argument("--id", required=True)

    # neighbors
    p_nb = sub.add_parser("neighbors", help="Get neighbor nodes")
    p_nb.add_argument("--id", required=True)
    p_nb.add_argument("--depth", type=int, default=1)
    p_nb.add_argument("--limit", type=int, default=50)
    p_nb.add_argument("--types", default="", help="Comma-separated edge type filter")

    args = parser.parse_args()
    cmds = {
        "query": cmd_query,
        "exec": cmd_exec,
        "upsert-nodes": cmd_upsert_nodes,
        "upsert-edges": cmd_upsert_edges,
        "get-node": cmd_get_node,
        "neighbors": cmd_neighbors,
    }
    fn = cmds.get(args.command)
    if fn:
        fn(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
