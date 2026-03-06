#!/usr/bin/env python3

import sys
import json
import subprocess

def main():
    params = json.load(sys.stdin)
    
    action = params.get("action", "files")
    pattern = params.get("pattern", "")
    path = params.get("path", ".")
    
    if not pattern:
        print(json.dumps({"error": "缺少必需参数: pattern"}))
        sys.exit(1)
    
    try:
        results = []
        
        if action == "files":
            cmd = ["find", path, "-iname", f"*{pattern}*"]
            output = subprocess.run(cmd, capture_output=True, text=True)
            if output.returncode == 0:
                results = output.stdout.strip().split('\n') if output.stdout.strip() else []
        
        elif action == "content":
            cmd = ["grep", "-r", "-l", pattern, path]
            output = subprocess.run(cmd, capture_output=True, text=True)
            if output.returncode == 0:
                results = output.stdout.strip().split('\n') if output.stdout.strip() else []
        
        elif action == "both":
            cmd_files = ["find", path, "-iname", f"*{pattern}*"]
            output_files = subprocess.run(cmd_files, capture_output=True, text=True)
            file_results = output_files.stdout.strip().split('\n') if output_files.stdout.strip() else []
            
            cmd_content = ["grep", "-r", "-l", pattern, path]
            output_content = subprocess.run(cmd_content, capture_output=True, text=True)
            content_results = output_content.stdout.strip().split('\n') if output_content.stdout.strip() else []
            
            results = list(set(file_results + content_results))
        
        print(json.dumps({
            "results": results,
            "count": len(results),
            "pattern": pattern
        }))
        
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

if __name__ == "__main__":
    main()
