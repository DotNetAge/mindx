---
name: mcp-filesystem
description: 通过 MCP 协议访问文件系统，支持文件读写和目录操作
version: 1.0.0
category: system
tags:
  - mcp
  - filesystem
  - file
os:
  - darwin
  - linux
enabled: true
timeout: 30
parameters:
  path:
    type: string
    description: 文件或目录路径
    required: true
  action:
    type: string
    description: 操作类型：read, write, list
    required: true
  content:
    type: string
    description: 文件内容（write 操作时必需）
    required: false
metadata:
  mcp:
    server: "filesystem"
    tool: "filesystem_operation"
---

# MCP 文件系统技能

通过 MCP 协议提供文件系统操作能力。

## 功能说明

- 读取文件内容
- 写入文件内容
- 列出目录内容

## 使用示例

```json
{
  "name": "mcp-filesystem",
  "parameters": {
    "path": "/tmp/test.txt",
    "action": "read"
  }
}
```

```json
{
  "name": "mcp-filesystem",
  "parameters": {
    "path": "/tmp/test.txt",
    "action": "write",
    "content": "Hello, MCP!"
  }
}
```
