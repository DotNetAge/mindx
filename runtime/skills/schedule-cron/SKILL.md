---
name: schedule-cron
description: >
  Register, list, and delete recurring tasks via the MindX Scheduler.
  Handles cron-based scheduling for agent tasks.
allowed-tools: bash read write
metadata:
  name_zh: 定时任务
  name_zh-tw: 定時任務
  description_zh: 通过 MindX 调度器注册、列出和删除基于 cron 的定时重复任务
  description_zh-tw: 透過 MindX 排程器註冊、列出和刪除基於 cron 的定時重複任務
---

## When to Use

- User asks to schedule recurring agent work ("every day at 9", "weekly on Monday")
- User wants to list, delete, or check scheduled tasks

## How It Works

```
schedule.add    → Register a recurring task
schedule.list   → List all scheduled tasks
schedule.del    → Delete a task
```

All methods go through `scripts/scheduler_client.py` — never construct JSON-RPC manually.

## Commands

### Add a single task

```bash
python3 scripts/scheduler_client.py add-job \
    --agent "writer" --content "..." --cron "0 0 9 * * 1"
```

Optional: `--session-id <id>` (auto-generated if omitted), `--project-dir <path>`

### List all tasks

```bash
python3 scripts/scheduler_client.py list-jobs
python3 scripts/scheduler_client.py list-jobs --json   # machine-readable
```

### Delete a task

```bash
python3 scripts/scheduler_client.py del-job --id a1b2c3d4
```

### Test connection

```bash
python3 scripts/scheduler_client.py test-conn
```

## Input Validation Checklist

- **Agent**: must be a registered agent name (no `@` prefix needed, stripped automatically)
- **Cron**: 6-field format required (`0 0 9 * * 1`, includes seconds)
- **Content**: specific enough for the agent to produce good output
- **Session ID**: omit for auto-generated UUID, or provide to link to existing session

## Notes

- Gateway runs on `ws://localhost:1314/ws` (port 1314, not 8081)
- Task responses include `id`, `agent`, `cron_expr`, `enabled`, `success_count`, `failure_count`
- Always save the returned task ID for later management
