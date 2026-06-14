---
name: mindx-cli
description: >
  Complete reference for the mindx CLI — the command-line interface for
  MindX AI Agent. Use when the user asks to manage the daemon, check status,
  view logs, run diagnostics, install/upgrade, configure providers/models/agents,
  query long-term memory, or otherwise interact with the MindX system via CLI.
  Do NOT trigger for general AI agent usage questions unrelated to MindX.
allowed-tools:
  - Bash(mindx *)
  - Bash(~/mindx *)
  - Bash(/tmp/mindx *)
metadata:
  name_zh: MindX 指令集
  name_zh-tw: MindX 指令集
  description_zh: mindx CLI 的完整指令参考，包括服务管理、配置管理、诊断升级和数据查询
  description_zh-tw: mindx CLI 的完整指令參考，包括服務管理、設定管理、診斷升級和資料查詢
---

# MindX CLI Reference

mindx is the command-line interface for MindX AI Agent. Run `mindx --help` or
`mindx <command> --help` for full option details.

## Prerequisites

MindX must be installed. Check with:

```bash
mindx version
mindx status
```

## Command Map by Use Case

### Service Lifecycle

Manage the background daemon service.

| Task | Command | When |
|------|---------|------|
| Start daemon service | `mindx start` | Daemon installed but not running |
| Stop daemon service | `mindx stop` | Daemon is running and needs shut down |
| Restart daemon service | `mindx restart` | After upgrade, config change, or troubleshooting |
| Check daemon status | `mindx status` | Any time — shows all system component health |
| View daemon logs | `mindx logs` | Troubleshooting, monitoring, investigating errors |
| Tail daemon logs | `mindx logs -f` | Real-time log monitoring |

```bash
# Typical workflow
mindx status
mindx logs -n 20
mindx restart
```

### System Management

Install, upgrade, diagnose, and manage the application.

| Task | Command | When |
|------|---------|------|
| Install to system | `mindx install` | First-time setup |
| Install without daemon | `mindx install --no-daemon` | Server or container where daemon is managed separately |
| Check for updates | `mindx upgrade --check` | Before deciding to upgrade |
| Upgrade to latest | `mindx upgrade` | New version available |
| Diagnose issues | `mindx doctor` | Something is broken or misconfigured |
| Auto-fix issues | `mindx doctor --fix` | Doctor detects fixable problems |
| Open WebUI | `mindx web` | User wants the browser UI |
| Show version | `mindx version` | Need to report version or verify build |
| Manage macOS .app | `mindx app create` | macOS users who want Finder/Dock integration |

```bash
# Upgrade workflow
mindx upgrade --check
mindx upgrade
mindx restart           # restart daemon after upgrade
```

```bash
# Troubleshooting workflow
mindx version
mindx status
mindx doctor --fix
mindx logs -n 50
```

### Configuration: Providers

Manage LLM API providers (OpenAI, DashScope, Ollama, etc.).

| Task | Command | When |
|------|---------|------|
| List providers | `mindx provider list` | See what's configured |
| Add/update provider | `mindx provider add --name <name> --base-url <url> --api-key <env_var>` | Setting up a new API provider |
| Add local provider | `mindx provider add --name ollama --base-url http://localhost:11434 --local` | Local models like Ollama |
| Set API key | `mindx provider setkey <provider> <api-key>` | Store actual API key in system credential store |
| Remove provider | `mindx provider rm <name>` | Clean up unused providers |

```bash
# Typical provider setup
mindx provider add --name dashscope --base-url https://dashscope.aliyuncs.com/compatible-mode/v1 --api-key DASHSCOPE_API_KEY
mindx provider setkey dashscope sk-xxxxxxxxxxxx
mindx provider list
```

### Configuration: Models

Manage LLM models tied to providers.

| Task | Command | When |
|------|---------|------|
| List models | `mindx model list` | See available models |
| Add model | `mindx model add --name <name> --provider <provider> --context-length <n>` | Add a new model |
| Set default model | `mindx model set <model-name>` | Change default for new sessions |
| Remove model | `mindx model rm <name>` | Remove an unused model |

```bash
# Typical model setup
mindx model add --name qwen-max --provider dashscope --context-length 32000 --max-tokens 4096 --func-calling
mindx model set qwen-max
mindx model list
```

### Configuration: Agents

Manage AI agent profiles.

| Task | Command | When |
|------|---------|------|
| List agents | `mindx agent list` | See configured agents |
| Add/update agent | `mindx agent add <name> --role <role> --model <model>` | Create a customized agent |
| Remove agent | `mindx agent rm <name>` | Delete an agent |

```bash
mindx agent list
mindx agent add helper --role "Assistant" --model qwen-max --description "General-purpose helper"
```

### Data: Long-Term Memory

Search the semantic memory store.

| Task | Command | When |
|------|---------|------|
| Search memory | `mindx query <terms>` | User asks about past discussions or decisions |
| Limit results | `mindx query -n 20 <terms>` | Control result count |
| Filter by score | `mindx query --min-score 0.5 <terms>` | Only high-relevance results |

```bash
mindx query "architecture decisions"
mindx query --min-score 0.7 "API design" -n 5
```

### Development

Run the daemon directly (for development or containers).

| Task | Command | When |
|------|---------|------|
| Run daemon directly | `mindx daemon` | Development, containers, direct process management |
| Specify port | `mindx daemon --port :1314` | Custom WebSocket port |

> **Important**: `mindx daemon` is the actual server process. In production, use
> `mindx start` to launch it via the system service manager (launchctl/systemd/schtasks).
> Use `mindx daemon` only for development or containerized environments.

### TUI Client

Run the terminal user interface (default entry point).

```bash
mindx                         # Start the interactive TUI chat
```

The TUI is the primary user interface. It provides:
- Chat sessions with AI agents
- Provider and model management (in-app)
- Daemon connection via WebSocket RPC
- Agent switching and skill management

### Skills

Inspect installed skills.

| Task | Command | When |
|------|---------|------|
| List installed skills | `mindx skill list` | See what skills are available |
| View skill details | `mindx skill get <name>` | Check a skill's description and allowed tools |

```bash
mindx skill list
mindx skill get batch
```

### Security: Permission Rules

View tool access control rules.

| Task | Command | When |
|------|---------|------|
| List permission rules | `mindx rule list` | Review allow/deny/ask rules |
| Get rule details | `mindx rule get <tool-name>` | Inspect a specific tool's permission |

```bash
mindx rule list
mindx rule get fs.write
```

### Automation: Scheduled Tasks

Manage cron-scheduled agent tasks (requires daemon restart after changes).

| Task | Command | When |
|------|---------|------|
| List scheduled tasks | `mindx schedule list` | See all scheduled tasks |
| Add scheduled task | `mindx schedule add --agent <name> <content> <cron>` | Set up recurring agent runs |
| Delete scheduled task | `mindx schedule del <id>` | Remove a schedule |

```bash
mindx schedule list
mindx schedule add --agent notes "Daily summary" "0 18 * * 1-5"
mindx restart
```

## Parallel Operations

Multiple independent mindx commands can be run in parallel:

```bash
mindx status &
mindx logs -n 10 &
wait
```
