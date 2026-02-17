---
name: calculator
description: 计算器技能，执行数学计算和运算表达式
version: 1.0.0
category: general
tags:
  - calculator
  - math
  - 计算器
  - 计算
  - 数学
  - 运算
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./calculator_cli.py
parameters:
  expression:
    type: string
    description: 数学表达式，如"2+3*4"、"sin(0.5)"
    required: true
---

# 计算器技能

## 示例
```json
{
  "name": "calculator",
  "parameters": {
    "expression": "2+3*4"
  }
}
```
