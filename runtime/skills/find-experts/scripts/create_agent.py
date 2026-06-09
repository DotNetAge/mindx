#!/usr/bin/env python3
"""
Create a new agent via daemon RPC.

Usage:
    python create_agent.py --name <agent_name> --role <agent_role> \\
        --description "<description>" --model <model_name> \\
        [--skills skill1,skill2,...]

Output: JSON object with status, agent_name, and message.
"""

import json
import os
import sys
import argparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    parser = argparse.ArgumentParser(description="Create a new agent via daemon RPC")
    parser.add_argument("--name", required=True, help="Agent identifier")
    parser.add_argument("--role", required=True, help="Short role title")
    parser.add_argument("--description", required=True, help="Detailed role description")
    parser.add_argument("--body", required=True, help="System prompt / core instructions for the agent")
    parser.add_argument("--model", required=True, help="Model name to use")
    parser.add_argument("--skills", type=str, default="", help="Comma-separated skill names")
    args = parser.parse_args()

    skills_list = []
    if args.skills:
        skills_list = [s.strip() for s in args.skills.split(",") if s.strip()]

    params = {
        "name": args.name,
        "role": args.role,
        "description": args.description,
        "body": args.body,
        "model": args.model,
    }
    if skills_list:
        params["skills"] = skills_list

    result = rpc_client.rpc_call("agent.create", params)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
