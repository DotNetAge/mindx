# MindX2 TUI 设计

> **注意：本文档是设计规约，所有实现必须严格遵循。任何偏离需先修改本文档再实现。**

MindX2 的 TUI 基于 Bubble Tea v2 的 MVU 架构实现。整个界面为**树状组件组合结构**，严禁将全部状态集中到单个 Model 中。

## 核心架构原则

### 原则 1：树状组件组合

界面的组件构成一棵树，每个组件是一个独立的 `tea.Model`，拥有自己的状态、消息类型、Update 和 View。**禁止将多个组件内嵌到一个 Model 中。**

```
                    ┌──────────────────────────────────────┐
                    │              RootModel                │  ← 仅负责组装、路由、外部通信
                    └──────┬──────────────────────┬────────┘
                           │                      │
              ┌────────────▼──────────┐    ┌──────▼───────┐
              │      ContentPanel     │    │   StatusBar  │
              │      (tea.Model)      │    │  (tea.Model) │
              │  scrollable viewport  │    └──────────────┘
              └────────────┬──────────┘
                           │  顺序: Welcome (不存在时显示) + AgentAnswer[]
              ┌────────────┼────────────────────────────┐
              │            │                            │
         ┌────▼────┐  ┌───▼────────────┐       ┌───────▼───────────┐
         │ Welcome │  │  AgentAnswer   │  ...  │    AgentAnswer    │
         │(默认展示)│  │  (tea.Model)   │       │    (tea.Model)    │
         └─────────┘  └───┬────────────┘       └───────────────────┘
                          │  顺序: Thinks → Results（从上到下）
               ┌──────────┼──────────┐
               │          │          │
          ┌────▼───┐ ┌───▼────┐ ┌───▼───────┐
          │ Thinks │ │Results │ │  TipsBar  │  ← 默认隐藏，工具执行时显示
          │流式追加 │ │Markdown│ │ 单行+spinner│
          │        │ │表格/待办│ └───────────┘
          └────────┘ └────────┘

              ┌──────────────────────────────┐
              │   InputLayer（底部输入层）      │
              │  ┌──────┐ ┌────┐ ┌─────┐    │
              │  │Input │ │AGS │ │CMDS │    │  ← 三者互斥显示
              │  │Area  │ │uggest│ │uggest│   │
              │  └──────┘ └────┘ └─────┘    │
              │         ChoicesPanel         │  ← 与 InputLayer 互斥
              └──────────────────────────────┘
```

**执行期间状态：**

```
请求/命令执行中:
 ContentPanel (可见) | StatusBar (可见)
 InputLayer (全部隐藏) | ChoicesPanel (全部隐藏)

响应返回后:
 ContentPanel (可见, 已追加新内容)
 StatusBar (可见)
 InputLayer (按消隐条件恢复可见)
 ChoicesPanel (需要选择时显示, 与 InputLayer 互斥)
```

### 原则 2：Root 是协调器，不是万能容器

RootModel 的职责**仅限于**：

1. **创建子组件**并持有子组件引用
2. **消息路由**：从 tea.Msg 中提取信息，调用对应子组件的 Update，或将网络消息转换为子组件理解的事件
3. **View 组装**：用 `lipgloss.JoinVertical` 将子组件的 View() 拼接起来
4. **外部通信**：持有 `*gateway.Client`，管理连接、消息通道
5. **执行生命周期**：管理当前是否处于"请求执行中"状态，此状态下隐藏所有输入组件

RootModel 的 Update 中，对自身字段的要求：

- **只允许持有**：子组件引用、Client、通道、共享只读引用（如 `*SlashCommandRegistry`）、执行状态标识
- **不允许持有**：业务状态（消息列表、AgentAnswer 内部数据）、UI 子状态

### 原则 3：每个子组件自我管理

每个子组件：

- 定义自己的 `tea.Model` 类型（**必须**与其它组件是不同 struct 类型）
- 定义自己的消息类型（**必须在消息名前加组件名前缀**，避免消息名冲突）
- 实现自己的 `Init()`、`Update()`、`View()`
- 对外的通信通过 Root 转发的 `tea.Cmd` 或消息订阅实现
- 子组件之间**不允许直接引用**，所有跨组件通信必须经过 RootModel

### 原则 4：文件组织

每个子组件一个 `.go` 文件，以组件名命名：

```
mindx/internal/client/
├── component_root.go              # RootModel
├── component_content.go           # ContentPanel + Welcome 渲染
├── component_agent_answer.go      # AgentAnswer
├── component_statusbar.go         # StatusBar
├── component_inputbox.go          # InputBox / textarea
├── component_suggest_agent.go     # AgentSuggestions
├── component_suggest_cmd.go       # CommandSuggestions
├── component_choices.go           # ChoicesPanel
├── connect.go                     # 网络连接
├── fetch.go                       # 网络请求
├── filter.go                      # 工具函数
├── types.go                       # 全局类型/styles
├── registry.go                    # SlashCommandRegistry
```

***

## 界面组件（完整定义）

### 1. ContentPanel（内容面板）

整体可滚动的内容区域。基于 viewport 实现，标准 CLI 滚动行为：新内容在底部，旧内容向上滚出，用户可用鼠标/键盘滚动回溯。

```
ContentPanel（viewport 包裹）
├── [可选] Welcome 面板（AgentAnswer 列表为空时显示）
├── AgentAnswer{sessionID: "xxx", agentName: "master"}    ← 主 Agent 会话
├── AgentAnswer{sessionID: "yyy", agentName: "coder"}     ← 子 Agent 独立会话
├── AgentAnswer{sessionID: "zzz", agentName: "researcher"}← 另一个子 Agent
├── ...
```

**内容管理规则：**

- **所有内容都是 append-only**：ContentPanel 的内容只会增长，不会删除（Ctrl+L 除外）
- Welcome 仅首次启动时显示，之后不再出现
- 每个 AgentAnswer 对应服务器端的**一个独立会话**（sessionID + agentName 联合唯一标识）
- 多 Agent 协作时主 Agent 调起子 Agent，子 Agent 获得新 session，在 TUI 上以新 AgentAnswer 呈现

**Ctrl+L 清屏行为：**

- 与 Linux `clear` 命令完全一致：**所有内容全部清除**，包括 Welcome
- 清屏后 Welcome 彻底消失，不重新显示。后续内容从第一个 AgentAnswer 开始

```go
type ContentPanel struct {
    width, height    int
    viewport         viewport.Model
    glamourRenderer  *glamour.TermRenderer
    welcome          WelcomeData
    answers          []*AgentAnswer       // 有序列表，按时间排序
}

type WelcomeData struct {
    appTitle   string
    version    string
    agentName  string
    workspace  string
    sessionID  string
}
```

***

### 2. Welcome（欢迎面板）

程序启动时的系统信息显示，**仅显示一次**。

| 字段        | 来源                                    |
| --------- | ------------------------------------- |
| App Title | 编译时版本号                                |
| Version   | 编译时版本号                                |
| AgentName | 从 agentsFetchedMsg 获取的 master agent 名 |
| Workspace | 由环境变量或启动参数指定                          |
| SessionID | TUI 本地生成的 UUID                        |

**行为规则：**

- 程序启动后立即显示，作为 ContentPanel 的首个内容
- 用户 Ctrl+L 清屏后 Welcome **彻底消失**，之后无需再处理它的显示逻辑
- 后续所有内容以 AgentAnswer 追加

***

### 3. AgentAnswer（Agent 回复块）

对应**服务器端的一个会话单元**。一个会话由 (sessionID, agentName) 唯一标识。

#### Session 与 AgentAnswer 的对应关系

```
用户视角:                        TUI 内部:
 主 Agent 对话 ──────────────→ AgentAnswer{sessionID="A", agentName="master"}
   ├── 调起子 Agent ──────────→ AgentAnswer{sessionID="B", agentName="coder"}
   │    子 Agent 执行完成 ────→ AgentAnswer 完成，内容保留
   │    子 Agent 会话自动结束 ─→ 后台清理，UI 上 AgentAnswer 仍保留
   │
   └── 主 Agent 继续对话 ─────→ AgentAnswer{sessionID="A"} 继续追加内容
   
 主 Agent 对话结束 ───────────→ 所有子 Agent 会话（B, C, ...）后台自动结束
                              UI 上所有 AgentAnswer 内容保留（append-only）
```

**每个 AgentAnswer 内部结构（从上到下）：**

```
 AgentAnswer{sessionID="xxx", agentName="coder"}
 ┌─────────────────────────────────────────┐
 │  [灰色斜体] 思考过程                     │  ← Thinks：流式追加
 │  我在分析这个问题的上下文...               │
 │                                         │
 │  [Markdown] 结果输出                     │  ← Results：Markdown
 │  这是一个表格数据:                        │      含表格/待办/纯文本
 │  ┌─────┬──────┐                        │      （只读，不可交互）
 │  │ ID  │ Name │                        │
 │  ├─────┼──────┤                        │
 │  │ 1   │ Foo  │                        │
 │  └─────┴──────┘                        │
 │                                         │
 │  @coder [正在执行: read_file]  [预计 2.5K]│  ← TipsBar：@agentName 打头
 │                                         │      （默认隐藏，执行时显示）
 └─────────────────────────────────────────┘
```

**内部排列顺序（不可变）：Thinks → Results → TipsBar**

**生命周期：**

```
Root 收到 sendMsg → ContentPanel.CreateAnswer(sessionID, agentName)
  → 追加到 ContentPanel.answers[]
  → 收到 thinking_delta → AgentAnswer.AppendThinking(content)
  → 收到 action_start    → AgentAnswer.ShowTips(toolName, tokens)
  → 收到 action_progress → AgentAnswer.TipsBar.UpdateText(text)
  → 收到 action_result   → AgentAnswer.HideTips()
  → ... (多个工具循环)
  → 收到 final_answer    → AgentAnswer.AppendResult(content)
  → 收到 execution_complete → 该会话的活跃期结束
  → 该 AgentAnswer 转为历史记录，不再接受更新

子 Agent 场景：
  主 Agent 对话中 → 调起子 Agent → 服务器创建新 session
  → TUI 收到新 session 的事件 → 创建新的 AgentAnswer
  → 新 AgentAnswer 与主 AgentAnswer 并列在 ContentPanel 中
  → 子 Agent 会话结束 → 其 AgentAnswer 转为历史记录
```

**Results 的内容类别与扩展机制：**

Results 区域设计为**可扩展的内容渲染器**，支持注册新的内容格式，而非硬编码的 switch-case。

**当前已知的渲染器：**

| 类别       | 标识符          | 渲染方式                                  |
| -------- | ------------ | ------------------------------------- |
| 纯文本      | `"markdown"` | glamour Markdown 渲染                   |
| 表格       | `"table"`    | renderTableEnvelope                   |
| 待办/任务    | `"todo"`     | renderTodoEnvelope                    |
| 选项列表（只读） | `"options"`  | renderOptionsEnvelope                 |
| JSON     | `"json"`     | 自动格式化（isJSON 检测后 fallback 到 markdown） |

**扩展机制：**

```go
// 渲染器注册表 — 允许新增内容格式而不改动 AgentAnswer 核心逻辑
type ResultRenderer func(content string, width int) string

type ContentPanel struct {
    // ...
    renderers map[string]ResultRenderer  // key = 内容类型标识
}
```

注册新渲染器：

```go
contentPanel.RegisterRenderer("mermaid", func(content string, width int) string {
    // 将 mermaid 图表渲染为 ASCII 示意图
})

contentPanel.RegisterRenderer("diff", func(content string, width int) string {
    // 语法高亮的 diff 渲染
})
```

**路由规则：**

- 服务器返回的 `typedContentMsg` 携带 `contentType` 字段（如 `"table"`, `"mermaid"`）
- ContentPanel 根据 `contentType` 查找注册的渲染器，如果未注册则 fallback 到 markdown 渲染
- 这种方式保证未来任何新内容格式都可以通过注册新渲染器来支持，无需修改已有的组件结构

**ChoicesPanel 与 Results 的区别：**

- Results 的选项是**只读的展示信息**
- ChoicesPanel 是可以**选择并回传结果**的交互组件

***

### 4. Thinks（思考区）

AgentAnswer 的子区域，显示 Agent 思考过程。

- 文本风格：灰色、斜体
- 显示方式：打字型流式追加（同一 thinking\_delta 流中追加到同一块）
- 数据源：`agentEventMsg` 中 `eventType == "thinking"` 的内容
- 路由到 sessionID 匹配的 AgentAnswer

### 5. Results（结论区）

AgentAnswer 的子区域，显示 Agent 最终输出。

- 支持 Markdown（通过 glamour 渲染）
- 支持 JSON 自动格式化（isJSON 检测）
- 支持表格/待办/选项列表的渲染（只读展示）
- 每条结果带有 @AgentName 标签

### 6. TipsBar（提示栏 → AgentAnswer 内部组件）

**TipsBar 从 Root 的直接子组件变为 AgentAnswer 的内部组件。**

- 默认隐藏，工具执行时显示，完成后自动隐藏
- 单行显示，第一个信息项是 `@agentName`，标识当前执行工具的 Agent
- 格式：`@agentName [正在执行: toolName] [预计 X.XK tokens] [$ spinner动画]`
- Tips 与 AgentAnswer 生命周期绑定

***

### 7. StatusBar（状态栏）

始终显示，单行，固定在 ContentPanel 与 InputLayer 之间。

```
[Connected] [5.7K Tokens]           [gpt-4o | tokens: ↓1.2K ↑4.5K]
```

**布局规则：**

- 左侧：连接状态 + Token 总量
- 右侧：当前 Model + 当前会话的 Token 消耗（输入↓ + 输出↑）
- 左右两端对齐

***

### 8. InputBox（输入栏）

使用 `textarea.Model`（bubbles 官方组件）实现多行编辑。

**快捷键：**

- `Enter`：发送消息（触发 `sendMsg`）
- `Alt+Enter`：换行
- `Ctrl+C`：退出程序
- `Ctrl+L`：清屏（清除 ContentPanel 所有内容，同 Linux `clear`）

**无外部编辑器**：删除原有的 `ctrl+g` / `ctrl+x` 打开外部编辑器的功能。

**互斥规则：**

- 用户输入 `@prefix` → 显示 AgentSuggestions，隐藏 CommandSuggestions
- 用户输入 `/prefix` → 显示 CommandSuggestions，隐藏 AgentSuggestions
- 匹配不到 → 两者都隐藏

**命令输入历史：由 InputBox 内部管理。**

***

### 9. AgentSuggestions（Agent 建议列表）

| 属性   | 值                    |
| ---- | -------------------- |
| 触发条件 | 输入匹配 `^@[\w-]*$`     |
| 隐藏条件 | 输入离开 `@` 模式          |
| 最大行数 | 5                    |
| 组件   | bubbles `list.Model` |
| 选择行为 | 补全到输入框 + 切换当前 agent  |

### 10. CommandSuggestions（命令建议列表）

| 属性   | 值                    |
| ---- | -------------------- |
| 触发条件 | 输入匹配 `^/[\w-]*$`     |
| 隐藏条件 | 输入离开 `/` 模式          |
| 最大行数 | 5                    |
| 组件   | bubbles `list.Model` |
| 选择行为 | **选中后直接执行该命令**       |

### 11. ChoicesPanel（选择器）

当服务器要求用户选择时显示，默认隐藏。与 Results 不同，ChoicesPanel 负责**可交互的选择操作**。

```
┌─────────────────────────────────────────┐
│ 请选择要切换的 Agent:                     │
│  1. master     通用助手                   │
│  2. coder      代码助手                   │
│  按数字键或上下选择，Enter 确认            │
└─────────────────────────────────────────┘
```

**显示规则：**

- ChoicesPanel 可见时 InputBox 及所有 Suggestions 隐藏
- 选择完成后自动隐藏，恢复 InputBox

***

## 会话隔离架构：Per-Session Goroutines

这是整个 TUI 的消息通信基础。基于 GoReact 的会话隔离特性，每个会话拥有独立的 goroutine 和专属通道。

### 当前问题（现状）

当前代码中所有会话共用同一组通道（`respCh`, `typedCh`, `agentCh`）和单一 `OnReceived` 回调。当多个会话并行时，消息会在共享通道中交错，无法区分归属。

### 目标架构：Per-Session Channels

每个活跃会话拥有自己独立的通道和 goroutine：

```
RootModel
│
├── activeSessions map[string]*sessionRuntime
│     │
│     ├── key: "session-master-001"
│     │     └── sessionRuntime{
│     │           sessionID:  "session-master-001",
│     │           agentName:  "master",
│     │           answerRef:  *AgentAnswer ← 指向 ContentPanel 中的对应 AgentAnswer
│     │           respCh:     chan string,        ← 专属通道
│     │           agentCh:    chan agentEventMsg,
│     │           stopCh:     chan struct{},      ← 终止信号
│     │         }
│     │
│     └── key: "session-coder-002"
│           └── sessionRuntime{
│                 sessionID:  "session-coder-002",
│                 agentName:  "coder",
│                 answerRef:  *AgentAnswer ← 另一个 AgentAnswer
│                 respCh:     chan string,
│                 agentCh:    chan agentEventMsg,
│                 stopCh:     chan struct{},
│               }
│
└── client.On*(handler)  ← 统一注册一次，handler 内按 sessionID 分发
```

### 生命周期

```go
type sessionRuntime struct {
    sessionID string
    agentName string
    answerRef *AgentAnswer          // 指向 ContentPanel 中的 AgentAnswer

    // 专属通道（goroutine-safe 隔离）
    respCh   chan string
    agentCh  chan agentEventMsg
    stopCh   chan struct{}
}
```

**创建（用户发送消息时）：**

```
InputBox 按 Enter → Root.Update(sendMsg)
  │
  ├── 1. 生成 sessionID
  │     （若服务器返回中携带 sessionID 则用服务器的，否则 TUI 本地生成）
  │
  ├── 2. ContentPanel.CreateAnswer(sessionID, agentName)
  │     → 返回 *AgentAnswer
  │
  ├── 3. 创建 sessionRuntime{ sessionID, agentName, answerRef, channels }
  │     → 存入 rootModel.activeSessions[sessionID]
  │
  ├── 4. 启动 session goroutine:
  │     go sessionLoop(client, runtime)
  │
  └── 5. executing = true, 隐藏 InputLayer
```

**session goroutine（`sessionLoop`）—— 无超时：**

```go
func sessionLoop(client *gateway.Client, rt *sessionRuntime) {
    for {
        select {
        case msg := <-rt.respCh:
            // 推入 Bubble Tea 事件循环
        case event := <-rt.agentCh:
            // 推入 Bubble Tea 事件循环
        case <-rt.stopCh:
            close(rt.respCh)
            close(rt.agentCh)
            return
        case <-client.Done():
            return
        }
    }
}
```

**关键：session goroutine 不设置任何定时器或超时。** 只有两种退出路径：

- `stopCh` 收到信号（会话正常结束）
- `client.Done()` 收到信号（连接断开）

删除旧的 `waitServerMsg`、`waitAgentEvent`、`waitTypedContent` 等函数及其中所有 `time.After` 超时逻辑。这些超时在旧架构中引发了指数级消息级联，导致系统行为不可预测。

**分发机制（client 事件 handler 注册一次）：**

client 的 `OnReceived` / `On(RespXxx)` 等 handler 在 `Init()` 中统一注册一次。handler 内部从服务器消息中提取 sessionID，然后推送到对应 sessionRuntime 的通道：

```go
// 一次注册，按 sessionID 分发
m.client.On(string(gateway.RespThinkingDelta), func(ctx context.Context, params json.RawMessage) {
    var env struct {
        SessionID string `json:"session_id"`
        Data      string `json:"data"`
    }
    if err := json.Unmarshal(params, &env); err != nil {
        return
    }
    // 找到 sessionRuntime 并推送
    if rt, ok := m.activeSessions[env.SessionID]; ok {
        select {
        case rt.agentCh <- agentEventMsg{eventType: "thinking", content: env.Data}:
        default:
        }
    }
})
```

**销毁（会话结束时）：**

```
收到 final_answer / error / execution_complete
  → close(rt.stopCh)
  → 从 activeSessions 中移除
  → 若 activeSessions 为空 → executing = false, 恢复 InputLayer
```

### 服务器协议要求

服务器**必须**在每条事件消息中携带 `session_id` 字段，对于 `typedContentMsg`（表格/待办等）还需携带 `content_type` 字段以支持内容渲染器路由：

```json
// RespTable
{
    "session_id": "session-master-001",
    "content_type": "table",
    "data": { "headers": [...], "rows": [...] }
}

// RespThinkingDelta
{
    "session_id": "session-master-001",
    "content_type": "thinking",
    "data": "正在分析上下文..."
}

// RespFinalAnswer
{
    "session_id": "session-coder-002",
    "content_type": "markdown",
    "data": "已完成代码审查"
}
```

若服务器当前不包含 `session_id`，由于 GoReact 内部基于会话隔离，服务器端可以很容易地在消息发送时注入当前会话 ID。

***

## 执行期间隐藏规则

```
执行前:                         执行中:                         执行后:
┌───────────────────┐          ┌───────────────────┐          ┌───────────────────┐
│   ContentPanel    │          │   ContentPanel    │          │   ContentPanel    │
│                   │          │   (正在追加内容)   │          │   (已追加新内容)   │
│   StatusBar       │          │   StatusBar       │          │   StatusBar       │
│   InputBox /      │          └───────────────────┘          │   InputBox /      │
│   ChoicesPanel    │                                        │   ChoicesPanel    │
└───────────────────┘                                        └───────────────────┘
                            ↑ 全部隐藏                        ↑ 恢复显示
```

Root 管理 `executing bool` 状态：

- `sendMsg` 发出 → `executing = true` → 隐藏 InputLayer + ChoicesPanel
- `final_answer` / `error` / `complete` 到达 → `executing = false` → 恢复 InputLayer

并行场景（子 Agent 在主 Agent 结束前被调起）：

- 主 Agent 和子 Agent 各有自己的 sessionID
- 需要所有活跃 session 都结束时才 `executing = false`

***

## RootModel 消息路由表

Root 是唯一持有 `*gateway.Client` 和 `activeSessions` 的组件。消息在两种维度上路由：

- **操作系统消息** → 分发给所有子组件
- **业务消息** → 按 sessionID 路由到对应 sessionRuntime，再经 `tea.Program.Send()` 或 `tea.Msg` 进入 Update

### 操作系统消息

| 消息类型                | 分发逻辑                                                   |
| ------------------- | ------------------------------------------------------ |
| `tea.WindowSizeMsg` | 分发给 ContentPanel, StatusBar, InputBox                  |
| `tea.KeyPressMsg`   | 执行中 → 忽略；ChoicesPanel 可见 → 给 ChoicesPanel；否则给 InputBox |
| `tea.MouseWheelMsg` | ContentPanel.viewport                                  |
| `tea.PasteMsg`      | InputBox                                               |

### 内部业务消息

| 消息类型                   | 来源                | 分发逻辑                                                                                                     |
| ---------------------- | ----------------- | -------------------------------------------------------------------------------------------------------- |
| `connectedMsg`         | connect.go        | StatusBar.Connected(true) + fetchCommands + fetchAgents                                                  |
| `agentsFetchedMsg`     | fetch.go          | 更新共享引用，转发给 AgentSuggestions                                                                              |
| `commandsFetchedMsg`   | fetch.go          | 更新 registry，转发给 CommandSuggestions                                                                       |
| `sendMsg`              | InputBox          | **创建 sessionRuntime（含专属通道）→ 启动 session goroutine → 记录到 activeSessions → executing=true → 隐藏 InputLayer** |
| `agentAnswerUpdateMsg` | session goroutine | 携带 sessionID 和具体更新类型 → 查找到对应 AgentAnswer → 调用 AppendThinking / AppendResult / ShowTips / HideTips        |
| `agentAnswerDoneMsg`   | session goroutine | 对应 session 结束 → ContentPanel 标记 AgentAnswer 为已完成 → activeSessions 移除 → 若全部完成则 executing=false            |
| `agentEventMsg`        | 通道                | **不再使用**。已由 per-session goroutine + `agentAnswerUpdateMsg` 替代                                            |
| `serverMsg`            | 通道                | **不再使用**。已由 per-session goroutine + `agentAnswerUpdateMsg` 替代                                            |
| `typedContentMsg`      | 通道                | **不再使用**。由 per-session goroutine 统一转为 `agentAnswerUpdateMsg`                                             |
| `errMsg`               | 通道                | 路由到对应 session 的 AgentAnswer + 关闭 sessionRuntime                                                          |
| `agentSwitchMsg`       | InputBox          | 更新 Root.currentAgent，通知 StatusBar                                                                        |
| `clearMsg`             | InputBox(Ctrl+L)  | ContentPanel.ClearAll()                                                                                  |

### 数据流向（重构后）

```
用户 Enter
  → InputBox 返回 sendMsg
  → Root 创建 sessionRuntime + AgentAnswer
  → Root 启动 sessionLoop goroutine
       ↓
  session goroutine:
    client.Notify("user.message", ...)    ← 发送请求给服务器
    ↓
    服务器按 GoReact 会话模型处理
    每条响应/事件都携带 session_id
    ↓
    client.On*(handler) 提取 session_id
    ↓
    推送到对应 sessionRuntime 的专有通道
    ↓
    sessionLoop 从专有通道收到消息
    ↓
    sessionLoop 将消息封装为 agentAnswerUpdateMsg{sessionID, type, data}
    ↓
    推送到 (通过 chan tea.Msg 或 tea.Program.Send)
    ↓
  Root.Update(agentAnswerUpdateMsg)
    ↓
    按 sessionID 查找到 activeSessions[sessionID]
    → 调用 agentAnswerRef.AppendThinking / AppendResult / ...
    → ContentPanel 更新 viewport 内容
```

对比旧架构：

```
旧: 共享通道 → Root.Update(原始消息) → 大 switch 分发给 *model 自身
新: 专有通道 → sessionLoop → agentAnswerUpdateMsg → Root.Update → 按 sessionID 路由到 AgentAnswer
```

***

## RootModel 结构

```go
type rootModel struct {
    // 直接子组件
    contentPanel *ContentPanel
    statusBar    StatusBar
    inputBox     InputBox
    choicesPanel ChoicesPanel

    // 外部通信（Root 统一持有 Client）
    client   *gateway.Client
    registry *SlashCommandRegistry
    sessionID string               // 当前会话的 TUI 本地 ID

    // 会话隔离 — 每个活跃会话一个 goroutine
    activeSessions map[string]*sessionRuntime

    // 跨组件共享状态
    currentAgent string
    currentModel string

    // 执行状态
    executing     bool
    sessionStart  time.Time
}

// sessionRuntime 管理一个会话的生命周期
type sessionRuntime struct {
    sessionID string
    agentName string
    answerRef *AgentAnswer         // ContentPanel 中的对应 AgentAnswer

    respCh   chan string           // 专属通道：普通服务器响应
    agentCh  chan agentEventMsg    // 专属通道：Agent 事件
    stopCh   chan struct{}         // 关闭信号
}
```

***

## 初始化流程

```
NewProgram()
  ├── 生成 sessionID
  ├── 创建 ContentPanel
  ├── 创建 StatusBar
  ├── 创建 InputBox(registry)
  ├── 创建 ChoicesPanel
  ├── 初始化 activeSessions = make(map[string]*sessionRuntime)
  └── return rootModel{...}

Init()
  ├── 注册 client 事件处理器（一次注册，内部按 sessionID 分发）:
  │     client.OnReceived(func(msg) { 提取 sessionID → 找 activeSessions → 推 respCh })
  │     client.On(RespThinkingDelta, ...) { 提取 sessionID → 找 activeSessions → 推 agentCh }
  │     client.On(RespFinalAnswer, ...) { 提取 sessionID → 找 activeSessions → 推 agentCh }
  │     ... 其他事件 handler
  │
  └── connectWithRetry()

连接建立 → fetchAgents, fetchCommands
         → 显示 Welcome + StatusBar.Connected

用户按 Enter:
  InputBox → sendMsg{text, targetAgent}
  Root:
    1. 生成 sessionID（如 "session-{agentName}-{seq}"）
    2. ContentPanel.CreateAnswer(sessionID, targetAgent)
    3. 创建 sessionRuntime{channels, answerRef}
    4. activeSessions[sessionID] = &runtime
    5. executing = true
    6. go sessionLoop(client, &runtime)   ← 启动专属 goroutine
       在 sessionLoop 内部:
         client.Notify("user.message", ...)
         for { select { case <-runtime.respCh: ... case <-runtime.agentCh: ... } }

Agent 响应到达:
  sessionLoop 收到消息
    → 封装为 agentAnswerUpdateMsg{sessionID, type, content}
    → 推入 Root 的更新队列
  Root.Update(agentAnswerUpdateMsg)
    → activeSessions[sessionID].answerRef.AppendXxx()
    → 如果是 final_msg → close(stopCh), 清除 sessionEntry
    → activeSessions 为空 → executing = false → 恢复 InputLayer

子 Agent 场景:
  服务器在处理主 Agent 请求过程中调起子 Agent
  子 Agent 的响应也携带 sessionID（新 ID）
  TUI 的 On* handler 发现 sessionID 不在 activeSessions 中
  → Root 自动创建新的 AgentAnswer + sessionRuntime
  → 新的 session goroutine 启动
  → 子 Agent 的 Thinking/Result 独立构建在自己的 AgentAnswer 中
```

***

## 文件拆分计划

```
mindx/internal/client/
├── component_root.go              # RootModel + sessionRuntime
├── component_content.go           # ContentPanel + Welcome + AgentAnswer[]
├── component_agent_answer.go      # AgentAnswer (Thinks+Results+Tips 内部逻辑)
├── component_statusbar.go         # StatusBar
├── component_inputbox.go          # InputBox (textarea, Alt+Enter换行)
├── component_suggest_agent.go     # AgentSuggestions
├── component_suggest_cmd.go       # CommandSuggestions (选中直接执行)
├── component_choices.go           # ChoicesPanel
├── session.go                     # sessionRuntime + sessionLoop (新增)
├── connect.go                     # 网络连接 (client handler 注册, 无超时逻辑)
├── fetch.go                       # fetchAgents, fetchCommands
├── registry.go                    # SlashCommandRegistry
├── types.go                       # 全局类型/styles
└── filter.go                      # 工具函数
```

***

## 迁移清单

- [x] 创建 `component_root.go`，精简为协调器 + executing 状态机 + activeSessions 管理
- [x] 创建 `component_content.go`，Welcome + AgentAnswer 列表管理
- [x] 创建 `component_agent_answer.go`，Thinks + Results + Tips 容器逻辑
- [x] 创建 `component_statusbar.go`
- [x] 创建 `component_inputbox.go`，textarea 替代 textinput，Alt+Enter 换行
- [x] 创建 `component_suggest_agent.go`
- [x] 创建 `component_suggest_cmd.go`，选中直接执行
- [x] 创建 `component_choices.go`
- [x] 创建 `session.go`，实现 sessionRuntime + sessionLoop + 通道管理
- [x] 修改 `connect.go` 中的 handler 注册逻辑，改为 sessionID 感知的分发；**删除所有超时函数（waitServerMsg/waitAgentEvent/waitTypedContent 的 time.After 逻辑）**
- [x] 在服务器端（GoReact 层）确保每一条响应/事件都携带 `session_id` 字段
- [x] 删除 `model.go`、`interaction.go`、`display.go` 及外部编辑器相关代码

