---
name: cron
description: 定时任务管理，可以添加、列出、删除、暂停和恢复定时任务
version: 2.0.0
category: system
tags:
  - cron
  - schedule
  - 定时
  - 任务
  - 自动化
  - 提醒
  - 日程
  - 调度
os:
  - darwin
  - linux
  - windows
enabled: true
timeout: 30
is_internal: true
parameters:
  action:
    type: string
    description: "操作类型：add（添加任务）、list（列出任务）、delete（删除任务）、pause（暂停任务）、resume（恢复任务）"
    required: true
  name:
    type: string
    description: 任务名称，例如 "每日天气提醒"、"每周六写日报"（仅 action=add 时需要）
    required: false
  cron:
    type: string
    description: Cron 表达式，格式为 "分 时 日 月 周"，例如 "0 9 * * 6" 表示每周六早上9点（仅 action=add 时需要）
    required: false
  message:
    type: string
    description: 定时要发送的消息，例如 "帮我写日报"、"今天天气怎么样"（仅 action=add 时需要）
    required: false
  id:
    type: string
    description: 任务 ID（仅 action=delete/pause/resume 时需要）
    required: false
---

# 定时任务管理技能

统一的定时任务管理功能，可以添加、列出、删除、暂停和恢复定时任务。

## 设计理念

定时任务执行的第一逻辑应该是**一轮对话**，而不是直接调用技能。例如："每周六帮我写日报" 包含两个行为：
1. 每周六触发一次对话
2. 对话：帮我写日报

这样设计的好处：
- 即使客观条件产生变化，对话所执行也是大脑的正常对话路径
- 一切架构都没有进行过改变，也没有入侵性
- Cron 本身不关心技能是什么，只负责定时模拟发起对话

## 功能特点

- 使用系统原生调度器（crontab / Task Scheduler）
- 即使 MindX 不启动，任务也能正常执行
- 定时触发完整对话流程，走大脑正常处理路径
- 一个技能统一管理所有定时任务操作

## 操作类型

| action   | 说明                 |
| -------- | -------------------- |
| `add`    | 添加新的定时任务     |
| `list`   | 列出所有定时任务     |
| `delete` | 删除指定的定时任务   |
| `pause`  | 暂停指定的定时任务   |
| `resume` | 恢复已暂停的定时任务 |

## Cron 表达式格式

```
分 时 日 月 周
```

- 分：0-59
- 时：0-23
- 日：1-31
- 月：1-12
- 周：0-7（0和7都表示周日）

## 常用 Cron 示例

| 表达式         | 说明                        |
| -------------- | --------------------------- |
| `0 9 * * *`    | 每天早上9点                 |
| `0 9 * * 1-5`  | 工作日（周一至周五）早上9点 |
| `0 9 * * 6`    | 每周六早上9点               |
| `0 9 1 * *`    | 每月1号早上9点              |
| `*/30 * * * *` | 每30分钟                    |

## 使用方法

### 添加任务 (action=add)

```json
{
  "action": "add",
  "name": "每周六写日报",
  "cron": "0 9 * * 6",
  "message": "帮我写日报"
}
```

### 列出任务 (action=list)

```json
{
  "action": "list"
}
```

### 删除任务 (action=delete)

```json
{
  "action": "delete",
  "id": "job-uuid-1234"
}
```

### 暂停任务 (action=pause)

```json
{
  "action": "pause",
  "id": "job-uuid-1234"
}
```

### 恢复任务 (action=resume)

```json
{
  "action": "resume",
  "id": "job-uuid-1234"
}
```

## 输出格式

### add 操作输出

```
Cron job added with ID: job-uuid-1234
```

### list 操作输出

```json
[
  {
    "id": "job-uuid-1234",
    "name": "每周六写日报",
    "cron": "0 9 * * 6",
    "message": "帮我写日报",
    "enabled": true,
    "created_at": "2026-02-20T00:00:00Z",
    "last_run": "2026-02-20T09:00:00Z",
    "last_status": "success",
    "last_error": null
  }
]
```

### delete/pause/resume 操作输出

```
Cron job job-uuid-1234 deleted
Cron job job-uuid-1234 paused
Cron job job-uuid-1234 resumed
```

## 使用场景

- 需要定时提醒时
- 需要自动执行重复任务时
- 需要定期生成报告时
- 需要查看或管理已有定时任务时
