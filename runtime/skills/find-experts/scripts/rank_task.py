#!/usr/bin/env python3
"""
Record a performance score for an agent via daemon RPC.

Scores are stored in the agent's metadata under meta.performance.scores,
persisted through the daemon's agent registry.

Usage:
    python rank_task.py --agent-name <expert_name> \\
        --task "<task_description>" --score <1-10> \\
        [--notes "<evaluation_notes>"]

Output: JSON object with status, agent, task, score, timestamp,
        all historical scores, and completions count.
"""

import json
import os
import sys
import argparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    parser = argparse.ArgumentParser(description="Score an agent's task performance via daemon RPC")
    parser.add_argument("--agent-name", required=True, help="Name of agent to score")
    parser.add_argument("--task", required=True, help="Description of the delegated task")
    parser.add_argument("--score", type=int, required=True, help="Score from 1 to 10")
    parser.add_argument("--notes", type=str, default="", help="Optional evaluation notes")
    args = parser.parse_args()

    if args.score < 1 or args.score > 10:
        print('{"error": "score must be between 1 and 10"}', file=sys.stderr)
        sys.exit(1)

    params = {
        "agent_name": args.agent_name,
        "task": args.task,
        "score": args.score,
    }
    if args.notes:
        params["notes"] = args.notes

    result = rpc_client.rpc_call("agent.score", params)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
