#!/usr/bin/env python3
"""
Thin wrapper around agent_cli.py create subcommand.

Usage:
    python create_agent.py --name <name> --role <role> --description <desc> --model <model> [options...]
"""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from agent_cli import main

if __name__ == "__main__":
    sys.argv.insert(1, "create")
    main()
