# MindX Scheduler WebSocket Protocol

This document describes the CMD protocol used to communicate with MindX Scheduler via the Gateway WebSocket channel.

## Protocol Overview

### Message Format

All messages follow this structure:

```
CMD|<command_name>|<arguments>|||
```

| Component | Description | Example |
|-----------|-------------|---------|
| `CMD` | Fixed protocol prefix | Always `CMD` |
| `<command_name>` | Command to execute | `job-add`, `job-list`, `job-del` |
| `<arguments>` | Command-specific arguments | `@agent content expr="cron"` |
| `\|\|\|` | Fixed terminator | Always `\|\|\|` |

### Response Format

Responses are JSON objects:

**Success:**
```json
{
    "cmd": "CMD",
    "name": "<command_name>",
    "data": "<response data>"
}
```

**Error:**
```json
{
    "cmd": "CMD",
    "name": "<command_name>",
    "error": "<error message>"
}
```

---

## Commands

### job-add

Registers a new scheduled message.

**Arguments format:**
```
@<agent> <content> expr="<cron_expression>"
```

**Parameters:**

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `@<agent>` | Yes | Target agent (must start with @) | `@writer` |
| `<content>` | Yes | Message content to send to agent | `"Weekly blog post"` |
| `expr="<cron>"` | Yes | 6-field Cron expression | `expr="0 0 9 * * 1"` |

**Example message:**
```
CMD|job-add|@writer Weekly technical blog post expr="0 0 9 * * 1"|||
```

**Success response:**
```json
{
    "cmd": "CMD",
    "name": "job-add",
    "data": "✅ Scheduled message created:\n  ID: a1b2c3d4\n  Target: @writer\n  Content: Weekly technical blog post\n  Schedule: 0 0 9 * * 1"
}
```

**Error response:**
```json
{
    "cmd": "CMD",
    "name": "job-add",
    "error": "Missing target agent: use @<agent-name> format to specify one"
}
```

---

### job-list

Lists all registered scheduled messages.

**Arguments:** None

**Example message:**
```
CMD|job-list|||
```

**Success response (table format):**
```json
{
    "cmd": "CMD",
    "name": "job-list",
    "response_type": "table",
    "data": {
        "title": "Scheduled Message Tasks",
        "headers": ["ID", "Target Agent", "Content", "Schedule", "Status", "Success/Fail"],
        "rows": [
            ["a1b2c3", "@writer", "Weekly blog post", "0 0 9 * * 1", "✅ Enabled", "10/0"],
            ["d4e5f6", "@analyst", "Analytics report", "0 0 16 * * 5", "✅ Enabled", "5/1"]
        ]
    }
}
```

---

### job-del

Deletes a registered scheduled message.

**Arguments format:**
```
id=<task_id>
```

**Parameters:**

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `id=<task_id>` | Yes | Task ID (8-character string) | `id=a1b2c3d4` |

**Example message:**
```
CMD|job-del|id=a1b2c3d4|||
```

**Success response:**
```json
{
    "cmd": "CMD",
    "name": "job-del",
    "data": "🗑️ Scheduled message deleted:\n  ID: a1b2c3d4\n  Target: @writer\n  Content: Weekly blog post"
}
```

**Error response:**
```json
{
    "cmd": "CMD",
    "name": "job-del",
    "error": "Task not found: a1b2c3d4"
}
```

---

## Connection Details

### WebSocket URL

```
ws://<gateway-host>:<gateway-port>/ws
```

**Default:** `ws://localhost:8081/ws`

### Headers

| Header | Value | Required |
|--------|-------|----------|
| `Origin` | `ws://<host>:<port>` | Yes |

### Timeout

Default connection timeout: 30 seconds.

### Connection Flow

1. Open WebSocket connection to Gateway URL
2. Send CMD protocol message (text frame)
3. Wait for JSON response (text frame)
4. Parse response and check for error field
5. Close connection when done

---

## Error Handling

### Common Errors

| Error Pattern | Cause | Solution |
|---------------|-------|----------|
| `Connection refused` | Gateway not running | Start the MindX Gateway service |
| `Missing target agent` | Agent name missing or no @ prefix | Use format `@agent-name` |
| `Invalid cron expression` | Wrong cron format or invalid syntax | Use 6-field format: `0 0 9 * * 1` |
| `Task not found` | Incorrect task ID | Get ID from `job-list` output |
| `Timeout` | Gateway unresponsive | Check Gateway health, increase timeout |

### Retry Strategy

- Connection errors: Retry 3 times with 2-second backoff
- Command errors: Do NOT retry — the command format is invalid
- Timeout errors: Retry once with increased timeout

---

## Architecture

```
┌─────────────────────────────────────────────┐
│              Skill (LLM)                     │
│  Reads SKILL.md, decides what to do          │
└──────────────────┬──────────────────────────┘
                   │ Invokes script
                   ▼
┌─────────────────────────────────────────────┐
│     scripts/scheduler_client.py              │
│  Python WebSocket client                     │
│  - Constructs CMD messages                   │
│  - Parses JSON responses                     │
│  - Formats output for user                   │
└──────────────────┬──────────────────────────┘
                   │ WebSocket connection
                   ▼
┌─────────────────────────────────────────────┐
│         MindX Gateway                        │
│  - Routes CMD messages to Scheduler          │
│  - Returns formatted responses               │
└──────────────────┬──────────────────────────┘
                   │ Internal call
                   ▼
┌─────────────────────────────────────────────┐
│          MindX Scheduler                     │
│  - Stores entries in JSON files              │
│  - Executes on Cron schedule                 │
│  - Sends content to target Agent             │
└─────────────────────────────────────────────┘
```

---

## Implementation Notes

- The `scheduler_client.py` script handles all protocol details
- Scripts should be called via `bash` or `python3` commands from the skill
- Never construct CMD messages manually — always use the script
- The script validates input before sending to prevent protocol errors
- Responses are formatted for human readability by default, with `--json` flag for machine parsing
