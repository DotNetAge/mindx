---
name: calendar
description: 日历管理技能，查看、创建日历事件和日程安排
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

## 示例
```json
{
  "name": "calendar",
  "parameters": {
    "action": "list",
    "days": 14
  }
}
```
