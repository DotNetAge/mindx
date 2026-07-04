---
name: mindx-cli
description: >
  Complete command reference for the mindx CLI — the control plane for MindX AI Agent.
  Covers service lifecycle, AI capability configuration (providers/models/agents/skills/rules),
  data layer operations (memory, knowledge base, graph, kv store, sessions),
  automation (scheduling, token stats), and file system / utility operations.
  Use when the user needs to manage, diagnose, configure, or query
  any aspect of the MindX system via CLI. This is the sysops agent's primary reference.
allowed-tools: Bash(mindx *) Bash(~/mindx *) Bash(/tmp/mindx *)
metadata:
  name_zh: MindX 指令集
  name_zh-tw: MindX 指令集
  description_zh: mindx CLI 完整指令参考——服务管理、AI 能力配置、数据层操作、自动化和文件系统
  description_zh-tw: mindx CLI 完整指令參考——服務管理、AI 能力配置、數據層操作、自動化和檔案系統
---

# MindX CLI Reference

mindx is the command-line interface for MindX AI Agent.
Run `mindx --help` or `mindx <command> --help` for full option details.

## Trigger Decision

Use this skill when:
- User asks to manage, diagnose, or configure any part of the MindX system
- User needs to check status, view logs, run health checks
- User wants to add/update providers, models, agents, skills, or rules
- User needs to query memory, graph, sessions, or token usage
- User needs to set up scheduled tasks or troubleshoot daemon issues

**Do NOT use** for general AI agent conversations unrelated to MindX administration.

## Command Map — Quick Index

Detailed references are in `references/`. Use this table to find which file covers your need.

| Group          | What It Manages                                                                 | Reference File                                    | Daemon Required?                |
| -------------- | ------------------------------------------------------------------------------- | ------------------------------------------------- | ------------------------------- |
| **Service**    | Install, upgrade, start/stop/restart, logs, doctor, web UI, app bundle, shell completion | [ref-service.md](references/ref-service.md)       | Partial                         |
| **Config: AI** | Providers, models, agents, skills, permission rules                             | [ref-config-ai.md](references/ref-config-ai.md)   | Partial                         |
| **Memory**     | Long-term memory (RAG), knowledge base, key-value store, offline query          | [ref-memory.md](references/ref-memory.md)         | Yes (memory/kb/kv) / No (query) |
| **Graph**      | Knowledge graph (Cypher CRUD, nodes, edges)                                     | [ref-graph.md](references/ref-graph.md)           | Yes                             |
| **Session**    | Agent session lifecycle (create/list/get/delete/meta/confirm/rollback)          | [ref-session.md](references/ref-session.md)       | Yes                             |
| **Automation** | Scheduled tasks, token usage statistics, translation                            | [ref-automation.md](references/ref-automation.md) | Yes                             |
| **Ops**        | File system ops, file watcher, daemon logs, user config, entity tags, utilities | [ref-ops.md](references/ref-ops.md)               | Partial                         |

## Quick Diagnostic Workflow

When something seems wrong, follow this order:

```bash
# 1. Is it running?
mindx status

# 2. What version?
mindx version

# 3. Any obvious issues?
mindx doctor

# 4. Check recent logs
mindx logs -n 30

# 5. If daemon is unhealthy
mindx restart

# 5b. Or if only agent/skill configs changed (no full restart needed)
mindx reload agents    # after editing ~/.mindx/agents/*.md
mindx reload skills    # after editing skills/*/SKILL.md

# 6. If still broken, check full logs
mindx log read --limit 50 --stream error
```

## Prerequisites

```bash
# Verify installation
mindx version
mindx status
```

Both should succeed before using any other commands.

## Offline vs Online Commands

Some commands work without the daemon running; others require it.

**Offline-safe** (work anytime):
`install`, `uninstall`, `upgrade`, `version`, `doctor`, `start`, `stop`, `restart`, `status`,
`logs`, `query`, `app`, `utils`, `completion`, `provider list/add/rm/setkey`, `model list/add/rm/set`,
`agent list/add/rm`, `skill list/get/add/validate/eval`

> Note: `provider list`, `model list`, `agent list`, and `skill list` use the daemon only when passed `--json`.

**Daemon-required** (need `mindx start` first):
All `memory`, `kb`, `graph`, `session`, `schedule`, `kv`, `fs`, `fw`, `token`, `rule`,
`log read/clear/count`, `translate`, `entity-tags`, `user config`,
`agent get/score/update`, `provider create/update/delete`, `model switch`,
`reload agents|skills`, `web`
