# 重构计划：internal/svc 和 pkg/rpc

## 背景

两个包之间参数类型大量重复，文件命名臃肿，客户端返回值无类型安全。

**现状数据：**
- `pkg/rpc`：20 个文件，933 行，38 个参数结构体，62 个客户端方法，全部返回 `json.RawMessage`
- `internal/svc`：21 个 handler 文件 + daemon.go(1869 行) + 若干辅助文件，共 9061 行
- 两个包之间存在约 30 对完全相同的参数结构体（一个导出、一个未导出）
- handler 注册文件 73 行逐行 `gw.RegisterMethod` 调用

---

## 阶段一：消除跨包类型重复

**目标：** `internal/svc` 的 handler 不再各自定义 `xxxParams`，统一复用 `pkg/rpc.XXXParams`。

### 任务 1.1：文件重命名（无风险，先做以免后续混淆）

| 旧文件名 | 新文件名 |
|---|---|
| `daemon_rpc_session.go` | `handler_session.go` |
| `daemon_rpc_agent.go` | `handler_agent.go` |
| `daemon_rpc_model.go` | `handler_model.go` |
| `daemon_rpc_memory.go` | `handler_memory.go` |
| `daemon_rpc_kb.go` | `handler_kb.go` |
| `daemon_rpc_interact.go` | `handler_interact.go` |
| `daemon_rpc_kvstore.go` | `handler_kvstore.go` |
| `daemon_rpc_graph.go` | `handler_graph.go` |
| `daemon_rpc_fs.go` | `handler_fs.go` |
| `daemon_rpc_schedule.go` | `handler_schedule.go` |
| `daemon_rpc_rule.go` | `handler_rule.go` |
| `daemon_rpc_entity_tags.go` | `handler_entity_tags.go` |
| `daemon_rpc_translate.go` | `handler_translate.go` |
| `daemon_rpc_optimize.go` | `handler_optimize.go` |
| `daemon_rpc_server.go` | `handler_server.go` |
| `daemon_rpc_update.go` | `handler_update.go` |
| `daemon_rpc_token_usage.go` | `handler_token_usage.go` |
| `daemon_rpc_terminal.go` | `handler_terminal.go` |
| `daemon_rpc_user.go` | `handler_user.go` |
| `daemon_rpc_log.go` | `handler_log.go` |
| `daemon_rpc_i18n.go` | `handler_i18n.go` |
| `daemon_rpc_skill.go` | `handler_skill.go` |
| `daemon_rpc_registry.go` | `handler_registry.go` |
| `daemon_rpc_test.go` | `handler_test.go` |

验证：`go build ./...` + 全部测试通过。

### 任务 1.2：删除 session 域重复

删除以下 svc 私有结构体，改用 `rpc.SessionCreateParams` 等：
- `sessionCreateParams` → 不再需要（`session.create` handler 直接用 `rpc.SessionCreateParams`）
- `sessionGetParams` → 改用 `rpc.SessionGetParams`
- `sessionDeleteParams` → 改用 `rpc.SessionDeleteParams`
- `sessionListParams` → 改用 `rpc.SessionListParams`
- `sessionFileActionParams` → 改用 `rpc.SessionFileActionParams`

验证：`go build ./...`

### 任务 1.3：删除 agent 域重复

- `agentGetParams` → 改用 `rpc.AgentGetParams`
- `agentScoreParams` → 改用 `rpc.AgentScoreParams`
- `agentCreateParams` → 改用 `rpc.AgentCreateParams`
- `agentUpdateParams` → 改用 `rpc.AgentUpdateParams`

验证：`go build ./...`

### 任务 1.4：删除 model 和 provider 域重复

- `modelGetParams` → 改用 `rpc.ModelGetParams`
- `modelSwitchParams` → 改用 `rpc.ModelSwitchParams`
- `modelCreateParams` → 改用 `rpc.ModelCreateParams`
- `modelUpdateParams` → 改用 `rpc.ModelUpdateParams`
- `providerCreateParams` → 改用 `rpc.ProviderCreateParams`
- `providerUpdateParams` → 改用 `rpc.ProviderUpdateParams`
- `providerDeleteParams` → 改用 `rpc.ProviderDeleteParams`

验证：`go build ./...`

### 任务 1.5：删除 memory 域重复

- `memoryQueryParams` → 改用 `rpc.MemoryQueryParams`
- `memoryStoreParams` → 改用 `rpc.MemoryStoreParams`
- `memoryDeleteParams` → 改用 `rpc.MemoryDeleteParams`
- `memoryChunksParams` → 改用 `rpc.MemoryChunksParams`
- `memoryGetChunksParams` → 改用 `rpc.MemoryGetChunksParams`
- `filewatchRemoveParams` → 先在 rpc 中补定义（当前 rpc 的 fw.go 未导出此类型）
- `filewatchRetryFailedParams` → 同上，补定义
- `filewatchIgnoreFailedParams` → 同上，补定义

验证：`go build ./...`

### 任务 1.6：删除其他域重复（kb、graph、kvstore、rule、schedule、translate、token、entity_tags、fs、log、i18n、skill）

原则：将 svc 中已命名定义的 `xxxParams` struct 删除，统一替换为 `rpc.XXXParams`。对于 svc 中未对应 rpc 类型的情况（如 filewatch.remove/retry/ignore 的参数），先在 `pkg/rpc` 补定义。

补充到 rpc 的类型清单：
- `FilewatchRemoveParams` (Dir string)
- `FilewatchRetryFailedParams` (Dir string, Files []string)
- `FilewatchIgnoreFailedParams` (Dir string, Files []string)
- `OptimizeParams` (Text string), `OptimizeResult` (Text string)
- `TranslateResult` (Text string, Cached bool)
- `I18nSwitchParams` (Lang string)
- `LogReadParams` (Offset, Limit int, Stream string) — 已存在
- 检查 handler_memory.go 中的 `memoryChunksResult`、`chunkItem`、`chunkMetaItem` 等是否应导出到 rpc

验证：`go build ./...`

### 任务 1.7：检查无对应 rpc 类型的 handler 参数

以下 handler 使用匿名 struct 做参数，没有命名 xxxParams：
- `handleTerminalStart/Input/Resize/Kill` → 保持原样，等阶段二再处理
- `handleAskUserReply` → rpc 未定义 `AskUserReplyParams`，是否需要补？看前端调用方式
- `handlePermissionReply` → 同上

决定：阶段一不处理这些，只处理已命名 xxxParams 的删除。

验证：全部 handler 文件无未导出命名 `xxxParams` 结构体（除 `handler_terminal.go` 外）。

### 任务 1.8：最终验证

- `go build ./...`
- `go test ./...`
- 确认 svc 目录中不再有 `daemon_rpc_*` 文件
- 确认所有 handler 的 param 解包使用了 `rpc.XXXParams`

---

## 阶段二：类型安全的返回值

**目标：** `pkg/rpc` 客户端方法返回具体 `Result` 类型而非 `json.RawMessage`。

### 任务 2.1：定义结果类型

在 `pkg/rpc` 中为每个返回有结构的方法添加 result 结构体：

```
session.go:     SessionCreateResult, SessionGetResult, SessionListResult
agent.go:       AgentListResult
kb.go:          KBSearchResult, KBChunksResult, KBStatsResult, KBFileStatesResult
memory.go:      MemoryQueryResult, MemoryChunksResult, MemoryCountResult
graph.go:       GraphQueryResult, GraphExecResult
fs.go:          FSListResult, FSReadResult, FSHomeResult
token.go:       TokenOverviewResult, TokenMonthlyResult, TokenByModelResult, TokenTotalResult, TokenSessionResult
translate.go:   TranslateResult
optimize.go:    OptimizeResult
```

### 任务 2.2：修改客户端方法签名

```diff
-func (c *Client) SessionCreate(agent, dir string) (json.RawMessage, error)
+func (c *Client) SessionCreate(agent, dir string) (*SessionCreateResult, error)
```

方法体内改为 `json.Unmarshal(data, &result)`。

### 任务 2.3：查找调用方并适配

搜索 `pkg/rpc` 客户端的所有调用点。

---

## 阶段三：注册声明表

**目标：** 减少 `handler_registry.go` 中 73 行逐行注册的重复。

### 任务 3.1：提取 handlers map

在 `handler_registry.go` 中添加：

```go
func (r *RPCHandlerRegistry) handlers() map[string]handlerFunc {
    return map[string]handlerFunc{
        "session.create": r.daemon.handleSessionCreate,
        "session.delete": r.daemon.handleSessionDelete,
        // ... 所有 73 项
    }
}
```

### 任务 3.2：简化 RegisterAll

```go
func (r *RPCHandlerRegistry) RegisterAll(gw *gateway.Server) {
    for method, handler := range r.handlers() {
        gw.RegisterMethod(method, handler)
    }
}
```

验证：`go build ./...`
