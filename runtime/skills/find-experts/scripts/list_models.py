#!/usr/bin/env python3
"""
List available models from models.yml.

Usage:
    python list_models.py --models-file <path/to/models.yml>

Output: JSON array of model summaries.
"""

import os
import sys
import argparse

# Add scripts dir to path for yaml_helper import
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import yaml_helper


def main():
    parser = argparse.ArgumentParser(description="List available models")
    parser.add_argument("--models-file", required=True, help="Path to models.yml")
    args = parser.parse_args()

    if not os.path.isfile(args.models_file):
        yaml_helper.fail(f"Models file not found: {args.models_file}")

    data = yaml_helper.parse_yaml_file(args.models_file)
    models = data.get("models", [])

    summaries = []
    for m in models:
        if isinstance(m, dict):
            summaries.append({
                "name": m.get("name", ""),
                "provider": m.get("provider", ""),
                "description": m.get("description", ""),
                "max_tokens": m.get("max_tokens", None),
            })

    yaml_helper.output_json(summaries)


if __name__ == "__main__":
    main()
