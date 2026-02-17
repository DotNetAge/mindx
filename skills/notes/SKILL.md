---
name: notes
description: 笔记管理技能，创建、列出、打开笔记，管理备忘录
version: 1.0.0
category: productivity
tags:
  - notes
  - productivity
  - 笔记
  - 备忘录
  - 记事本
  - 创建笔记
os:
  - darwin
enabled: true
timeout: 30
command: ./notes_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："create"创建笔记、"list"列出笔记、"open"打开笔记
    required: true
  title:
    type: string
    description: 笔记标题（create/open时必需）
    required: false
  content:
    type: string
    description: 笔记内容（创建时使用）
    required: false
---

# 笔记技能

## 示例
```json
{
  "name": "notes",
  "parameters": {
    "action": "create",
    "title": "会议记录",
    "content": "重要要点..."
  }
}
```
