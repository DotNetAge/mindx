# Session Management

Sessions are the unit of conversation between a user and an agent. Each session
has its own message history, context, and optionally associated file changes.
**All commands require the daemon to be running.**

## Lifecycle

```
create → (interact) → get/list → confirm/rollback → delete
```

## CRUD Operations

| Task | Command | Notes |
|------|---------|-------|
| Create new session | `mindx session create --agent <name>` | Starts fresh conversation |
| Set project directory | `mindx session create ... --project-dir /path` | Bind to working directory |
| List all sessions | `mindx session list` | All sessions across agents |
| Filter by agent | `mindx session list --agent csm-lead` | Only that agent's sessions |
| Get session details | `mindx session get --session-id <id>` | Full message history + metadata |
| Get metadata only | `mindx session meta --session-id <id>` | Lightweight — no messages |
| Delete session | `mindx session delete --session-id <id>` | **Destructive** — removes history |

## File Change Management

When an agent modifies files during a session, changes are tracked and can be
confirmed or rolled back.

| Task | Command | Notes |
|------|---------|-------|
| Confirm file changes | `mindx session confirm --session-id <id> --files "a.go,b.go"` | Accept modifications |
| Rollback file changes | `mindx session rollback --session-id <id> --files "a.go"` | Revert to pre-session state |

### Workflow
```bash
# 1. Create a session for a task
SESSION_ID=$(mindx session create --agent developer --project-dir ./myapp)

# 2. Agent works... (files are modified under this session)

# 3. Review what changed
mindx session meta --session-id $SESSION_ID

# 4. Confirm good changes, rollback bad ones
mindx session confirm --session-id $SESSION_ID --files "main.go,utils.go"
mindx session rollback --session-id $SESSION_ID --files "experimental.go"

# 5. When done, archive or delete
mindx session delete --session-id $SESSION_ID
```

## Session + Schedule Integration

Sessions and scheduled tasks work together:

```bash
# Create a session for recurring work
TASK_SESSION=$(mindx utils uuid)
mindx session create --agent weekly-reporter --session-id $TASK_SESSION

# Schedule agent to use this session ID
mindx schedule add \
  --agent weekly-reporter \
  --content "Generate weekly report. Use AgentTalk to report back in session '$TASK_SESSION'" \
  --cron "0 17 * * 5" \          # Every Friday at 5pm
  --session-id $TASK_SESSION

# Later: check what happened in that session
mindx session get --session-id $TASK_SESSION
```

This is how project-manager / customer-success skills link long-running
scheduled work back to trackable graph nodes.
