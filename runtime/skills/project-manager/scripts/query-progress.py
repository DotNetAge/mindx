#!/usr/bin/env python3
"""
Query project progress — status, completion rate, issues.

Usage:
  query-progress.py --project-id <id>
  query-progress.py --project-id <id> --json
"""

import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import graph_client


def main():
    parser = argparse.ArgumentParser(description="Query project progress")
    parser.add_argument("--project-id", required=True, help="Project ID to query")
    args = parser.parse_args()

    class Args:
        project_id = args.project_id
        db_path = None

    graph_client.cmd_progress_report(Args)


if __name__ == "__main__":
    main()
