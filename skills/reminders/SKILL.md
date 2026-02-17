---
name: reminders
description: 提醒事项管理技能，创建、列出、完成提醒任务和待办事项
version: 1.0.0
category: productivity
tags:
  - reminders
  - tasks
  - 提醒
  - 待办
  - 任务
  - 提醒事项
  - 待办事项
os:
  - darwin
enabled: true
timeout: 30
command: ./reminders_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："list"列出提醒、"add"添加提醒、"complete"完成提醒
    required: true
  title:
    type: string
    description: 提醒标题（add/complete时必需）
    required: false
  due_date:
    type: string
    description: 截止日期（格式YYYY/MM/DD HH:MM:SS）
    required: false
  priority:
    type: number
    description: 优先级（0-9），默认0
    required: false
---

# 提醒技能

## 示例
```json
{
  "name": "reminders",
  "parameters": {
    "action": "add",
    "title": "购买日用品"
  }
}
```
