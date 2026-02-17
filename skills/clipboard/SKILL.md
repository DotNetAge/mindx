---
name: clipboard
description: 剪贴板管理技能，读取和写入剪贴板内容，复制粘贴文本
version: 1.0.0
category: system
tags:
  - clipboard
  - copy
  - 剪贴板
  - 复制
  - 粘贴
  - 复制粘贴
os:
  - darwin
enabled: true
timeout: 30
command: ./clipboard_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："get"读取剪贴板，"set"写入剪贴板
    required: true
  text:
    type: string
    description: 要写入的文本内容（仅在action为"set"时需要）
    required: false
---

# 剪贴板技能

## 示例
```json
{
  "name": "clipboard",
  "parameters": {
    "action": "get"
  }
}
```
