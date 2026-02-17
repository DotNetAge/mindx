---
name: finder
description: Finder文件管理技能，浏览目录、查看文件信息、打开文件夹
version: 1.0.0
category: system
tags:
  - finder
  - files
  - 文件管理
  - 浏览文件
  - 文件夹
  - 目录
os:
  - darwin
enabled: true
timeout: 30
command: ./finder_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："list"列出目录、"open"打开目录、"info"获取文件信息
    required: true
  path:
    type: string
    description: 文件路径，默认为当前目录（.）
    required: false
---

# Finder 技能

## 示例
```json
{
  "name": "finder",
  "parameters": {
    "action": "list",
    "path": "/Users"
  }
}
```
