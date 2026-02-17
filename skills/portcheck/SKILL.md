---
name: portcheck
description: 端口占用查询技能，查看指定端口的占用情况、进程信息
version: 1.0.0
category: system
tags:
  - port
  - 端口
  - 端口占用
  - 端口查询
  - 网络
  - 进程
os:
  - darwin
  - linux
enabled: true
timeout: 10
command: ./portcheck_cli.sh
parameters:
  port:
    type: number
    description: 要查询的端口号，如 8080、3000
    required: true
output_format: |
  请以 Markdown 表格格式输出结果，包含以下列：端口号,状态,进程名,进程ID,用户
guidance: |
  1. 端口号必须是 1-65535 之间的数字
  2. 如果端口未被占用，状态显示"空闲"，进程名/进程ID/用户显示"-"
  3. 如果端口被占用，显示占用进程的详细信息
---

# 端口占用查询技能

## 功能
- 查看指定端口是否被占用
- 显示占用端口的进程信息（PID、进程名、用户）

## 示例
```json
{
  "name": "portcheck",
  "parameters": {
    "port": 8080
  }
}
```

## 返回格式
```json
{
  "port": 8080,
  "in_use": true,
  "process": {
    "pid": 12345,
    "name": "node",
    "user": "ray"
  }
}
```
