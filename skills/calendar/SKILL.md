---
name: calendar
description: "Lists and creates events in macOS Calendar via AppleScript. Use when the user asks about their schedule, upcoming events, meetings, or wants to add a calendar entry."
version: 1.0.0
category: productivity
tags:
  - calendar
  - events
  - 日历
  - 日程
  - 事件
  - 安排
  - 行程
os:
  - darwin
enabled: true
timeout: 30
command: ./calendar_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："list"列出事件、"create"创建事件
    required: true
  title:
    type: string
    description: 事件标题（创建时需要）
    required: false
  start_date:
    type: string
    description: 开始日期（格式YYYY/MM/DD，创建时需要）
    required: false
  end_date:
    type: string
    description: 结束日期（格式YYYY/MM/DD）
    required: false
  days:
    type: number
    description: 列出未来几天的事件，默认7天
    required: false
---

# 日历技能

Manages macOS Calendar events. Supports listing upcoming events and creating new ones.

## Workflow

1. Determine the action: `list` to view events, `create` to add one.
2. For `list`: optionally set `days` (default 7) or a `start_date`/`end_date` range.
3. For `create`: provide `title` and `start_date` (required); `end_date` is optional.
4. Parse the returned JSON for event summaries or confirmation.

## Examples

List events for the next 14 days:

```json
{
  "name": "calendar",
  "parameters": {
    "action": "list",
    "days": 14
  }
}
```

Create a new event:

```json
{
  "name": "calendar",
  "parameters": {
    "action": "create",
    "title": "团队周会",
    "start_date": "2025/01/15"
  }
}
```
