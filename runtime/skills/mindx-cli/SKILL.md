---
name: mindx-cli
description: >
  mindx CLI 完整命令参考——MindX AI Agent 的控制面板。
  涵盖服务生命周期、AI 能力配置（提供商/模型/智能体/技能/规则）、
  数据层操作（记忆、知识库、图、键值存储、会话）、
  自动化（定时任务、Token 统计）以及文件系统/工具操作。
  当用户需要通过 CLI 管理、诊断、配置或查询
  MindX 系统的任何方面时使用。这是运维智能体的主要参考。
allowed-tools: Bash(mindx *) Bash(~/mindx *) Bash(/tmp/mindx *)
metadata:
  name_zh: MindX 指令集
  name_zh-tw: MindX 指令集
  description_zh: mindx CLI 完整指令参考——服务管理、AI 能力配置、数据层操作、自动化和文件系统
  description_zh-tw: mindx CLI 完整指令參考——服務管理、AI 能力配置、數據層操作、自動化和檔案系統
---

# MindX CLI 参考

mindx 是 MindX AI Agent 的命令行界面。
运行 `mindx --help` 或 `mindx <command> --help` 获取完整选项详情。

## 触发条件

遇到以下情况时使用此技能：
- 用户要求管理、诊断或配置 MindX 系统的任何部分
- 用户需要检查状态、查看日志、运行健康检查
- 用户想要添加/更新提供商、模型、智能体、技能或规则
- 用户需要查询记忆、图、会话或 Token 使用情况
- 用户需要设置定时任务或排查守护进程问题

**不要**在与 MindX 管理无关的一般 AI 智能体对话中使用。

## 命令地图 — 快速索引

详细参考在 `references/` 中。用此表找到对应文件。

| 分组          | 管理内容                                                                 | 参考文件                                    | 需要守护进程?                |
| -------------- | ------------------------------------------------------------------------------- | ------------------------------------------------- | ------------------------------- |
| **服务**    | 安装、升级、启动/停止/重启、日志、诊断、Web UI、应用包、Shell 补全 | [ref-service.md](references/ref-service.md)       | 部分                         |
| **AI 配置** | 提供商、模型、智能体、技能、权限规则                             | [ref-config-ai.md](references/ref-config-ai.md)   | 部分                         |
| **记忆**     | 长期记忆（RAG）、知识库、键值存储、离线查询          | [ref-memory.md](references/ref-memory.md)         | 是（memory/kb/kv）/ 否（query） |
| **图**      | 知识图谱（Cypher CRUD、节点、边）                                     | [ref-graph.md](references/ref-graph.md)           | 是                             |
| **会话**    | 智能体会话生命周期（创建/列表/获取/删除/元数据/确认/回滚）          | [ref-session.md](references/ref-session.md)       | 是                             |
| **自动化** | 定时任务、Token 使用统计、翻译                            | [ref-automation.md](references/ref-automation.md) | 是                             |
| **运维**        | 文件系统操作、文件监控、守护进程日志、用户配置、实体标签、工具 | [ref-ops.md](references/ref-ops.md)               | 部分                         |

## 快速诊断流程

遇到问题时，按以下顺序操作：

```bash
# 1. 是否在运行？
mindx status

# 2. 什么版本？
mindx version

# 3. 有明显问题吗？
mindx doctor

# 4. 检查最近日志
mindx logs -n 30

# 5. 如果守护进程不健康
mindx restart

# 5b. 或者如果只是智能体/技能配置变更（无需完全重启）
mindx reload agents    # 编辑 ~/.mindx/agents/*.md 后
mindx reload skills    # 编辑 skills/*/SKILL.md 后

# 6. 如果仍有问题，查看完整日志
mindx log read --limit 50 --stream error
```

## 前置条件

```bash
# 验证安装
mindx version
mindx status
```

两个命令都成功执行后，再使用其他命令。

## 离线与在线命令

部分命令无需守护进程即可工作，其余则需要。

**离线安全**（随时可用）：
`install`、`uninstall`、`upgrade`、`version`、`doctor`、`start`、`stop`、`restart`、`status`、
`logs`、`query`、`app`、`utils`、`completion`、`provider list/add/rm/setkey`、`model list/add/rm/set`、
`agent list/add/rm`、`skill list/get/add/validate/eval`

> 注意：`provider list`、`model list`、`agent list` 和 `skill list` 仅在传入 `--json` 时使用守护进程。

**需要守护进程**（需先 `mindx start`）：
所有 `memory`、`kb`、`graph`、`session`、`schedule`、`kv`、`fs`、`fw`、`token`、`rule`、
`log read/clear/count`、`translate`、`entity-tags`、`user config`、
`agent get/score/update`、`provider create/update/delete`、`model switch`、
`reload agents|skills`、`web`
