# PR：主回合等待子代理 —— 可见等待、阻断冒泡、自动收集

> 跨仓库变更：`core/goharness`（运行时）、`mindx`（daemon 接线）、`mindx-desktop`（前端）。
> 本文按文件落实到具体改动，并在文末给出逐项可行性复核。

## 背景（事故结论摘要）

2026-09-20 10:32 主会话派发 4 个 SubAgent 后，主 Agent 依 [tools/subagent.go:78-85](../../../core/goharness/tools/subagent.go) 的异步契约直接结束回合（日志 `exec finished reason=completed answer_len=1838`，10:33:02）。此后：

1. 子代理事件经 `parentEmit` 转发进**主 exec 的 EventBus**（[event_setup.go:31-42](../../../core/goharness/agents/event_setup.go)），exec 退出即注销唯一订阅者（cleanup）→ 子代理全部后续 delta、`SubtaskCompleted`（[tools/subagent.go:250-261](../../../core/goharness/tools/subagent.go)）**静默丢弃**；
2. 用户看到子会话冻结在"一两步"、无加载态、无最终答案（实际 3/4 子代理已完成并落盘交付物）；
3. 用户紧急停机 → daemon 停机路径（[daemon.go:411-441](../internal/svc/daemon.go)）**未调用 cancelSubAgents**，data-analyst 被进程死亡杀死，无终止标记，重启后 CollectResults 对其轮询将死等 30 分钟。

## 目标行为（已与用户定案）

```
主 Agent 派发 SubAgent × N（同一响应并行）
        │
        ▼
主回合不收尾，进入「等待子代理」阶段（主树可见等待状态）
        ├── 子代理全过程事件持续可见（事件订阅随回合存活）
        ├── 子代理 AskUser / Permission 冒泡到当前界面作答/授权
        │     （Permission 通路已存在；AskUser 需补镜像通路）
        ▼
全部子任务落定 → 自动收集结果注入主循环 → 主 Agent 总结收尾
```

- **保留** SubAgent 异步并行语义（一轮多个并行 spawn）；
- **"等"从"LLM 自觉 / 用户手催"上移到回合生命周期内**，不做事后唤醒死回合。

---

## 工作块 A：SubAgent 契约改为「派发后立即阻塞等待」（核心，最小改动）

| 文件 | 位置 | 改动 |
|---|---|---|
| `core/goharness/tools/subagent.go` | L78-85 Prompt 契约 | 改为：同一响应并行派发全部 SubAgent 后，**必须立即调用 `CollectResults(session_ids...)` 阻塞等待全部落定再继续**；禁止在子任务未收集时结束回合回答用户 |
| `core/goharness/tools/collect_results.go` | L111-187 | 无需改动：已剥离单次工具 deadline、保留取消信号，支持长阻塞（事故中 10:38:51→10:41:19 阻塞 2.5 分钟无超时，已实测） |

**依据**：exec 在工具调用期间保持存活 → [event_setup.go](../../../core/goharness/agents/event_setup.go) 的转发订阅存活 → 子代理 delta 与完成事件全程可达前端。主树已有 CollectResults 节点"正在收集结果"流光态（前端 `registry/summary.ts:131-145` + `NodeCard.vue:50-70`），**等待状态 UI 零新增**。

**残余风险**：依赖 LLM 遵从 Prompt。由工作块 B 做确定性兜底。

## 工作块 B：harness 兜底自动等待（确定性保证）

LLM 若仍未调 CollectResults 就产出无工具调用的收尾响应，在 finalize 前拦截。

| 文件 | 位置 | 改动 |
|---|---|---|
| `core/goharness/agents/ask.go` | L31-45 AskBuilder 字段区 | 新增可选字段 `subagentWaitHook func(ctx, sessionID) (collected string, ok bool)`，与 `permissionCh`/`permissionSink` 同一注入模式 |
| `core/goharness/agents/subagent.go` | L313-398 管理器；L400 `registerSponsored` | 新增 `ActiveSponsored(sponsorSessionID) []string`：读 `sponsored` 映射（该映射按 sponsor 会话 ID 索引强停句柄，正是所需视图；勿用 `doneChs`，它全局按子会话 ID 索引），返回该主会话名下仍在运行的子会话 ID 列表 |
| `core/goharness/agents/runtime.go` | L281-332 工具注册/Ask 构建处 | 构建 AskBuilder 时注入 `subagentWaitHook`（内部调 `waitCompletions` + 收集各子会话结果，带上限 deadline 30m，超时返回部分结果并注明） |
| `core/goharness/agents/executor.go` | L893-925 `finalizeAnswer` 调用点（"LLM 未调用工具"分支） | finalize 之前检查：`subagentWaitHook` 命中（有活跃子任务）→ **跳过 finalize**，阻塞等待 → 把收集结果以 user 角色消息注入（复用 [executor.go:877-888](../../../core/goharness/agents/executor.go) `imgMsg` 视觉消息的注入先例）→ `continue` 循环让 LLM 下一迭代总结 |
| `core/goharness/agents/events`（事件类型定义处） | — | 新增 `SubagentWaitStarted` / `SubagentWaitEnded` 事件，经 EventBus → mindx `client/rpc.go` 转发 → 前端回合等待徽标 |

**关键结构验证**（已核实）：`finalizeAnswer` 注释明确"LLM 未调用工具时收尾、调用方在调用后立即 return"，即它在迭代循环体内被调用——在调用点前拦截并 `continue` 结构上成立，不影响 permission_pending / ask_user_pending 分支（[executor.go:48-55](../../../core/goharness/agents/executor.go)、L500-534、L573-586）。

**边界处理**：
- 用户点停止 → `message.cancel`（[handler_interact.go:36-61](../internal/svc/handler_interact.go)）级联 `failDone` 关闭通道 → `waitCompletions` 立即返回失败列表 → 循环继续 → LLM 按失败结果收尾；
- hook 等待超时（30m）→ 返回部分结果 + 注明超时 → 循环继续，LLM 决定收尾或重派；
- 子代理自身 exec 也会命中 hook（它也可能派孙代理）——语义一致，无需特判。

## 工作块 C：子代理 AskUser 冒泡（镜像 permissionSink 模式）

Permission 通路**已存在且经事故验证**：子代理授权请求经 `permissionSink` 直达前端、不依赖父 exec 存活（[permission.go:121-148](../../../core/goharness/agents/permission.go)、[daemon.go:721-745](../internal/svc/daemon.go) `WithPermissionSink` 广播 `RespPermissionRequest`；前端 [PermissionBar.vue:61-78](../../mindx-desktop/src/renderer/src/components/chat/PermissionBar.vue) 的 grant/deny payload 携带 `session_id` 精确路由回挂起子会话）。AskUser 补齐同款：

| 文件 | 位置 | 改动 |
|---|---|---|
| `core/goharness/agents/ask.go` | L31-45 字段区 | 新增 `askSink func(data AskPendingData) (answer string, ok bool)` 与 `askCh chan …`（镜像 permissionCh 挂起/恢复） |
| `core/goharness/agents/executor.go` | L573-586 AskUser 分支 | **仅当子代理**（有 askSink 注入）时：经 askSink 直达前端并阻塞等待回答，不再以 `ask_user_pending` 结束子 exec；主会话自身 AskUser 行为不变（维持现有 pending 结束 + 恢复机制） |
| `core/goharness/agents/subagent.go` | L570-594 spawn 注入点 | 随 `permissionSink` 一并注入 `askSink` |
| `mindx/internal/svc/daemon.go` | L721-745 旁 | 新增 `WithAskSink`：把子代理提问广播为 `RespAskUserRequest`（形态复用 PermissionPendingData），携带子会话 `session_id`、agent 名、问题文本与选项 |
| `mindx/internal/client/rpc.go` | L407-435 附近 | 新增 `RespAskUserRequest` → 前端消息转换 |
| `mindx-desktop …/components/chat/` | AskUser Drawer（按 PR-REFACTOR-THINKING-LOOP 决策记录：激活时 ChatArea 底部 Drawer 强制作答） | 渲染子代理提问卡片；作答 payload 携带 `session_id`，走与主会话 AskUser 应答相同的 RPC（按 session_id 路由到挂起子 exec 恢复） |

**注意**：子会话 Tab 内的直接作答路径保留不变（冒泡是新增入口，二者写同一恢复通道）。

## 工作块 D：daemon 停机安全网

| 文件 | 位置 | 改动 |
|---|---|---|
| `mindx/internal/svc/daemon.go` | L411-441 `stopBackgroundServices` | 在 gateway 关闭**之前**，遍历活跃 sponsor 会话调用与 [handler_interact.go:36-61](../internal/svc/handler_interact.go) 相同的 `cancelSubAgents` 级联：子 exec 以 cancelled 结束 → [subagent.go](../../../core/goharness/agents/subagent.go) spawn 的 defer 写入 `[sub-agent-terminated]` 标记 |

**效果**：异常停机后子会话留有终止标记，重启后 CollectResults 轮询立即判 `failed` 而非死等 30 分钟；主会话收到的 CollectResults 结果中子任务状态明确。

## 工作块 E：前端 —— 主树内联全程直播（已定案）

**目标**：子代理的思考/工具调用/工具结果像主 Agent 一样，以嵌套节点形式直播在主树的子代理节点下方（默认执行中展开、完成后折叠为摘要），而非只能去子会话 Tab 看。

**事件可达性**（由工作块 A/B 保证）：主回合存续期间，子代理 delta（带 `session_id`=子会话 ID、agent 名）全程到达前端。事故已证实该流在前 25 秒内是通的（用户看到的"一两步"即为直播），断的只是链路生命周期。

| 文件 | 位置 | 改动 |
|---|---|---|
| `mindx-desktop …/stores/chatStore.ts` | delta 分发入口；L1911-1975 `handleSubtaskSpawned`（已建立主会话↔子会话 sponsor 关系） | delta 事件的 `session_id` 命中本会话活跃子会话集合时，除照旧写入 `messagesBySession[子会话ID]`（子会话 Tab 保留）外，同步转发树构建器（携带父会话 ID + 子会话 ID + agent 名） |
| `mindx-desktop …/tree/builder/index.ts` | L721-849 `upsertSubagentFromTool`（已有 session_id→节点映射）；L240-250 事件归一化入口 | 新增子代理嵌套流处理：按（父会话, session_id）建独立 groupBuffer，**复用现有节点归一化逻辑**生成思考/工具/内容节点，挂接到对应 SubagentNode 的 children |
| `mindx-desktop …/tree/nodes/subagent/SubagentNodeView.vue` | L31-44（现仅状态行+摘要） | 新增可展开 children 渲染区：执行中默认展开跟随、完成/失败后折叠为"共 N 步 · 结果摘要"行；样式遵循现有树约定——无边框、1px 左侧引导线贯穿 header 与展开区、思考区背景色与主文本同色 |
| `mindx-desktop …/tree/types/entity.ts` | SubagentNode 定义 | 增加 `children` 容器字段（嵌套节点复用现有节点类型） |
| `mindx-desktop …/tree/builder/restore.ts` | L23-54 | 恢复态重建：子会话有持久化消息时经 `session.get(子会话ID)` 拉取历史离线重建嵌套节点（默认折叠）；若主回合仍在等待（工作块 B 的 WaitStarted 状态），恢复后继续接实时流追加 |

**性能约束**：4 路并行流全程直播的 DOM 量以"每节点紧凑单行卡片"控制（与主树节点同规格）；完成后折叠为摘要行，长会话主树不膨胀。

**现状保留**：CollectResults 阻塞期间主树"正在收集结果"节点（`summary.ts:131-145`）、PendingRow 兜底（`TreeView.vue:96-105`）、`applyCollectResults` 结果回填（chatStore.ts:1503-1553）均不动。

---

## 可行性复核（逐项）

| 工作块 | 结论 | 依据 / 剩余风险 |
|---|---|---|
| A | **可行，已实测** | CollectResults 长阻塞 2.5 分钟无超时（事故日志）；工具层已剥单次 deadline。风险仅 LLM 遵从度，由 B 兜底 |
| B | **结构可行，最大改动点** | `finalizeAnswer` 位于迭代循环体内、调用后 return 的结构已核实；消息注入有 `imgMsg` user 角色先例；`failDone`/`waitCompletions` 通道语义已核实。剩余风险：hook 阻塞期间 TokenUsage/Duration 统计口径需在实现时归入 `resultDuration`（`finalizeAnswer` L921-923 填充点不受影响） |
| C | **可行，低风险** | permission 同款通路全程存在且事故中子会话授权路由已被前端代码与注释证实（PermissionBar.vue:66-68）；仅"子代理 ask 不结束子 exec"为行为变更，需保证仅子代理分支生效，主会话语义不变 |
| D | **可行，极低风险** | `cancelSubAgents` 级联已存在（message.cancel / 断连路径在用），仅补停机调用点 |
| E | **可行** | 事件关联字段（`session_id`、agent 名）在 delta 事件与子代理节点上均已存在；嵌套渲染复用现有 groupBuffer/归一化逻辑，不改主树架构。剩余工作量集中在 SubagentNodeView 展开区与 restore 离线重建 |

**整体依赖顺序**：A（契约）→ B（兜底）→ D（安全网）为后端主线；C（AskUser 冒泡）独立可并行；E 依赖 A/B 产生的事件流，前端最后收口。

## 验证方案

1. **等待可见 + 内联直播**：主 Agent 派发 4 个耗时子代理 → 主树出现"正在收集结果"节点，各子代理节点下方实时流式追加思考/工具嵌套节点（展开跟随）→ 全程无冻结；完成后折叠为摘要行；
2. **授权冒泡**：子代理内触发需授权的 Bash → 主界面 PermissionBar 出现 → 授权后子代理继续（携带子 session_id 路由）；
3. **提问冒泡**：子代理内触发 AskUser → 主界面 Drawer 出现提问 → 作答后子代理恢复继续执行；
4. **自动兜底**：构造 LLM 派发后直接收尾的回合 → executor 自动进入等待并在落定后继续总结（日志可见 `SubagentWaitStarted/Ended`）；
5. **停机安全**：等待期间强停 daemon → 重启后子会话含 `[sub-agent-terminated]` 标记 → 重新 CollectResults 立即返回 failed 而非死等；
6. **回归**：主会话自身 AskUser/Permission 行为不变；子会话 Tab 直接作答路径不变。

## 决策记录（与用户对齐）

- 否决"死后唤醒"方案：只要主回合还会在子代理存活时退出，表现就必然怪异；改为回合内可见阻塞等待；
- 否决"CollectResults 发现已仍在执行就提前返回"：维持 `waitCompletions` 全量阻塞语义（Promise.all），不存在提前退出；
- SubAgent 并行异步语义保留；"等"上移到 harness 回合生命周期，Prompt 契约 + 兜底钩子双保险；
- daemon 停机级联强停作为崩溃安全网保留（非功能补丁）。
