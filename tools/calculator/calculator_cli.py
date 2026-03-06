#!/usr/bin/env python3

# Calculator CLI Skill
# 从stdin接收JSON参数，执行计算并返回结果

import sys
import json

def main():
    # 读取参数
    params = json.load(sys.stdin)
    
    # 解析参数
    expression = params.get("expression")
    
    if not expression:
        result = {"error": "Missing required parameter: expression"}
        print(json.dumps(result))
        sys.exit(1)
    
    try:
        # 安全地计算表达式
        result = eval(expression, {"__builtins__": {}}, {})
        
        output = {
            "expression": expression,
            "result": result
        }
        print(json.dumps(output))
    except Exception as e:
        output = {
            "error": f"Calculation error: {str(e)}"
        }
        print(json.dumps(output))
        sys.exit(1)

if __name__ == "__main__":
    main()
