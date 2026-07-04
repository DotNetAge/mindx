---
name: scheduler
description: >
  Create, list, and delete recurring scheduled tasks.
  Use when the user asks to schedule, automate, set up recurring, or cron jobs
  for agents.
allowed-tools:
  - Bash(mindx schedule *)
metadata:
  name_zh: 定时任务
  name_zh-tw: 定時任務
  description_zh: 创建、查看和删除定时任务
  description_zh-tw: 建立、檢視和刪除定時任務
---

# Scheduler: Recurring Task Management

You are managing scheduled tasks through the `mindx schedule` CLI.

## Commands Reference

### `mindx schedule list`

List all scheduled tasks.

```
mindx schedule list
```

Output: a table with columns **ID**, **Agent**, **Cron**, **Enabled**, **Created**.

### `mindx schedule add`

Add a new recurring scheduled task.

Required flags:
- `--agent` — Target agent name (e.g. `writer`, `architect`)
- `--content` — Prompt content to send to the agent when the schedule fires
- `--cron` — 6-field cron expression (e.g. `"0 0 9 * * *"` for daily at 09:00)

Optional flags:
- `--session-id` — Link an existing session UUID or graph task ID
- `--project-dir` — Set the project working directory for the task
- `--enabled` — Enable immediately (default: `true`; pass `--enabled=false` to create disabled)

Examples:

```
mindx schedule add \
  --agent writer \
  --content "Daily standup report" \
  --cron "0 0 9 * * *"

mindx schedule add \
  --agent writer \
  --content "Blog post" \
  --cron "0 0 9 * * 1" \
  --session-id "task-abc123" \
  --project-dir /path/to/project

mindx schedule add \
  --agent architect \
  --content "Review open PRs and summarize" \
  --cron "0 0 10 * * 1-5" \
  --enabled false
```

### `mindx schedule delete`

Delete a scheduled task by its ID.

Required flags:
- `--id` — Schedule entry ID (shown in `list` output)

Example:

```
mindx schedule delete --id a1b2c3d4
```

## Common Cron Patterns

| Purpose           | Cron Expression  | Description                    |
| ----------------- | ---------------- | ------------------------------ |
| Daily at 9 AM     | `0 0 9 * * *`    | Every day at 09:00             |
| Weekdays at 10 AM | `0 0 10 * * 1-5` | Mon–Fri at 10:00               |
| Weekly on Monday  | `0 0 9 * * 1`    | Every Monday at 09:00          |
| Every hour        | `0 * * * *`      | At minute 0 of every hour      |
| Every 30 minutes  | `*/30 * * * *`   | At :00 and :30 past every hour |
| Monthly on 1st    | `0 0 9 1 * *`    | 1st of every month at 09:00    |
