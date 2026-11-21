---
name: run-mcp
description: 调用外部 MCP 服务。当用户要「调用 MCP、连外部工具、用 MCP 取数、装一个 MCP 服务、看 MCP 有哪些工具、查 MCP 服务」时使用；用户提到 mindx mcp、mcp.json 或某服务只提供了 MCP 接口时主动使用。
metadata:
  name_zh: 运行连接器
  version: 1.2.0
  author: Ray
---

# 运行连接器（Run MCP）

一条命令完成「解析服务 → 按需启用 → 解析工具 → 调用 → 截断结果」。正常场景只调用一次 Bash，不需要先列清单。

## 何时使用

- 目标能力只以 MCP 形式提供（地图、企业工商、数据库、浏览器调试等），本机没有对应 CLI/REST
- 用户点名「MCP」「mindx mcp」「mcp.json」，或要查看/安装/调用已配置的 MCP 服务
- 已有条目要确认是否启用、有哪些工具、怎么传参

不适用：目标服务有 REST 或原生 CLI 时直接走它——同能力下 REST 少一层协议失败。

## 主路径（默认只走这一条）

```bash
python3 scripts/mcp.py run <服务> <工具> --args-json '{"k":"v"}'
```

- `<服务>` 写 slug、中文标题或关键词都可以：脚本先按精确名直调，失败后自动拿清单做模糊匹配（name > title > 描述），命中唯一就重试。
- `<工具>` 写精确名或关键词；省略工具则列出该服务工具（≤15 条）。
- 条目 disabled 时脚本**不替用户启用**：直接退出 8，并给出 `admin enable` 命令。只有用户明确要求启用时才执行该命令。
- 工具名不对时脚本自动 `tools` 匹配并重试；候选不唯一返回 exit 2。
- 结果超过 3000 字自动截断，全文落在 `full_result_path`，输出里带 `truncated: true`。

**禁止在执行前跑 `list`**。清单只在两种场景用：用户明确问「有哪些 MCP 服务」（用 `find`），或 `run` 返回 exit 2/5 需要候选时。把清单塞进上下文是纯 token 浪费，且对执行没有帮助。

## 查询与探索

| 目的 | 命令 |
|---|---|
| 找服务（不看完整清单） | `python3 scripts/mcp.py find <关键词>` |
| 看某服务工具（不调用） | `python3 scripts/mcp.py tools <服务>` |
| 健康检查 | `python3 scripts/mcp.py doctor` |
| 精确调用、不做任何解析/自动启用 | `python3 scripts/mcp.py call <slug> <工具> --args-json '{}'` |

`find` 只回 5 条 `name/title/enabled`；要更多结果再换关键词，禁止打印全量清单。

## 参数规则

- `--args-json`：数字、布尔、数组、嵌套对象必须走它；`--arg K=V` 只适合字符串，值一律按文本传（`3` 会变成 `"3"`）。
- `--timeout`：默认 60 秒；stdio 服务首次要下载依赖时给 120。
- 结果字段 `result` 可能是对象，也可能是被服务再包一层的 JSON 字符串；脚本已尽量解一次，取到的仍是字符串就再按 JSON 解析一次。

## 管理命令（用户明确要求安装/启停/测试时才用）

```bash
python3 scripts/mcp.py admin add <name> --command npx --args "-y,<pkg>" --enable
python3 scripts/mcp.py admin add <name> --url https://... --credential "KEY=VALUE" --enable
python3 scripts/mcp.py admin enable|disable <name>
python3 scripts/mcp.py admin test <name>
```

- `add` 默认 disabled；`run` 不会替用户启用，需要调用前必须由用户确认后执行 `admin enable`。
- 本地服务用 `--command`，远程用 `--url`，二者只能给一个；`--args` 逗号分隔，`--env` / `--header` / `--credential` 用 `K=V` 可重复。
- 缺 command/url、缺凭据、缺端点时问用户要，禁止猜参数。

## 退出码 → 动作

| 退出码 | 含义 | 动作 |
|---|---|---|
| 0 | 成功 | 直接用输出；`truncated` 时只读 full path 里需要的片段 |
| 2 | 服务或工具候选不唯一 | 从 `candidates` 里选一个准确 name，重跑一次 `run` |
| 3 | 找不到 mindx CLI | 提示安装或设置 `MINDX_CLI`，不要猜路径 |
| 4 | 智能主机不可达 | 启动 MindX 后重试 |
| 5 | 服务不存在 | 改关键词，或用 `admin add` 安装 |
| 6 | 工具不存在 | 输出里有该服务工具名，换一个重跑 |
| 7 | 调用失败 | 看 `error`：启动失败/凭据/网络/超时，按错误原文处理 |
| 8 | 条目未启用 | 向用户确认是否启用；同意后执行 `admin enable <name>` 再重跑 |

## 纪律边界

- 禁止手改 `~/.mindx/settings/mcp.json`；读写一律走脚本/CLI，daemon 独占写。
- 禁止替用户启用服务：disabled 条目只回报状态，`enable` 必须有用户明确指令。
- 凭据只经 `--credential` 进本机凭据库；禁止把密钥写进命令回显、日志或回复。
- 禁止把 MCP 工具 schema 塞进 Agent 配置（`allows_tools`）——已退役的装配注入路径，固定 token 开销且击穿 KV 缓存。
- 长结果只按需求读 `full_result_path` 的片段，禁止把全文再贴回上下文。

参考：`references/commands.md`（全部子命令、flag、输出形状）。
