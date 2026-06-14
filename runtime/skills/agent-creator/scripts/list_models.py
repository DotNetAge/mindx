#!/usr/bin/env python3
"""
List available models.

Usage:
    python list_models.py

Output: JSON array of model summaries with name, description, and max_tokens.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def main():
    result = rpc_client.rpc_call("model.list")
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
