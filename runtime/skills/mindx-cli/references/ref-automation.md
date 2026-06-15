# Automation & Statistics

Scheduled tasks, token usage tracking, and translation.

## Scheduled Tasks (Cron)

Run agents on a recurring schedule. **Requires daemon.**

| Task | Command | Notes |
|------|---------|-------|
| List all schedules | `mindx schedule list` | Shows agent, cron, enabled status, session binding |
| Add new schedule | `mindx schedule add --agent <name> --content "<prompt>" --cron "0 9 * * 1"` | **All three required** |
| Bind to session | `mindx schedule add ... --session-id <id>` | Links execution to a tracked session |
| Set project dir | `mindx schedule add ... --project-dir /path` | Working directory for the scheduled run |
| Disable on creation | `mindx schedule add ... --enabled false` | Create but don't activate yet |
| Delete schedule | `mindx schedule delete --id <schedule-id>` | Remove permanently |

### Cron Format

Standard 5-field cron (with optional seconds prefix):

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-7, 0 and 7 = Sun)
│ │ │ │ │
* * * * *
```

### Common Schedules

| Schedule | Cron | Use Case |
|----------|------|----------|
| Every weekday 9am | `0 9 * * 1-5` | Daily briefing |
| Every Friday 5pm | `0 17 * * 5` | Weekly report |
| First of month 10am | `0 10 1 * *` | Monthly summary |
| Every 6 hours | `0 */6 * * *` | Health check |
| Every Sunday noon | `0 12 * * 0` | Weekly cleanup |

### Examples
```bash
# Daily health check
mindx schedule add \
  --agent health-monitor \
  --content "Check system health. Report any issues." \
  --cron "0 8 * * *" \
  --enabled true

# Weekly report with session tracking
mindx schedule add \
  --agent weekly-reporter \
  --content "Generate weekly progress report. Report back via AgentTalk." \
  --cron "0 17 * * 5" \
  --session-id $WEEKLY_SESSION_ID

# After adding/editing schedules:
mindx restart   # Daemon reloads schedule definitions
```

## Token Usage Statistics

Track LLM API token consumption. **Requires daemon.**

| Task | Command | Notes |
|------|---------|-------|
| Overview | `mindx token overview` | This month vs last month comparison |
| Monthly breakdown | `mindx token monthly` | Current month's daily usage |
| Specific month | `mindx token monthly --year 2026 --month 6` | Historical data |
| By model | `mindx token by-model --model qwen-max` | Filter to one model |
| By model + month | `mindx token by-model --model qwen-max --year 2026 --month 6` | |
| Total cumulative | `mindx token total` | All-time usage |
| Per-session usage | `mindx token session --session-id <id>` | How much one conversation cost |

### Cost Monitoring Workflow
```bash
# Monthly review
mindx token overview
mindx token by-model --model gpt-4o
mindx token by-model --model qwen-max

# If a session seems expensive
mindx token session --session-id abc123

# Identify heavy users/sessions
mindx token monthly --year 2026 --month 6
```

## Translation

Translate text through the daemon. **Requires daemon.**

| Task | Command | Notes |
|------|---------|-------|
| Translate text | `mindx translate --text "Hello world" --lang zh` | Target language code |
| Translate long text | `mindx translate --text "$(cat file.txt)" --lang en` | Pipe content in |

### Supported Language Codes
Common codes: `en`, `zh`, `ja`, `ko`, `fr`, `de`, `es`, `pt`, `ru`, `ar`
(Actual support depends on configured model capabilities.)
