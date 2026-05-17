# MindX TUI Architecture V2 — 纯 ELM 组件化设计

> 本文件是架构设计文档，不含代码实现细节。
> UML 是唯一的架构语言。

---

## 1. 设计原则 — 严格遵循 Bubble Tea MVU / ELM 架构

> **核心信条：整个 UI 只能有一种模型（tea.Model），禁止以任何借口（性能、灵活等）破坏 MVU 设计模式。**
> 组件不是独立的 tea.Model，而是 rootModel 的内部状态分段，通过统一的 Update(Msg) 接口变更。

```
┌──────────────────────────────────────────────────┐
│                 七条核心原则                       │
├──────────────────────────────────────────────────┤
│                                                   │
│  ① 单一 Model 原则                                │
│     rootModel 是整个 TUI 中唯一的 tea.Model       │
│     子组件是普通 struct，不是 tea.Model            │
│     禁止子组件拥有自己的 tea.Program               │
│                                                   │
│  ② 数据与行为分离                                 │
│     Data = 纯 struct（无方法 / 仅只读 getter）    │
│     Behavior = Update(Data, Msg) → (NewData, Cmd) │
│                                                   │
│  ③ 唯一状态变更入口                               │
│     所有状态修改必须经过 Update 函数               │
│     禁止 setter，禁止外部直接写字段                │
│     禁止从 goroutine 直接调组件方法                │
│                                                   │
│  ④ 组件契约统一                                   │
│     每个子组件实现同一接口：                       │
│     Update(Msg) → (Self, Cmd)                     │
│     View() → string                               │
│                                                   │
│  ⑤ 无 exception 原则                              │
│     任何情况下不绕过上述原则                      │
│     不存在"这里性能特殊所以用 setter"的例外        │
│     如果发现模式被打破，修复模式而非添加例外        │
│                                                   │
│  ⑥ 设计即代码                                     │
│     代码必须严格按照本设计文档实现                │
│     §1-§12 中的每一条规则都对应具体的代码约束      │
│     不允许出现"设计这样写但我换个写法"的情况      │
│     如果代码无法匹配设计，修改代码而非修改设计      │
│                                                   │
│  ⑦ 优先内置组件                                   │
│     界面优先采用 Bubble Tea / Bubbles 提供的组件   │
│     例如：textarea、viewport、list、table 等        │
│     在未找到对应内置组件时才选择手写实现            │
│     避免重复造轮子，保持一致性                       │
└──────────────────────────────────────────────────┘
```

---

## 2. 架构总览 — 三层模型

```
┌───────────────────────────────────────────────────────────┐
│                     ASSEMBLY LAYER                        │
│                   rootModel (唯一 tea.Model)               │
│                                                           │
│  职责：组合子组件 + 消息分发 + 跨组件协调                  │
│  不包含：业务逻辑、数据突变                                │
├───────────────────────────────────────────────────────────┤
│                     COMPONENT LAYER                        │
│                                                           │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────┐  │
│  │  StatusBar   │  │ ConversationPanel │  │  InputArea   │  │
│  │  (展示型)    │  │  (容器型)        │  │  (交互型)    │  │
│  └──────────────┘  └──────────────────┘  └──────────────┘  │
│  ┌──────────────────┐  ┌──────────────┐                     │
│  │ NotificationBar  │  │ ChoicesPanel │                     │
│  │  (浮动型)        │  │  (模态型)    │                     │
│  └──────────────────┘  └──────────────┘                     │
├───────────────────────────────────────────────────────────┤
│                      DATA LAYER                           │
│                                                           │
│  AnswerData │ ActionStep │ SessionMeta │ AgentInfo        │
│  WelcomeData │ ChatSession │ ...                          │
│                                                           │
│  纯 struct，无方法，只描述状态                             │
├───────────────────────────────────────────────────────────┤
│                      MESSAGE LAYER                        │
│                                                           │
│  强类型 tea.Msg，按领域分组                                │
│  AgentEvent │ UIEvent │ SystemEvent                        │
└───────────────────────────────────────────────────────────┘
```

---

## 3. 数据层 — 纯数据结构

所有 Data 是纯 struct，不包含任何修改自身的方法。
如果要修改，由 Component 的 Update 函数操作。

### 3.1 AnswerData — 一次会话应答单元

```
┌──────────────────────────────────────────────────┐
│                  AnswerData                       │
├──────────────────────────────────────────────────┤
│ + SessionID:     string                           │
│ + AgentName:     string                           │
│ + UserQuestion:  string                           │
│ + ThinkingLog:   []ThinkingRound                  │  ← 已完成的思考轮次
│ + PendingThink:  string                           │  ← 当前流式思考缓冲区
│ + Actions:       []ActionStep                     │  ← 工具调用历史
│ + Results:       []ResultEntry                    │  ← 最终回答
│ + CreatedAt:     time.Time                        │
│ + UpdatedAt:     time.Time                        │
│ + Duration:      time.Duration                    │
│ + ThinkCollapsed bool                             │  ← 思考区折叠/展开
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│                 ThinkingRound                     │
├──────────────────────────────────────────────────┤
│ + Content:    string                             │  ← 该轮累积的思想流内容
│ + TokensIn:   int                                │  ← 输入 Tokens 数
│ + TokensOut:  int                                │  ← 输出 Tokens 数
│ + Timestamp:  time.Time                          │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│                  ActionStep                       │
├──────────────────────────────────────────────────┤
│ + ToolName:     string                           │
│ + Status:       ActionStatus                     │  ← enum: Executing/Done/Failed
│ + EstimatedTok: int                              │
│ + Params:       map[string]any                   │
│ + ProgressText: string                           │
│ + ResultText:   string                           │
│ + Collapsed:    bool                             │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│                 ResultEntry                       │
├──────────────────────────────────────────────────┤
│ + Role:    string                                │  ← "result" / "error" / "typed"
│ + Content: string                                │
└──────────────────────────────────────────────────┘

AnswerStatus (enum):
  ┌─────────────┐
  │ Thinking    │  ← Agent 正在思考中
  │ Executing   │  ← Agent 正在执行工具
  │ Responding  │  ← Agent 正在生成最终回答
  │ Done        │  ← 本轮完成
  │ Error       │  ← 本轮出错
  └─────────────┘
```

### 3.2 SessionMeta — 会话元数据

```
┌──────────────────────────────────────────────────┐
│                 SessionMeta                       │
├──────────────────────────────────────────────────┤
│ + SessionID: string                              │
│ + AgentName: string                              │
│ + CreatedAt: time.Time                           │
│ + Answers:   []AnswerData                        │  ← 该会话的所有应答
└──────────────────────────────────────────────────┘
```

### 3.3 AgentInfo — 智能体信息

```
┌──────────────────────────────────────────────────┐
│                  AgentInfo                        │
├──────────────────────────────────────────────────┤
│ + Name:        string                            │
│ + Role:        string                            │
│ + Description: string                            │
│ + Model:       string                            │
│ + IsDefault:   bool                              │
└──────────────────────────────────────────────────┘
```

### 3.4 WelcomeData — 欢迎面板

```
┌──────────────────────────────────────────────────┐
│                 WelcomeData                       │
├──────────────────────────────────────────────────┤
│ + AppTitle:   string                             │  ← 应用标题（默认: "MindX CLI v2.0.0"）
│ + Version:    string                             │  ← 版本号（预留字段）
│ + AgentName:  string                             │  ← 当前 Agent 名称
│ + Workspace:  string                             │  ← 工作目录路径
│ + SessionID:  string                             │  ← 会话 ID
│ + ProjectDir: string                             │  ← 项目目录路径
│ + ModelName: string                              │  ← 模型名称
└──────────────────────────────────────────────────┘
```

### 3.5 数据关系

```
rootModel
  ├── ConversationPanel
  │     ├── Answers:  []AnswerData          ← 全部会话的应答
  │     ├── Viewport 状态（不可序列化）
  │     ├── SearchState                     ← 当前搜索匹配位置
  │     └── WelcomeShown bool
  ├── StatusBar
  │     ├── ConnectionState, TokenStats, Cost
  │     ├── AgentMeta, ModeLabel
  │     └── ShortcutHints                    ← 静态配置
  ├── InputArea
  │     ├── TextBuffer, CursorPos
  │     └── SuggestionState                  ← 临时交互状态
  ├── NotificationBar
  │     └── Notification[]                   ← 临时 UI 状态
  └── ChoicesPanel
        ├── Options, Selection
        └── Visible/Hidden                   ← 临时交互状态
```

---

### 3.6 Notification — 通知

```
┌──────────────────────────────────────────────────┐
│                   Notification                     │
├──────────────────────────────────────────────────┤
│ + ID:        string                              │
│ + Level:     NotificationLevel                   │  ← enum: Info / Success / Error / Warning
│ + Message:   string                              │
│ + CreatedAt: time.Time                           │
│ + Duration:  time.Duration                       │  ← 0 = 手动关闭，>0 = 自动超时
└──────────────────────────────────────────────────┘
```

### 3.7 ConnectionState — 连接状态枚举

```
ConnectionState (enum):
  ┌──────────────┐
  │ Disconnected │  ← 未连接
  │ Connecting   │  ← 正在连接
  │ Authenticated│  ← 已认证
  │ Connected    │  ← 已连接
  └──────────────┘
```

### 3.8 Shortcut — 快捷键

```
┌──────────────────────────────────────────────────┐
│                   Shortcut                         │
├──────────────────────────────────────────────────┤
│ + Key:         string                            │  ← 快捷键名称（如 "Ctrl+O"）
│ + Description: string                            │  ← 功能描述
└──────────────────────────────────────────────────┘
```

### 3.9 SearchState — 搜索匹配状态

```
┌──────────────────────────────────────────────────┐
│                  SearchState                       │
├──────────────────────────────────────────────────┤
│ + Query:        string                           │  ← 当前搜索文本
│ + CurrentIndex: int                              │  ← 当前匹配位置（从 0 开始）
│ + TotalMatches: int                              │  ← 总匹配数
└──────────────────────────────────────────────────┘
```

---

## 4. 消息层 — 强类型消息

所有事件分为三类，每类下细分具体消息类型。
不再有任何 `string contentType` 的 switch。

### 4.1 AgentEvent — Agent 运行时事件

来自 goroutine 的异步事件，由 `consumeEvents` 转发到 `outputCh`。

```
┌────────────────────────────────────────────────────────────┐
│                    AgentEvent 消息族                         │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ThinkingDelta    { SessionID, Content }                    │
│    └─ 一次流式思考 chunk 到达                               │
│                                                            │
│  ThinkingDone     { SessionID }                             │
│    └─ 当前轮思考完成                                        │
│                                                            │
│  ActionStart      { SessionID, ToolName, EstimatedTok,      │
│                     Params }                                │
│    └─ 工具开始执行                                          │
│                                                            │
│  ActionProgress   { SessionID, ToolName, Progress }         │
│    └─ 工具执行进度更新                                      │
│                                                            │
│  ActionResult     { SessionID, ToolName, Success,           │
│                     Result, Error }                         │
│    └─ 工具执行完成/失败                                     │
│                                                            │
│  FinalAnswer      { SessionID, Content }                    │
│    └─ Agent 生成最终回答                                    │
│                                                            │
│  AgentError       { SessionID, Error }                      │
│    └─ Agent 运行出错                                        │
│                                                            │
│  SessionDone      { SessionID }                             │
│    └─ 会话所有事件完成                                      │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 4.2 UIEvent — 用户交互事件

来自组件内部，由用户操作触发。

```
┌────────────────────────────────────────────────────────────┐
│                     UIEvent 消息族                          │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  UserSend         { Text }                                 │
│    └─ 用户按 Enter 发送消息                                │
│                                                            │
│  AgentSwitch      { AgentName }                            │
│    └─ 用户在输入框 @ 选择 agent                            │
│                                                            │
│  SlashCommand     { Name, Args }                           │
│    └─ 用户输入 / 命令                                      │
│                                                            │
│  CollapseToggle   { AnswerIndex, ActionIndex }             │
│    └─ 用户切换工具输出折叠/展开                              │
│                                                            │
│  ThinkCollapse    { AnswerIndex }                          │
│    └─ 用户切换思考区折叠/展开                                │
│                                                            │
│  ClearScreen      {}                                       │
│    └─ 用户按 Ctrl+L 清屏                                   │
│                                                            │
│  Exit             {}                                       │
│    └─ 用户按 Ctrl+C 退出                                   │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 4.3 SystemEvent — 系统内部事件

```
┌────────────────────────────────────────────────────────────┐
│                   SystemEvent 消息族                        │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Tick               {}                                     │
│    └─ 定时器 tick，驱动闪烁动画                            │
│                                                            │
│  ChoiceSelected     { Index }                              │
│    └─ 用户在 ChoicesPanel 中选择一项                       │
│                                                            │
│  NotifTimeout       { ID }                                 │
│    └─ 通知自动消失超时触发                                 │
│                                                            │
│  SessionLoaded      { AgentName, SessionID }               │
│    └─ 启动时从文件加载会话完成                              │
│                                                            │
│  WindowResize       { Width, Height }                      │
│    └─ 终端窗口大小变化（tea 原生 WindowSizeMsg 的包装）    │
│                                                            │
│  ShowChoices        { Options, Prompt }                    │
│    └─ 请求用户从选项列表中选择一项                         │
│                                                            │
│  MouseScroll        { Lines }                              │
│    └─ 鼠标滚轮滚动                                       │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

---

## 5. 显示规范 — 视觉渲染规约

> 这是界面的最终视觉规范，所有组件必须遵守。
> 文档中 `---Title---` 格式的行仅用于说明区块划分，不是实际输出内容。

### 5.1 整体版面布局

```
● 用户的问题内容                                      ← 用户问题


● 用户让我总结之前的搜索结果，                         ← ⚡ 白蓝闪烁动画
   我要分析电商行业的影响...
● 第二轮思考时的思想流                                 ← 多轮思考追加
● 第三轮思考时的思想流                                 ← 多轮思考追加

                                                        ← 区块间空行
⏺ Bash(go test ...) | 预计消耗 10K Tokens              ← ⚡ 白绿闪烁动画
  ⎿ === RUN   TestMouseClickRestoresFocus     ← 默认折叠
     === RUN   TestMouseClickRestoresFocus/...
    … +30 lines (ctrl+o to expand)

⏺ Bash(sed -n ...) | 预计消耗 10K Tokens              ← ✅ 绿色固定
  ⎿ func (m *rootModel) routeToAnswer(...) {  ← 展开
     … +12 lines (ctrl+o to expand)

                                                        ← 区块间空行
⏺ 回答的第一行内容                                     ← 白色图标
 这里是正文

                                                        ← 空行
── 消息2 ──                                             ← Transcript 模式下的分割线

⏺ 回答...
```

### 5.2 区块划分

版面分为三个自然区块，区块之间以空行分隔：

| 区块           | 视觉标识                           | 说明                   |
| -------------- | ---------------------------------- | ---------------------- |
| **思考区**     | 用户输入 → 第一个 ActionStart 之间 | 流式思考内容，直通显示 |
| **工具调用区** | ActionStart → FinalAnswer 之间     | 工具执行记录，支持折叠 |
| **最终回答区** | FinalAnswer 及之后                 | 只返回一个最终结果     |

> 注意：区块划分是**自然区隔**，不以 `---思考区---` 等标注行作为标题。
> 区块之间用空行隔开，用户通过图标和上下文自然区分。

### 5.3 图标与颜色规范

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    图标 / 颜色 / 动画对照表                             │
├──────────────┬────────┬──────────┬──────────┬───────────────────────────┤
│  元素        │ 图标   │ 颜色     │ 动画     │ 状态                      │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  用户问题    │ ●      │ 青色     │ 无       │ 固定的用户消息            │
│              │        │ #4FC3F7  │          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  思考等待    │ ●      │ 白蓝交替 │ ⚡ 闪烁  │ PendingThink 为空时       │
│              │        │ #E0E0E0  │          │                           │
│              │        │ ↔ #4FC3F7│          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  思考内容    │ ●      │ 蓝色     │ ⚡ 图标  │ PendingThink 有内容时     │
│              │        │ #4FC3F7  │ 闪烁     │ 内容稳定，图标闪烁        │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  已完成思考  │ ●      │ 蓝色     │ 无       │ ThinkingRound 已归档       │
│              │        │ #4FC3F7  │          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  工具执行中  │ ⏺     │ 白绿交替 │ ⚡ 闪烁  │ Status = Executing         │
│              │        │ #E0E0E0  │          │                           │
│              │        │ ↔ #4CAF50│          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  工具完成    │ ⏺     │ 绿色     │ 无       │ Status = Done              │
│              │        │ #4CAF50  │          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  工具失败    │ ⏺     │ 红色     │ 无       │ Status = Failed            │
│              │        │ #CF6679  │          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  最终回答    │ ⏺     │ 白色     │ 无       │ 固定的最终结果            │
│              │        │ #E0E0E0  │          │                           │
├──────────────┼────────┼──────────┼──────────┼───────────────────────────┤
│  错误        │ ⏺     │ 红色     │ 无       │ 独立的错误条目            │
│              │        │ #CF6679  │          │                           │
└──────────────┴────────┴──────────┴──────────┴───────────────────────────┘
```

### 5.4 区块详情 — 思考区

```
布局:
  ● 深度思考中                    ← 等待第一个 Delta 时
  ● 用户让我总结之前的搜索结果，  ← Delta 到达后实时替换等待文字
     我要分析电商行业的影响...     ← 内容直通，不做任何处理
     … +N lines (ctrl+o to expand)← 默认折叠，仅显示最近三行
     [Tokens: 1,234 in / 5,678 out]
  ● 第二轮思考的内容              ← 多轮：依次追加
     … +N lines (ctrl+o to expand)
     [Tokens: 789 in / 4,321 out]
  ● 第三轮...                     ← 正在流式输出的当前轮次
  ● 仍继续...

规则:
  • 内容 100% 流式直通：Agent 输出什么就显示什么
  • 不做 JSON 解析，不做格式化（如 "reasoning"/"decision" 等字段原文显示）
  • 多轮思考：每轮完整保留，按顺序追加显示
  • 等待第一个 Delta 时显示 "深度思考中"，图标白蓝闪烁
  • Delta 到达后：内容替换等待文字，图标继续闪烁
  • 思考完成（ThinkingDone）后：图标停止闪烁，内容保留
  • 思考内容以斜体灰色显示（Italic, #888888）
  • 思考区结束于第一个 ActionStart 事件
  • 默认折叠：每轮思考内容默认仅显示最近三行
  • 超过三行的部分以 "… +N lines (ctrl+o to expand)" 省略
  • Ctrl+O 切换展开/折叠：展开后显示该轮全部内容
  • 展开/折叠状态由 AnswerData.ThinkingCollapsed 控制
  • 当前正在流式输出的轮次（isThinking=true）始终完全展开
  • 历史已完成的轮次（ThinkingRound）默认折叠
  • 每轮思考完成后，在内容底部追加一行 Token 统计：
    `[Tokens: {TokensIn} in / {TokensOut} out]`
    数字带千分位分隔，颜色为暗灰色（#666666），不加粗

状态机:
  WaitFirstDelta ──ThinkingDelta──→ Streaming (当前轮完全展开)
  Streaming ──ThinkingDone──→ RoundDone (归档后变为折叠)
  RoundDone ──ThinkingDelta──→ Streaming (下一轮，完全展开)
  RoundDone ──ActionStart──→ End (思考区关闭)
```

### 5.5 区块详情 — 工具调用区

```
布局:
  ⏺ ToolName(param: value) | 预计消耗 N Tokens     ← 执行中
    ⎿ 完成 (N lines)                                 ← 折叠状态
  ⏺ ToolName(param: value) | 预计消耗 N Tokens     ← 完成
    ⎿ 输出第一行                                       ← 展开状态
      输出第二行
      … +N lines (ctrl+o to expand)
  ⏺ ToolName | failed: 错误信息                      ← 失败
  ⏺ ToolName | 进度文本                               ← 有进度信息

规则:
  • 工具按执行顺序排列，每个工具占一行（含参数预览）
  • 执行中：图标白绿闪烁，显示参数和 Token 预估
  • 完成：图标固定绿色，默认折叠输出结果
  • 失败：图标红色，显示失败原因
  • 折叠状态显示 "完成 (N lines)" 或 "失败: 错误信息"
  • 展开状态显示前三行内容 + "… +N lines (ctrl+o to expand)"
  • Ctrl+O 切换折叠/展开
  • 展开/折叠状态记录在 ActionStep.Collapsed 中
  • 工具区结束于 FinalAnswer 事件

格式:
  ⏺ToolName(param_str) | 预计消耗 N Tokens | ProgressText     ← 执行中
  ⏺ToolName | 预计消耗 N Tokens | [+] Show output             ← 完成折叠
  ⏺ToolName | 预计消耗 N Tokens | output_preview  [−] Hide   ← 完成展开
  ⏺ToolName | failed: errorText                                ← 失败
```

### 5.6 区块详情 — 最终回答区

```
布局:
  ⏺ 回答的第一行内容
  这里是正文内容
  可以有多行 Markdown

规则:
  • 每个 AnswerData 只有一个最终回答区块
  • 无论经过几轮思考、几个工具调用，最终只有一个 ResultEntry
  • 回答内容以 Markdown 渲染
  • 图标白色固定，不闪烁
  • 回答区在 FinalAnswer 事件到达后创建并追加

关键约束:
  • 不检查是否有重复的 tool result（当前代码的 HasToolResult 逻辑删除）
  • 不解析 FinalAnswer 内容中的 JSON 字段
  • 不做 "reasoning" 字段提取——那是思考区的职责
  • FinalAnswer 就是最终回答，直接渲染
```

### 5.8 动画规则

| 动画           | 触发条件                                    | 行为                | 停止条件                           |
| -------------- | ------------------------------------------- | ------------------- | ---------------------------------- |
| 思考区图标闪烁 | PendingThink 为空                           | 白 ↔ 蓝，500ms 交替 | 第一个 Delta 到达                  |
| 思考区图标闪烁 | PendingThink 非空                           | 白 ↔ 蓝，500ms 交替 | ThinkingDone                       |
| 工具图标闪烁   | ActionStep.Status = Executing               | 白 ↔ 绿，500ms 交替 | ActionStep.Status 变为 Done/Failed |
| 所有动画停止   | ThinkingDone + 所有 ActionStep 非 Executing | —                   | 不再产生 Tick Cmd                  |

### 5.9 InputArea 与建议区渲染规范

InputBox 有**三种渲染状态**：普通输入模式 → 命令建议模式 → Agent 建议模式。
普通输入模式始终存在，建议模式在输入特定前缀时触发并在输入区下方追加建议面板。

---

#### 5.9.0 NormalInput — 普通输入

这是 InputBox 的基础渲染形态，三种模式共享相同的输入行布局。

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                              NormalInput — 普通输入布局                                 │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│  ────────────────────────────────────────────────────────────────────────────────       │
│  ❯ 你的消息...                                                                         │
│  ────────────────────────────────────────────────────────────────────────────────       │
│                                                                                        │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

规则:
  • 输入区始终位于屏幕最底部，宽度自动撑满终端
  • 输入行上下各有一条全宽分割线，与终端等宽
  • 分割线使用 `─` 字符连续绘制，左右不留空白
  • 提示符以 `❯ ` 开头，后跟用户输入文本和闪烁光标
  • 普通输入模式不显示任何建议面板

---

#### 5.9.1 CommandSuggestion — 命令建议

在 NormalInput 布局基础上，下分割线下方追加命令建议面板。

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                          CommandSuggestion — 命令建议布局                               │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│  ────────────────────────────────────────────────────────────────────────────────       │
│  ❯ /                                                                                   │
│  ────────────────────────────────────────────────────────────────────────────────       │
│                                                                                        │
│  /init          Initialize a new CLAUDE.md file with codebase documentation            │
│  /batch         Research and plan a large-scale change... (bundled)                    │
│  /pr-comments   Get comments from a GitHub pull request                                │
│  /add-dir       Add a new working directory                                            │
│  /agents        Manage agent configurations                                            │
│  /branch        Create a branch of the current conversation at this point              │
│                                                                                        │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

规则:
  • **基座布局**：继承 NormalInput（全宽分割线 + ❯提示行 + 全宽分割线）
  • **触发条件**：当输入以 `/` 开头时，进入命令建议模式
  • **建议面板**：位于下分割线下方，每条建议占一行
  • **建议行格式**：`/命令名   描述`
    - 命令名左对齐
    - 描述左对齐于固定列
    - 当前选中的匹配项高亮显示
  • **实时过滤**：建议列表随输入实时更新匹配
  • **退出条件**：当输入不以 `/` 开头时，回到 NormalInput

---

#### 5.9.2 AgentSuggestion — Agent 建议

在 NormalInput 布局基础上，下分割线下方追加 Agent 建议面板。

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                          AgentSuggestion — Agent 建议布局                               │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│  ────────────────────────────────────────────────────────────────────────────────       │
│  ❯ @                                                                                   │
│  ─────────────────────────────────────────────────────────────────────────────────────  │
│                                                                                        │
│  @architect     Expert in system design and architecture                               │
│  @developer     Full-stack software engineer                                           │
│  @reviewer      Code review specialist                                                 │
│                                                                                        │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

规则:
  • **基座布局**：继承 NormalInput（全宽分割线 + ❯提示行 + 全宽分割线）
  • **触发条件**：当输入以 `@` 开头时，进入 Agent 建议模式
  • **建议面板**：位于下分割线下方，每条建议占一行
  • **建议行格式**：`@agent名   描述`
    - Agent 名左对齐
    - 角色/描述左对齐于固定列
    - 当前选中的匹配项高亮显示
  • **实时过滤**：建议列表随输入实时更新匹配
  • **退出条件**：当输入不以 `@` 开头时，回到 NormalInput

### 5.10 Welcome 欢迎面板

首次启动时显示在 ConversationPanel 中，仅在会话开始时展示一次。
采用**垂直居中布局**：渐变标题 + 状态信息列表 + 底部提示。

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                                  Welcome — 欢迎面板布局                                   │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                          │
│                         MindX CLI v2.0.0                                                 │
│                                                                                          │
│                    Workspace: /projects/myapp                                            │
│                    Session: a1b2c3d4                                                     │
│                    Agent: architect                                                      │
│                    Model: claude-sonnet-4-20250514                                       │
│                                                                                          │
│  ────────────────────────────────────────────────────────────────────────────────────── │
│                                                                                          │
│   ℹ Type a message to start chatting                                                    │
│                                                                                          │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

规则:
  • 欢迎面板仅在应用启动、无已有应答时展示
  • 显示后不再重复出现（当 Answers 非空时自动隐藏）
  • 采用**垂直居中布局**，从上到下依次为：标题、状态信息、分割线、提示

标题区:
  • 使用 **渐变色彩渲染** `AppTitle` 文本（默认: "MindX CLI v2.0.0"）
  • 渐变配色方案（从左到右）:
    - 蓝色系: #42A5F5 → #1E88E5 → #1976D2 → #1565C0 → #0D47A1
    - 粉色系: #EC407A → #D81B60 → #C2185B → #AD1457 → #880E4F
  • 每个字符使用对应位置的渐变色渲染，加粗显示
  • 如果 AppTitle 为空，使用默认值 "MindX CLI v2.0.0 Beta"

状态信息区:
  • 位于标题下方，垂直排列
  • 仅在字段非空时显示对应的行
  • 格式: **`{Label}: {Value}`**
  - 第一行（可选）: `Workspace: {path}` — 标签白色加粗，值白色
  - 第二行（可选）: `Session: {id}` — 标签白色加粗，值白色
  - 第三行（可选）: `Agent: {name}` — 标签白色加粗，值白色
  - 第四行（可选）: `Model: {modelName}` — 标签白色加粗，值白色

底部提示:
  • 状态信息下方以一条全宽分割线（`─`）隔开
  • 分割线使用默认颜色
  • 提示文本以 `ℹ` 图标开头，灰色渲染
  • 固定内容: "Type a message to start聊天"

数据填充流程 (populateWelcome):
  1. 设置基础信息:
     - AppTitle = "MindX CLI v2.0.0"
     - ModelName = "unknown"
  2. 从 SessionMeta 填充:
     - Workspace = sessionMeta.GetProjectDir()
     - SessionID = sessionMeta.SessionID
  3. 从 MasterAgent 填充（如果可用）:
     - AgentName = masterAgent.Name()
     - ModelName = masterAgent.Model().Name
     - SessionID = masterAgent.SessionID()（覆盖步骤 2 的值）

```
┌───────────────────────────────────────────────┐
│              Component 契约                     │
├───────────────────────────────────────────────┤
│                                               │
│  Init()   → Cmd          (初始化 & 启动副作)  │
│  Update(Msg) → (Self, Cmd)  (状态变更入口)    │
│  View()   → string       (渲染输出)           │
│                                               │
│  关键约束：                                     │
│  • 不暴露公共 setter                           │
│  • 不直接修改其他组件的状态                     │
│  • 状态仅通过 Update 改变                      │
│                                               │
└───────────────────────────────────────────────┘
```

### 6.1 ConversationPanel — 容器型组件

```
┌──────────────────────────────────────────────────────────────┐
│                     ConversationPanel                        │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 管理所有 AnswerData 的增删改                              │
│  • 管理 viewport 滚动                                       │
│  • Markdown 渲染（glamour）                                  │
│  • 搜索匹配高亮                                              │
│  • 严格遵守 §5 显示规范进行 View 渲染                       │
│                                                              │
│  数据持有                                                   │
│  ────────                                                   │
│  - Answers:     []AnswerData           ← 全部应答            │
│  - Viewport     (bubbles/viewport)     ← 滚动区域           │
│  - Glamour      (TermRenderer)         ← Markdown 渲染器    │
│  - SearchState  (当前搜索位置)                               │
│  - WelcomeShown bool                                         │
│  - BlinkOn      bool                  ← Tick 驱动交替       │
│                                                              │
│  订阅消息                                                   │
│  ────────                                                   │
│  ThinkingDelta → 追加到当前 Answer 的 PendingThink          │
│                  详见 §5.4 思考区渲染规则                     │
│  ThinkingDone  → PendingThink 归档为 ThinkingRound           │
│                  图标停止闪烁                                 │
│  ActionStart   → 追加 ActionStep                            │
│                  返回 Tick Cmd 启动闪烁                       │
│  ActionProgress→ 更新最近的 ActionStep.ProgressText          │
│  ActionResult  → 更新最近的 ActionStep.Status 和 ResultText  │
│                  详见 §5.5 工具调用区渲染规则                 │
│  FinalAnswer   → 追加 ResultEntry                           │
│                  详见 §5.6 最终回答区渲染规则                 │
│  AgentError    → 追加 error ResultEntry                      │
│  SessionDone   → 标记 Answer 完成 清除所有动画               │
│  Tick          → 切换 BlinkOn 状态                           │
│                  如果仍有需要动画的状态则返回下一个 Tick Cmd  │
│                  详见 §5.8 动画规则                           │
│  WindowResize  → 调整 viewport 尺寸                          │
  │  CollapseToggle→ 切换 ActionStep.Collapsed (工具折叠)        │
│                  详见 §5.5 折叠/展开规则                      │
│  ThinkCollapse → 切换 AnswerData.ThinkCollapsed (思考折叠)   │
│                  详见 §5.4 思考区折叠规则                     │
│  ClearScreen   → 清空 Answers                                │
│                                                              │
│  不处理的消息                                               │
│  ────────────                                               │
│  UserSend, AgentSwitch — InputArea 专属                      │
│  NotifTimeout — NotificationBar 专属                         │
│  ChoiceSelected — ChoicesPanel 专属                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 6.2 StatusBar — 展示型组件

```
┌──────────────────────────────────────────────────────────────┐
│                       StatusBar                              │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 第一行：连接状态 + Token 用量 + 费用 + Agent/Model      │
│  • 第二行：快捷键提示（可选）                               │
│                                                              │
│  数据持有                                                   │
│  ────────                                                   │
│  - Width       int                                           │
│  - ConnState   ConnectionState                               │
│  - SessionName string                                        │
│  - TokensIn    int                                           │
│  - TokensOut   int                                           │
│  - TokensTotal int                                           │
│  - SessionCost string                                        │
│  - AgentName   string                                        │
│  - ModelName   string                                        │
│  - ModeLabel   string                                        │
│  - ShowHints   bool                                          │
│  - Shortcuts   []Shortcut                                    │
│                                                              │
│  订阅消息                                                   │
│  ────────                                                   │
│  WindowResize → 更新 Width + ConnState                      │
│  ActionStart  → 累计 TokensIn 估计消耗                       │
│  AgentSwitch  → 更新 AgentName                              │
│  SessionLoaded → 设置 AgentName + SessionName               │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 6.3 InputArea — 交互型组件

```
┌──────────────────────────────────────────────────────────────┐
│                        InputArea                             │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 管理文本输入（textarea）                                  │
│  • @ agent 建议列表                                         │
│  • / command 建议列表                                       │
│  • Enter 发送 / Alt+Enter 换行                              │
│  • Ctrl+C 退出 / Ctrl+L 清屏                                │
│  • 粘贴处理                                                  │
│                                                              │
│  数据持有                                                   │
│  ────────                                                   │
│  - Textarea      (bubbles/textarea)                          │
│  - AgentSuggest  (AgentSuggestion 子组件)                    │
│  - CmdSuggest    (CommandSuggestion 子组件)                  │
│  - Hidden        bool                                        │
│  - Agents        []AgentInfo                                 │
│                                                              │
│  产生消息                                                   │
│  ────────                                                   │
│  UserSend      — Enter 发送                                  │
│  AgentSwitch   — @agent 补全选中                            │
│  SlashCommand  — /command 执行                               │
│  Exit          — Ctrl+C                                      │
│  ClearScreen   — Ctrl+L                                      │
│                                                              │
│  订阅消息                                                   │
│  ────────                                                   │
│  WindowResize → 更新 textarea 宽度                           │
│  KeyPressMsg  → 键盘按键处理                                │
│  PasteMsg     → 粘贴文本                                     │
│  SuggestionComplete → 补全文本插入                           │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 6.4 NotificationBar — 浮动型组件

```
┌──────────────────────────────────────────────────────────────┐
│                     NotificationBar                          │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 浮动通知（info/success/error/warning）                    │
│  • 自动消失（duration > 0）                                  │
│  • 手动关闭（duration = 0）                                  │
│                                                              │
│  数据持有                                                   │
│  ────────                                                   │
│  - Notifications: []Notification                             │
│    - ID, Level, Message, CreatedAt, Duration                 │
│  - MaxVisible: int                                           │
│  - Width: int                                                │
│                                                              │
│  订阅消息                                                   │
│  ────────                                                   │
│  NotifTimeout    → 移除超时通知                              │
│  WindowResize    → 更新宽度                                  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 6.5 ChoicesPanel — 模态型组件

```
┌──────────────────────────────────────────────────────────────┐
│                       ChoicesPanel                           │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 显示可选项列表（服务器要求用户选择时）                     │
│  • Enter 确认选择 / Esc 取消                                │
│                                                              │
│  数据持有                                                   │
│  ────────                                                   │
│  - Visible: bool                                             │
│  - Items:   []ChoiceItem                                     │
│  - Prompt:  string                                           │
│  - ListModel (bubbles/list)                                  │
│                                                              │
│  产生消息                                                   │
│  ────────                                                   │
│  ChoiceSelected { Index }  — 用户选择一项                    │
│                                                              │
│  订阅消息                                                   │
│  ────────                                                   │
│  ShowChoices  → 显示面板 + 设置选项列表                     │
│  KeyPressMsg  → Enter/Esc 处理                              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## 7. 组装层 — rootModel

rootModel 是唯一的 `tea.Model`，负责组合所有子组件。

```
┌──────────────────────────────────────────────────────────────┐
│                        rootModel                             │
├──────────────────────────────────────────────────────────────┤
│  职责                                                       │
│  ────                                                       │
│  • 持有所有子组件实例                                        │
│  • 在 Update 中按消息类型分发到对应子组件                    │
│  • 跨组件协调（如 WindowResize 需要通知所有组件）            │
│  • 初始化 App + 会话                                         │
│  • 启动 consumeEvents goroutine                              │
│                                                              │
│  组合关系                                                   │
│  ────────                                                   │
│  rootModel                                                    │
│    ├── conversation: *ConversationPanel                      │
│    ├── statusBar: *StatusBar                                 │
│    ├── input: *InputArea                                     │
│    ├── notifBar: *NotificationBar                            │
│    ├── choices: *ChoicesPanel                                │
│    │                                                         │
│    ├── app: *core.App                          ← 外部依赖   │
│    ├── chatManager: *chatSessionManager        ← 持久化     │
│    ├── registry: *SlashCommandRegistry         ← 命令注册   │
│    └── (goroutine 相关)                                       │
│        ├── outputCh: chan tea.Msg                            │
│        ├── currentCancel: context.CancelFunc                 │
│        └── executing: bool                                   │
│                                                              │
│  Update 消息分发逻辑（伪代码）                                │
│  ────────────────────────                                    │
│  Update(msg):                                                │
│    switch msg:                                               │
│      AgentEvent:        conversation.Update(msg)             │
│      WindowResize:      statusBar.Update(msg)                │
│                          conversation.Update(msg)             │
│                          input.Update(msg)                    │
│                          notifBar.Update(msg)                 │
│      UserSend:          handleSend(msg)                      │
│      Exit:              save + tea.Quit                      │
│                         同时分发 statusBar/input 状态同步     │
│      [其他消息]:        找到对应组件 → component.Update(msg) │
│                                                              │
│  关键：                                                       │
│  • 不包含业务逻辑                                          │
│  • 不直接修改子组件内部数据                                │
│  • 只做"这个Msg该给谁"的转发                              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 消息分发矩阵

| 消息类型       | StatusBar | Conversation | InputArea | NotifBar | Choices |
| -------------- | --------- | ------------ | --------- | -------- | ------- |
| ThinkingDelta  |           | ●            |           |          |         |
| ThinkingDone   |           | ●            |           |          |         |
| ActionStart    | ●         | ●            |           |          |         |
| ActionProgress |           | ●            |           |          |         |
| ActionResult   |           | ●            |           |          |         |
| FinalAnswer    |           | ●            |           |          |         |
| AgentError     |           | ●            |           |          |         |
| SessionDone    |           | ●            |           |          |         |
| UserSend       |           |              |           |          |         |
| AgentSwitch    | ●         |              | ●         |          |         |
| CollapseToggle |           | ●            |           |          |         |
| ThinkCollapse  |           | ●            |           |          |         |
| WindowResize   | ●         | ●            | ●         | ●        |         |
| Tick           |           | ●            |           |          |         |
| ChoiceSelected |           |              |           |          | ●       |
| NotifTimeout   |           |              |           | ●        |         |
| Exit           |           |              |           |          |         |

---

## 8. 数据流

### 8.1 主消息流 — 用户发送消息到 Agent 应答

```
User
  │
  ▼
InputArea.HandleKey(Enter)
  │
  ├── 产生 UserSend{Text}
  │
  ▼
rootModel.Update(UserSend)
  │
  ├── 1. conversation.CreateAnswer(sessionID, agentName)
  ├── 2. goroutine: agent.Ask(sessionID, text)
  ├── 3. goroutine: consumeEvents(eventCh, sessionID)
  │
  ▼
agent.Ask() → eventCh(ReactEvent)
  │
  ▼
consumeEvents goroutine
  │  for event := range eventCh:
  │    switch event.Type:
  │      ThinkingDelta → outputCh ← ThinkingDeltaMsg
  │      ActionStart   → outputCh ← ActionStartMsg
  │      ActionResult  → outputCh ← ActionResultMsg
  │      FinalAnswer   → outputCh ← FinalAnswerMsg
  │      Error         → outputCh ← AgentErrorMsg
  │  after loop → outputCh ← SessionDoneMsg
  │
  ▼
rootModel.Update(ThinkingDeltaMsg)
  │
  ├── conversation.Update(ThinkingDeltaMsg)
  │     └── 在当前 Answer 的 PendingThink 中追加 Content
  │
  ▼ (循环反复)
  │
  rootModel.Update(SessionDoneMsg)
  │
  ├── conversation.Update(SessionDoneMsg)
  │     └── 标记 Answer 为 Done
  ├── executing = false
```

### 8.2 闪烁动画流

```
ConversationPanel 在 ActionStart/ThinkingDelta 时
  返回 Cmd: Tick
      │
      ▼
  rootModel 收到 Tick 消息
      │
      ├── conversation.Update(Tick)
      │     └── 切换 BlinkOn 状态
      ├── 如果仍有需要动画的状态，返回下一个 Tick Cmd
      │
      ▼
  conversation.View() 根据 BlinkOn 交替闪烁图标

动画规则详见 §5.8：
  • 思考区：BlinkOn 交替 白色 ↔ 蓝色 (#E0E0E0 ↔ #4FC3F7)
  • 工具区：BlinkOn 交替 白色 ↔ 绿色 (#E0E0E0 ↔ #4CAF50)
  • 停止条件：ThinkingDone + 所有 ActionStep 非 Executing
```

### 8.3 窗口尺寸变化流

```
tea.WindowSizeMsg
  │
  ▼
rootModel.Update(WindowSizeMsg)
  │
  ├── statusBar.Update(WindowResize)   → 更新 Width + ConnState
  ├── conversation.Update(WindowResize)→ 更新 viewport 尺寸
  ├── input.Update(WindowResize)       → 更新 textarea 宽度
  └── notifBar.Update(WindowResize)    → 更新通知栏宽度
```

---

## 9. 与当前架构的关键差异

| 维度             | 当前架构 (V1)                              | 新架构 (V2)                                |
| ---------------- | ------------------------------------------ | ------------------------------------------ |
| **组件接口**     | 4+ 种不同签名                              | 统一 `Update(Msg) → (Self, Cmd)`           |
| **状态变更**     | setter + Update 双路径                     | 仅通过 Update                              |
| **事件路由**     | `routeToAnswer` switch string              | 强类型消息直接分发                         |
| **Answer**       | 既是数据又是组件                           | 纯数据 AnswerData + ConversationPanel 管理 |
| **goroutine→UI** | 调 answer.SetXxx() + trySend               | 仅 trySend(msg)                            |
| **消息类型**     | `agentAnswerUpdateMsg{contentType string}` | 细分到具体语义的消息                       |
| **新增事件**     | 改 switch case                             | 新建消息类型 + 新增 Update 分支            |
| **显示规范**     | 无独立文档，散布在 View 代码中             | §5 独立章节，所有组件约束统一              |
| **动画控制**     | time.Time Tick 直接转发                    | Tick 经 ConversationPanel.Update 统一管理  |
| **可测试性**     | 需要 mock setter                           | `Update(msg) → assert View()`              |

---

## 10. 组件依赖关系

```
                      ┌──────────────┐
                      │  rootModel   │
                      └──────┬───────┘
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
  │ Conversation │   │  StatusBar   │   │  InputArea   │
  │    Panel     │   │              │   │              │
  └──────────────┘   └──────────────┘   └──────┬───────┘
         │                                      │
         │   依赖（纯数据，非组件）               │
         ▼                                      │
  ┌──────────────┐                   ┌──────────┬──────────┬──────┐
  │  AnswerData  │  ← 纯 struct      ▼          ▼          ▼
  │  ActionStep  │  ← 纯 struct  ┌────────┐ ┌────┐ ┌──────┐
  │  ResultEntry │  ← 纯 struct  │Agent   │ │Cmd │ │Text  │
  └──────────────┘               │Suggest │ │Sugg│ │area  │
                                 │ion     │ │est │ │      │
                                 └────────┘ └────┘ └──────┘
                                 (子组件，非独立)
```

---

## 11. 目录结构建议

```
internal/client/
├── client.go              # rootModel + NewProgram
├── types.go               # 所有消息类型定义
│
├── data/
│   ├── answer.go          # AnswerData, ActionStep, ResultEntry, ThinkingRound
│   ├── agent.go           # AgentInfo
│   └── session.go         # SessionMeta, ChatSession
│
├── component/
│   ├── conv/
│   │   ├── panel.go       # ConversationPanel (Update/View)
│   │   ├── welcome.go     # 欢迎面板渲染
│   │   └── viewmode.go    # 辅助渲染函数（ResultEntry 等）
│   │
│   ├── statusbar/
│   │   └── statusbar.go   # StatusBar (Update/View)
│   │
│   ├── input/
│   │   ├── input.go       # InputArea (Update/View)
│   │   ├── agent_suggest.go  # AgentSuggestion (子组件)
│   │   └── cmd_suggest.go    # CommandSuggestion (子组件)
│   │
│   ├── notify/
│   │   └── notify.go      # NotificationBar (Update/View)
│   │
│   └── choices/
│       └── choices.go     # ChoicesPanel (Update/View)
│
├── render/
│   ├── table.go           # 表格渲染器
│   ├── todo.go            # 待办渲染器
│   └── markdown.go        # Markdown 渲染器
│
├── command/
│   ├── registry.go        # SlashCommandRegistry
│   └── builtin.go         # BuiltinCommands
│
├── session/
│   └── manager.go         # ChatSessionManager (持久化)
│
├── style/
│   ├── theme.go           # 主题色
│   └── style.go           # 组件样式
│
├── filter.go              # 工具函数
```

