# Daemon JSON-RPC 协议

MindX Daemon 对外暴露 WebSocket 服务，通过 JSON-RPC 2.0 协议与客户端（TUI、WebUI、MacUI 等）通信。客户端通过 JSON-RPC 请求执行命令、发送消息，Daemon 通过 JSON-RPC 通知推送 Agent 事件流。

## 传输层

| 项目 | 说明 |
|---|---|
| 协议 | WebSocket（ws://） |
| 默认地址 | `ws://localhost:1314/ws` |
| 数据格式 | JSON-RPC 2.0，以 `\n` 分隔多消息（单次 WebSocket 帧可包含多条 JSON-RPC 消息） |
| 心跳 | 服务端每隔 54s 发送 Ping，客户端需回复 Pong，读超时 60s |

## 消息路由

客户端发送的消息在服务端按类型分发：

| JSON-RPC 类型 | 字段 | 路由目标 |
|---|---|---|
| Request（有 ID） | `method` | 注册的 MethodHandler 或 CommandHandler |
| Notification（无 ID） | `method: "user.message"` | defaultHandler（解析 `@agent text`，路由到 Agent） |
| Notification（无 ID） | 其他 method | 静默丢弃（暂未支持客户端间转发） |

## 客户端 → 服务端

### 发送消息

向 Agent 发送一条聊天消息。Daemon 收到后解析 `@agent_name session_id content` 格式，自动路由到对应的 Agent 实例（不存在则创建）。

- Type: Notification
- Method: `user.message`
- Params:

```json
{
  "text": "@developer 帮我看看这个bug"
}
```

如果指定 session：

```json
{
  "text": "@developer sess_abc123 上次那个问题还没解决"
}
```

### 执行命令

执行 Daemon 内置的斜杠命令。服务端处理完成后返回 JSON-RPC Response，同时可能推送额外的 Notification（如表格式数据）。

- Type: Request
- Method: `command.<name>`
- Response: 命令执行结果（JSON-RPC Response result 字段）

```json
// request
{"jsonrpc":"2.0","id":"1","method":"command.agents","params":{"args":""}}

// JSON-RPC response (result 字段 — 结构化数据供编程解析)
{"jsonrpc":"2.0","id":"1","result":[{"label":"developer","value":"developer","role":"软件工程师","desc":"负责编码","model":"gpt-4","active":"true"}]}

// 额外推送的 Notification (用于 TUI 渲染)
{"jsonrpc":"2.0","method":"table","params":{"type":"table","session_id":"","title":"Available Agents","data":{"headers":["Name","Role","Description"],"rows":[["developer","软件工程师","负责编码"]]}}}
```

```json
// request
{"jsonrpc":"2.0","id":"2","method":"command.job-add","params":{"args":"@writer sess_abc123 每日文章 expr=\"0 0 9 * * 1\" dir=\"/path/to/project\""}}

// response (成功)
{"jsonrpc":"2.0","id":"2","result":"✅ 定时消息已创建:\n  ID: abc12345\n  目标: @writer\n  Session: sess_abc123\n  项目目录: /path/to/project\n  内容: 每日文章\n  调度: 0 0 9 * * 1"}

// response (失败)
{"jsonrpc":"2.0","id":"2","error":{"code":-32603,"message":"无效的 cron 表达式: ..."}}
```

### 获取命令列表

- Type: Request
- Method: `command.list`

```json
// request
{"jsonrpc":"2.0","id":"99","method":"command.list","params":null}

// response
{"jsonrpc":"2.0","id":"99","result":[
  {"name":"help","description":"显示所有可用命令","category":"system","scope":"remote","example":"","params":""},
  {"name":"agents","description":"显示智能体列表","category":"agent","scope":"remote","example":"/agents","params":""},
  {"name":"job-add","description":"添加计划任务","category":"system","scope":"remote","example":"@writer ...","params":"@<agent-name> <session_id|new> <content> expr=\"<cron表达式>\""}
]}
```

### 命令注册表

所有命令通过 `RegisterCommand()` 注册。当前已注册的命令：

| 命令 | 类别 | Scope | 说明 |
|---|---|---|---|
| `help` | system | remote | 显示所有可用命令 |
| `about` | system | remote | 关于 MindX |
| `init` | system | remote | 初始化会话 |
| `clear` | system | both | 清理当前所有上下文 |
| `agents` | agent | remote | 显示智能体列表 |
| `models` | agent | remote | 列出所有可用模型 |
| `skills` | agent | remote | 列出所有可用技能 |
| `job-add` | system | remote | 添加计划任务 |
| `job-list` | system | remote | 列出所有计划任务 |
| `job-del` | system | remote | 删除计划任务 |

### 数据管理 RPC 方法

以下方法通过 `RegisterMethod()` 注册为原生 JSON-RPC method handler，与 Slash Command 不同，它们直接返回结构化 JSON 数据，不经过 Command 解析层。

#### Session 会话管理

| Method | Params | 返回值 | 说明 |
|---|---|---|---|
| `session.list` | `{ "agent": "可选过滤" }` | `[]SessionInfo` | 列出所有会话，支持按 agent 过滤 |
| `session.get` | `{ "session_id": "sess_xxx" }` | `{ messages: [], meta? }` | 获取会话完整消息 + 元数据 |
| `session.meta` | `{ "session_id": "sess_xxx" }` | `SessionMeta` | 仅获取会话元数据 |

**session.list** — 列出所有会话：

```json
// request
{"jsonrpc":"2.0","id":1,"method":"session.list","params":{}}

// response
{"jsonrpc":"2.0","id":1,"result":[
  {"session_id":"sess_abc12345","agent_name":"coder",
   "project_dir":"/Users/ray/workspaces/my-project",
   "messages":[],"last_activity_at":"2026-05-22T10:30:00Z","created_at":"2026-05-22T10:00:00Z"}
]}
```

按 agent 过滤：

```json
{"jsonrpc":"2.0","id":2,"method":"session.list","params":{"agent":"coder"}}
```

> 结果按 `last_activity_at` **降序**排列。空列表返回 `[]`（非 null）。

**session.get** — 获取会话详情：

```json
{"jsonrpc":"2.0","id":3,"method":"session.get","params":{"session_id":"sess_abc12345"}}

// response
{"jsonrpc":"2.0","id":3,"result":{
  "session_id":"sess_abc12345",
  "messages":[
    {"role":"user","content":"帮我写个排序算法","timestamp":1716376200000},
    {"role":"assistant","content":"好的，这是一个 Go 实现的快速排序...","timestamp":1716376201500}
  ],
  "meta":{
    "session_id":"sess_abc12345","agent_name":"coder",
    "created_at":"2026-05-22T10:00:00Z","message_count":15,
    "last_activity_at":"2026-05-22T10:30:00Z"
  }
}}
```

> - `messages`: 完整对话历史（`[]Message`）
> - `meta`: 仅当 session 存在 `meta.json` 时返回；不存在时该字段缺失
> - 不存在的 session 返回空 `messages: []` 而非报错

**session.meta** — 仅获取元数据（轻量级）：

```json
{"jsonrpc":"2.0","id":4,"method":"session.meta","params":{"session_id":"sess_abc12345"}}

// response
{"jsonrpc":"2.0","id":4,"result":{
  "session_id":"sess_abc12345","agent_name":"coder",
  "created_at":"2026-05-22T10:00:00Z","message_count":15,
  "last_activity_at":"2026-05-22T10:30:00Z"
}}
```

> 与 `session.get` 的区别：**只返回元数据**，不加载消息历史。

---

#### Memory 记忆管理

> ⚠️ 所有 Memory 方法依赖 **Embedder（向量模型）** 已配置。未配置时返回 `-32603` 错误。

| Method | Params | 返回值 | 说明 |
|---|---|---|---|
| `memory.query` | `{ "query", "limit?", "type?", "min_score?" }` | `[]MemoryRecord` | 语义检索记忆 |
| `memory.store` | `{ "content", "title?", "tags?", "type?" }` | `{ "id" }` | 写入新记忆 |
| `memory.delete` | `{ "id" }` | `{ "status", "deleted_id" }` | 按 ID 删除记忆 |

**memory.query** — 语义检索：

```json
// 基础查询
{"jsonrpc":"2.0","id":5,"method":"memory.query","params":{"query":"Go并发模式"}}

// 带过滤条件
{"jsonrpc":"2.0","id":6,"method":"memory.query","params":{
  "query":"数据库连接池","limit":5,"type":"longterm","min_score":0.7
}}

// response
{"jsonrpc":"2.0","id":5,"result":[
  {"id":"mem_1716376200000","type":1,
   "title":"Go 数据库连接池最佳实践","content":"使用 database/sql 包实现连接池...",
   "tags":["go","database","pool"],"score":0.892,
   "created_at":"2026-05-22T08:30:00Z"},
  {"id":"mem_1716375600000","type":1,
   "title":"sync.Pool 使用指南","content":"Go 的 sync.Pool 用于对象复用...",
   "tags":["go","performance"],"score":0.754,
   "created_at":"2026-05-22T08:00:00Z"}
]}
```

> - 结果按 **score 降序**排列；`type`: `0`=Session, `1`=LongTerm
> - 无匹配时返回空数组 `[]`（非 error）

**memory.store** — 写入新记忆：

```json
{"jsonrpc":"2.0","id":7,"method":"memory.store","params":{
  "title":"项目架构决策",
  "content":"本项目采用 CQRS 模式...",
  "tags":["architecture","cqrs"],"type":"longterm"
}}

// response
{"jsonrpc":"2.0","id":7,"result":{"id":"mem_1716376800000"}}
```

**memory.delete** — 删除记忆：

```json
{"jsonrpc":"2.0","id":9,"method":"memory.delete","params":{"id":"mem_1716376800000"}}

// response
{"jsonrpc":"2.0","id":9,"result":{"status":"ok","deleted_id":"mem_1716376800000"}}
```

> 删除不存在的 ID 返回 error。

---

#### Agent 配置管理

| Method | Params | 返回值 | 说明 |
|---|---|---|---|
| `agent.list` | 无 | `[]AgentEntry` | 列出所有 Agent 配置 |
| `agent.get` | `{ "name" }` | `*AgentConfig` | 获取单个 Agent 完整配置 |
| `agent.update` | `{ "name", ...可变字段 }` | `{ status, agent_name }` | 部分更新 Agent 并持久化到 `.md` 文件 |

**agent.list** — 列出所有 Agent：

```json
{"jsonrpc":"2.0","id":10,"method":"agent.list","params":{}}

// response
{"jsonrpc":"2.0","id":10,"result":[
  {"name":"architect","role":"Software Architect",
   "description":"Responsible for high-level system design...","model":"qwen3.6-plus",
   "skills":["architect","simplify","batch"],
   "enable_orchestration":false,"max_decompose_depth":0},
  {"name":"coder","role":"Software Engineer",
   "description":"A coding specialist focused on implementation...","model":"gpt-4o",
   "skills":["code-review","git"],"enable_orchestration":true}
]}
```

**agent.get** — 获取单个 Agent：

```json
{"jsonrpc":"2.0","id":11,"method":"agent.get","params":{"name":"architect"}}
```

**agent.update** — 部分更新 Agent 配置（**name 不可修改**）：

```json
// 全量更新多个字段
{"jsonrpc":"2.0","id":12,"method":"agent.update","params":{
  "name":"architect",
  "role":"Senior Software Architect",
  "description":"Updated: focuses on cloud-native architecture",
  "model":"gpt-4o",
  "skills":["architect","simplify","code-review"],
  "enable_orchestration":true,
  "max_decompose_depth":3
}}

// 仅更新 description（其他字段保持不变）
{"jsonrpc":"2.0","id":13,"method":"agent.update","params":{
  "name":"architect",
  "description":"Updated description only"
}}

// 更新 body（Markdown 正文内容）
{"jsonrpc":"2.0","id":14,"method":"agent.update","params":{
  "name":"architect",
  "body":"## Updated Identity\n\nI am a **Cloud Architect**..."
}}

// response (成功)
{"jsonrpc":"2.0","id":12,"result":{
  "status":"ok","agent_name":"architect","message":"agent config updated"
}}
```

> **写入位置**: `{UserPreferences}/agents/{name}.md`（如 `~/.mindx/agents/architect.md`）
>
> **部分更新规则**: 只传需要修改的字段，未传字段保持原值。
> 使用指针类型区分「未设置」与「显式设为 false/0」：
> - `enable_orchestration`: `*bool` — 不传则不改，传 `true`/`false` 则覆盖
> - `max_decompose_depth`: `*int` — 同理
> - 字符串/切片类型: 空字符串/nil 表示不修改，非空表示覆盖

**agent.update 可更新字段一览**:

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | **只读** — 定位目标 agent，不可修改 |
| `role` | string | 角色描述 |
| `description` | string | 能力描述 |
| `model` | string | 默认模型 |
| `skills` | `[]string` | 技能列表（整组替换） |
| `introduction` | string | 系统提示别名 |
| `body` | string | Markdown body（优先于 introduction 写入文件） |
| `enable_orchestration` | `*bool` | 编排模式开关 |
| `max_decompose_depth` | `*int` | WBS 最大分解深度 |
| `meta` | `map[string]any` | 扩展元数据 |

---

#### Model 模型管理

| Method | Params | 返回值 | 说明 |
|---|---|---|---|
| `model.list` | 无 | `[]ModelConfig` | 列出所有模型配置 |
| `model.get` | `{ "name" }` | `*ModelConfig` | 获取单个模型配置 |

**model.list**:

```json
{"jsonrpc":"2.0","id":15,"method":"model.list","params":{}}

// response
{"jsonrpc":"2.0","id":15,"result":[
  {"name":"gpt-4o","provider":"openai","base_url":"https://api.openai.com/v1",
   "api_key_ref":"OPENAI_API_KEY","max_tokens":128000,
   "supports_tools":true,"supports_vision":true},
  {"name":"claude-sonnet-4","provider":"anthropic",
   "base_url":"https://api.anthropic.com","api_key_ref":"ANTHROPIC_API_KEY",
   "max_tokens":200000}
]}
```

**model.get**:

```json
{"jsonrpc":"2.0","id":16,"method":"model.get","params":{"name":"gpt-4o"}}
```

---

#### Skill 技能管理

| Method | Params | 返回值 | 说明 |
|---|---|---|---|
| `skill.list` | `{ "agent_name": "可选" }` | `[]SkillEntry` | 列出当前 Agent 可用 Skills |
| `skill.get` | `{ "name" }` | `*Skill` | 获取单个 Skill 完整定义 |

**skill.list**:

```json
{"jsonrpc":"2.0","id":17,"method":"skill.list","params":{}}

// response
{"jsonrpc":"2.0","id":17","result":[
  {"name":"code-review","description":"Reviews code for quality and best practices",
   "root_dir":"/path/to/skills/code-review","source":"filesystem",
   "paths":["instructions.md","examples/"],"metadata":{"version":"1.0"}},
  {"name":"architect","description":"Generates architecture design documents",
   "root_dir":"","source":"bundled","paths":[],"metadata":{}}
]}
```

> 默认列出当前活跃 Agent 的 Skills。通过 `agent_name` 参数可指定其他 Agent。

**skill.get**:

```json
{"jsonrpc":"2.0","id":18,"method":"skill.get","params":{"name":"code-review"}}

// response (包含完整 instructions)
{"jsonrpc":"2.0","id":18,"result":{
  "name":"code-review","description":"Reviews code for quality and best practices",
  "root_dir":"/path/to/skills/code-review","source":"filesystem",
  "paths":["instructions.md"],"metadata":{"version":"1.0"},
  "instructions":"# Code Review Skill\n\n## Guidelines\n\n1. Check for..."
}}
```

---

### 方法索引总表

| # | Method | 类型 | 必填参数 | 返回类型 | 说明 |
|---|---|---|---|---|---|
| 1 | `session.list` | RPC | — | `[]SessionInfo` | 列出会话 |
| 2 | `session.get` | RPC | `session_id` | `{messages, meta?}` | 获取会话详情 |
| 3 | `session.meta` | RPC | `session_id` | `SessionMeta` | 获取会话元数据 |
| 4 | `memory.query` | RPC | `query` | `[]MemoryRecord` | 语义检索 |
| 5 | `memory.store` | RPC | `content` | `{id}` | 写入记忆 |
| 6 | `memory.delete` | RPC | `id` | `{status}` | 删除记忆 |
| 7 | `agent.list` | RPC | — | `[]AgentEntry` | 列出 Agent |
| 8 | `agent.get` | RPC | `name` | `*AgentConfig` | 获取 Agent 详情 |
| 9 | `agent.update` | RPC | `name` | `{status}` | 更新 Agent 配置 |
| 10 | `model.list` | RPC | — | `[]ModelConfig` | 列出模型 |
| 11 | `model.get` | RPC | `name` | `*ModelConfig` | 获取模型详情 |
| 12 | `skill.list` | RPC | — | `[]SkillEntry` | 列出技能 |
| 13 | `skill.get` | RPC | `name` | `*Skill` | 获取技能详情 |
| 14 | `command.list` | RPC (内置) | — | `[]CommandMeta` | 列出命令 |
| N | `user.message` | Notification | `{text}` | 事件流 | 用户消息 |

### 标准错误码

| Code | 含义 |
|---|---|
| -32700 | Parse Error — 无效的 JSON |
| -32600 | Invalid Request |
| -32601 | Method Not Found |
| -32602 | Invalid Params |
| -32603 | Internal Error |

## 服务端 → 客户端（事件推送）

所有事件均为 JSON-RPC **Notification**（无 ID），method 为对应的响应类型名（如 `thinking_delta`、`action_start`）。Params 中是一个 `ResponseEnvelope`，包含 `type`、`session_id`、`title`、`data`、`meta` 字段。

### 通用 Envelope 结构

```json
{
  "jsonrpc": "2.0",
  "method": "thinking_delta",
  "params": {
    "type": "thinking_delta",
    "session_id": "sess_abc123",
    "title": "思考中",
    "data": "...",
    "meta": {}
  }
}
```

各字段说明：
- `jsonrpc`：固定 `"2.0"`
- `method`：事件类型字符串，与 `params.type` 相同
- `params.type`：事件类型
- `params.session_id`：会话 ID（可能为空）
- `params.title`：中文标题（供客户端直接展示）
- `params.data`：事件数据，类型因事件而异（string / object / array）
- `params.meta`：附加元数据（可选，仅 TaskSummary 等使用）

### 事件类型一览

#### 思考阶段

| method | title | data 格式 |
|---|---|---|
| `thinking_delta` | "思考中" | string（流式文本片段） |
| `thinking_done` | "思考完成" | string（Markdown 文本） |

thinking_done data 示例：

```markdown
### 思考完成

**决策**: `read`  **置信度**: 92%

**推理**: 用户要求修改文件，需要先读取内容...

**即将调用工具**:
- `read_file` — `{"path": "main.go"}`
```

服务器端通过 `buildThinkingDoneMarkdown()` 将 `Thought` 结构体（reasoning, decision, confidence, tool_calls, clarification_question）渲染为 Markdown 字符串后发送。

#### 工具执行阶段

| method | title | data 格式 |
|---|---|---|
| `action_start` | "开始操作" / "工具开始" | 结构化对象 |
| `action_progress` | "操作进度" | 结构化对象 |
| `action_result` | "工具结果" | 结构化对象 |
| `action_end` | "操作完成" | 结构化对象 |

**注意**: `action_start` 被两个不同事件复用：
1. ActionStart（本轮级别）—— title="开始操作"，表示本轮将要调用 X 个工具
2. ToolExecStart（单个工具级别）—— title="工具开始"，表示单个工具开始执行

action_start data（批次级别，title="开始操作"）：

```json
{
  "tool_count": 3,
  "tool_names": ["read_file", "edit_file", "read_file"],
  "predicted_tokens": 523,
  "iteration": 2
}
```

action_start data（单工具级别，title="工具开始"）：

```json
{
  "tool_name": "read_file",
  "params": {"file_path": "main.go"}
}
```

action_progress data：

```json
{
  "completed": 1,
  "total": 3,
  "status": "进行中"
}
```

action_result data（单个工具执行完成）：

```json
{
  "tool_name": "read_file",
  "success": true,
  "result": "package main\n\nfunc main() { ... }",
  "error": "",
  "duration": "12.5ms"
}
```

action_end data：

```json
{
  "total": 3,
  "success": 2,
  "failed": 1,
  "summary": "2 tools succeeded, 1 failed"
}
```

#### 子任务（子 Agent 协作）

| method | title | data 格式 |
|---|---|---|
| `subtask_spawned` | "子任务生成" | string（Markdown 文本） |
| `subtask_completed` | "子任务完成" | string（Markdown 文本） |

subtask_spawned data 示例：

```markdown
### 🌿 子任务生成: `task_001`

**Agent**: reviewer
**描述**: 审查代码变更
```

subtask_completed data 示例（成功）：

```markdown
### ✅ 子任务完成: `task_001`

**回答**: 代码 LGTM，只有一个缩进问题
```

subtask_completed data 示例（失败）：

```markdown
### ❌ 子任务失败: `task_001`

**错误**: context deadline exceeded
```

#### 最终输出

| method | title | data 格式 |
|---|---|---|
| `final_answer` | "最终答案" | string（Markdown 文本） |

```json
{"type":"final_answer","session_id":"sess_abc123","title":"最终答案","data":"已经修改完成，改动如下：\n\n- main.go: 修复了 nil pointer dereference\n- utils.go: 添加了错误处理"}
```

#### 交互式（需要用户介入）

| method | title | data 格式 |
|---|---|---|
| `clarify_needed` | "需要澄清" | string（问题文本） |
| `permission_request` | "需要澄清"/"权限请求" | string（JSON 或 Markdown） |
| `permission_denied` | "权限拒绝" | string（原因） |

**permission_request 双路径**：根据 `PermissionRequestData.Questions` 长度走不同路径。

- **有 AskUser 问题**（`Questions > 0`）：title="需要澄清"，data 为 JSON 序列化的完整 `PermissionRequestData` 字符串，包含 tool_name、reason、security_level、questions（含 label 和 options 列表）
- **无 AskUser 问题**（纯权限请求）：title="权限请求"，data 为 Markdown 字符串

permission_request data（AskUser 模式）：

```json
{
  "type": "permission_request",
  "session_id": "sess_abc123",
  "title": "需要澄清",
  "data": "{\"tool_name\":\"edit_file\",\"reason\":\"需要修改 main.go\",\"security_level\":2,\"questions\":[{\"question\":\"确定要修改 main.go？\",\"options\":[\"是\",\"否\"]}]}"
}
```

permission_request data（纯权限请求，Markdown 模式）：

```markdown
### 🔒 权限请求: `edit_file`

**原因**: 需要修改 main.go
**安全级别**: 2
```

#### 汇总

| method | title | data 格式 |
|---|---|---|
| `cycle_end` | "循环结束" | string（Markdown 文本） |
| `execution_summary` | "执行摘要" | table（结构化表格） |
| `task_summary` | "任务总结" | string（Markdown 文本）+ meta 字段 |

cycle_end data 示例：

```markdown
### 🔄 T-A-O 循环结束 (迭代 #3, 耗时 1.2s)
```

execution_summary data（table 格式，由服务器 `sendExecutionSummary()` 构造）：

```json
{
  "type": "execution_summary",
  "session_id": "sess_abc123",
  "title": "执行摘要",
  "data": {
    "headers": ["Metric", "Value"],
    "rows": [
      {"metric": "Iterations", "value": "5"},
      {"metric": "Tool Calls", "value": "12"},
      {"metric": "Tools Used", "value": "read_file, edit_file, grep, web_search"},
      {"metric": "Duration", "value": "12.5s"},
      {"metric": "Tokens Used", "value": "4523"},
      {"metric": "Termination", "value": "task_complete"}
    ]
  }
}
```

task_summary data（data 为 Markdown 字符串，meta 包含 token 统计）：

```json
{
  "type": "task_summary",
  "session_id": "sess_abc123",
  "title": "任务总结",
  "data": "### 📋 任务总结\n\n修复了 main.go 中的 nil pointer dereference 问题，添加了单元测试\n\n**Token**: 输入 21500 / 输出 3200",
  "meta": {
    "input_tokens": 21500,
    "output_tokens": 3200
  }
}
```

#### 错误

| method | title | data 格式 |
|---|---|---|
| `error` | "错误" | string（错误描述） |

```json
{"type":"error","session_id":"sess_abc123","title":"错误","data":"Agent developer 未找到"}
```

## 典型会话流程

完整的一次用户请求到最终回答的事件流：

```
Client                                     Daemon
  │                                           │
  │── JSON-RPC Notification ─────────────────→│
  │   method: user.message                    │
  │   params: {"text":"@developer 分析bug"}   │
  │                                           │
  │←── thinking_delta (1..N) ────────────────│  流式思考文本
  │←── thinking_done ────────────────────────│  思考完成（Markdown）
  │←── action_start ────────────────────────│  "准备调用2个工具"
  │←── action_result (read_file) ───────────│  工具1结果
  │←── action_result (grep) ────────────────│  工具2结果
  │←── action_end ──────────────────────────│  "2 tools succeeded"
  │                                           │
  │←── thinking_delta (1..N) ────────────────│  第二轮思考
  │←── thinking_done ────────────────────────│
  │←── final_answer ────────────────────────│  "bug 是..."
  │                                           │
  ── 若有子 Agent 协作: ──────────────────────│
  │←── subtask_spawned ─────────────────────│  reviewer 被唤起（Markdown）
  │←── subtask_completed ───────────────────│  审查完成（Markdown）
  │←── execution_summary ───────────────────│  执行摘要（表格）
  │←── task_summary ────────────────────────│  任务总结（Markdown + token）
```

注意：`action_start` 会在批次开始和每个工具开始前各推送一次（method 相同但 data 结构不同，通过 title 区分）。`permission_request` 在有 AskUser 问题时推送 JSON 字符串，否则推送 Markdown。

## 通用响应类型

除 Agent 事件专用类型外，框架还定义了一组通用响应类型，主要用于命令响应：

| type | 用途 | data 格式 |
|---|---|---|
| `text` | 纯文本通知 | string |
| `markdown` | 富文本展示 | string（Markdown） |
| `table` | 表格数据（命令结果） | `{"headers":[],"rows":[]}` |
| `error` | 业务错误 | string |
| `progress` | 进度更新 | `{"total":N,"current":N,"message":"..."}` |
| `confirm` | 确认弹窗 | `{"message":"...","detail":"..."}` |
| `options` | 选项选择 | `[{"value":"...","label":"..."}]` |

命令处理器通过 `CommandContext.RespondWithType()` 推送上述类型的 Notification，同时通过 JSON-RPC Response 的 `result` 字段返回结构化数据。例如 `/agents` 命令同时推送一个 `type: "table"` 的通知用于 TUI 渲染，并在 `result` 中返回原始 Agent 数据供编程客户端使用。

所有响应均通过 `ResponseEnvelope` 包装，客户端通过 `type` 字段分发到对应的事件处理器。
