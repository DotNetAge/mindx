---
updated: 2026-11-21
scope: mindx CLI mcp 命令族（cmd/mcp.go）+ run-mcp 的 mcp.py 封装
---

# mcp.py 与 mindx mcp 命令参考

`scripts/mcp.py` 是技能的默认入口：解析服务、自动启用、解析工具、截断结果。`mindx mcp` 是它背后的真实 CLI；只有管理类操作才直接转发。

## mcp.py 子命令

| 命令 | 用途 | 输出 |
|---|---|---|
| `run <服务> [工具] [--args-json ...] [--arg K=V] [--timeout N]` | 主路径：解析并调用；省略工具时列工具 | 成功 `{server, tool, result}`；超长附 `truncated`/`full_result_path`；未启用 exit 8 |
| `find <关键词>` | 按 name/title/描述找服务 | `{candidates:[{name,title,enabled}], more}`，最多 5 条 |
| `tools <服务> [--timeout N]` | 列工具，不调用 | `{server, tools:[{name,description}]}`，最多 20 条 |
| `call <slug> <工具> [--args-json] [--arg] [--timeout]` | 精确调用，不解析、不自动启用 | `{server, tool, result}` |
| `admin <list/enable/disable/test/add ...>` | 原样转发 `mindx mcp` 子命令 | CLI 原始输出 |
| `doctor` | 打印 CLI 路径并探测主机 | `{cli, daemon, detail}` |

退出码：0 成功 / 2 候选不唯一 / 3 无 CLI / 4 主机不可达 / 5 服务不存在 / 6 工具不存在 / 7 调用失败 / 8 条目未启用（需用户确认后 enable）。

## run 的解析顺序

1. 直接 `call <输入> <工具>`。
2. 报「不在 mcp.json」→ 用 `list` 做模糊匹配（name 精确 100 / name 包含 80 / title 包含 70 / 描述包含 40），唯一命中则换 name 重试；多个返回 exit 2。
3. 报「未启用」→ 直接 exit 8，输出里给出 `admin enable` 命令；脚本不替用户启用。
4. 工具报未知 → 拉 `tools` 匹配，唯一命中则换工具名重试；多个 exit 2，全无 exit 6。
5. 最多重试 3 轮，避免死循环。

服务清单缓存 5 分钟（`~/.cache/mindx-run-mcp/catalog.json`），`--refresh` 强制重读；`find`/`tools` 走缓存，`run` 报错时强制刷新。

## 结果截断

- `result` 若可解析为 JSON 则原样输出对象；否则为字符串。
- 序列化后超过 3000 字符：输出前 3000 字 + `…[截断]`，全文写入 `~/.cache/mindx-run-mcp/result-<ts>.json`，字段 `full_result_path`。

## mindx mcp 原始命令（admin 转发目标）

| 命令 | 用途 | 关键 flag |
|---|---|---|
| `list` | 全量清单（123 条，禁止整段回显给上下文） | `--json` |
| `add <name>` | 安装，默认 disabled | `--command` / `--url`（二选一）、`--args a,b`、`--env K=V`、`--header K=V`、`--credential K=V`、`--title`、`--idle-ttl`、`--enable` |
| `tools <server>` | 列工具（disabled 也能列） | `--json` |
| `call <server> <tool>` | 调用已启用服务 | `--arg K=V`、`--args-json`、`--timeout`（默认 60）、`--json` |
| `enable` / `disable <server>` | 启停 | 无 |
| `test <server>` | 连通性探测 | 无；失败退出码非 0 |

## 输出形状

```json
// list --json 单条
{ "name": "memory", "type": "stdio", "command": "npx",
  "args": ["-y","@modelcontextprotocol/server-memory@latest"],
  "enabled": false, "idle_ttl_secs": 300, "title": "Memory 持久记忆" }

// tools --json
[{ "name": "read_graph", "description": "Read the entire knowledge graph",
   "inputSchema": { "type": "object", "properties": {} } }]

// call --json
{ "server": "memory", "tool": "read_graph", "result": "{\"entities\":[],\"relations\":[]}" }
```

## 运行机制

- 条目落盘 `~/.mindx/settings/mcp.json`，由智能主机独占写入；手工编辑可能被下一次全量保存覆盖。
- 启用的服务进程由智能主机连接池按需拉起，空闲超过 `idle-ttl` 回收；崩溃后下次调用自动重启。
- 远程托管端点 `initialize`/`tools/list` 免鉴权，`tools/call` 校验 `X-api-key`；探索阶段没报 401 不代表凭据完整。
