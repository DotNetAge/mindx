#!/usr/bin/env python3
"""
List all agents via daemon RPC.

Usage:
    python list_agents.py

Output: JSON array of agent summaries with name, role, description, model, skills.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    result = rpc_client.rpc_call("agent.list")
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
