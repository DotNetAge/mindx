#!/usr/bin/env python3
"""
List all skills via daemon RPC.

Usage:
    python list_skills.py

Output: JSON array of skill summaries with name and description.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    result = rpc_client.rpc_call("skill.list")
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
