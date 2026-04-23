---
name: notes
description: "Creates, lists, and opens notes in macOS Notes via AppleScript. Use when the user wants to jot something down, find a note, view their notes, or manage memos."
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

Manages notes in macOS Notes app. Supports creating, listing, and opening notes.

## Workflow

1. Determine the action: `create`, `list`, or `open`.
2. For `create`: provide `title` (required) and `content` (optional).
3. For `open`: provide the exact `title` of the note to open.
4. For `list`: no extra parameters needed — returns all note titles.

## Examples

Create a note:

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

List all notes:

```json
{
  "name": "notes",
  "parameters": {
    "action": "list"
  }
}
```

Open a specific note:

```json
{
  "name": "notes",
  "parameters": {
    "action": "open",
    "title": "会议记录"
  }
}
```
