#!/usr/bin/env python3
"""
Find a specific agent by name via daemon RPC.

Usage:
    python find_agent.py --name <agent_name>

Output: JSON object with the agent config or error if not found.
"""

import json
import os
import sys
import argparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    parser = argparse.ArgumentParser(description="Find an agent by name via daemon RPC")
    parser.add_argument("--name", required=True, help="Exact agent name to find")
    args = parser.parse_args()

    result = rpc_client.rpc_call("agent.get", {"name": args.name})
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
