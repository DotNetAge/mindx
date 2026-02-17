---
name: terminal
description: 终端命令执行技能，在终端中执行shell命令行指令
version: 1.0.0
category: system
tags:
  - terminal
  - command
  - shell
  - 终端
  - 命令行
  - 执行命令
  - shell命令
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./terminal_cli.sh
parameters:
  command:
    type: string
    description: 要执行的命令
    required: true
  timeout:
    type: number
    description: 超时时间（秒），默认30秒
    required: false
---

# 终端技能

## 示例
```json
{
  "name": "terminal",
  "parameters": {
    "command": "ls -la /Users"
  }
}
```
