# Scheduler Integration

Register recurring tasks via `mindx schedule` CLI (JSON-RPC 2.0 over WebSocket, port 1314).

**Methods:** `schedule.add`, `schedule.list`, `schedule.del`

---

## `schedule.add`

### Parameters

| Field         | Type   | Required | Description                                           |
| ------------- | ------ | -------- | ----------------------------------------------------- |
| `agent`       | string | yes      | Agent name                               |
| `content`     | string | yes      | Prompt to send to the agent                           |
| `cron_expr`   | string | yes      | 6-field cron expression                               |
| `session_id`  | string | no       | Link to a graph task ID (recommended)                 |
| `project_dir` | string | no       | Working directory                                     |

### Request (via CLI)

```bash
mindx schedule add --agent writer --content "Daily standup" --cron "0 0 9 * * *" --session-id "task-xxx" --project-dir /path
```

### Response

```json
{"id":"a1b2c3","agent":"writer","content":"Daily standup","cron_expr":"0 0 9 * * *","session_id":"task-xxx","enabled":true}
```

### Error

```json
{"code":-32600,"message":"agent is required"}
```

---

## `schedule.list`

```bash
mindx schedule list
```

Returns array of schedule entries (same fields as `schedule.add` response).

---

## `schedule.del`

| Field | Type   | Required | Description                                          |
| ----- | ------ | -------- | ---------------------------------------------------- |
| `id`  | string | yes      | 8-char hex ID from `schedule.add` or `schedule.list` |

```bash
mindx schedule delete --id a1b2c3
```

Response: `{"status":"deleted","id":"a1b2c3"}`

---

## Usage (for Agent)

### Create a recurring task

```bash
mindx schedule add --agent writer --content "Write blog monthly" --cron "0 0 9 * * 1" --session-id "task-xxx" --project-dir /path
```

### List all tasks

```bash
mindx schedule list
```

---

## Important Notes

- Cron uses **6-field** format (seconds, minute, hour, day, month, weekday)
- Pass the graph task ID as `--session-id` so execution reports are linked to the task
- Session is auto-generated as UUID v4 if omitted
