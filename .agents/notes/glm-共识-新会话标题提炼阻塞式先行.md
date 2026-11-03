# 新会话标题提炼：阻塞式先行与补录的协作语义

- 分类：共识 / 设计定稿
- 日期：2026-11-03
- 涉及：mindx internal/svc/session_title.go、internal/core/prompts.go、daemon.go defaultHandler、pkg/session 补录

## 定稿语义（2026-11-03 用户定案）

新会话首条提问 → **阻塞式先行**：先用独立 LLM 调用（`core.NewCaller` + 默认模型 + `PROMPT_SESSION_TITLE`）提炼标题并 `RenameSession` 回填，完成后才继续首轮执行；模型用默认模型（不新增配置）。

## 否决记录（2026-11-03 用户定案，强信号）

- **禁止把标题提炼耦合进 session.create RPC**（用户怒斥「污染 Session」）：标题提炼与翻译（handler_translate）/提交信息（handler_commit_message）同类，是原有功能的**独立补充**，独立成文（session_title.go）、挂在消费侧（defaultHandler 首轮前），不动创建契约。
- 同理类推：凡「横切增强」类功能（提炼、摘要、翻译）都不得侵入被增强流程的协议与创建语义，只能以独立模块挂消费侧钩子。

## 关键协作事实（下次改标题相关代码必读）

- pkg/session `AppendMessage` 的补录（首条 user 消息截断 → 标题）**仅在 meta.Title 为空时生效**——提炼标题在首轮 append 之前落库，补录自然跳过；提炼失败/超时不重试，静默回退补录，两条路径天然互斥不冲突。
- 新会话门控：`GetSessionMeta` 无错且 `Title=="" && MessageCount==0` 才触发；`ErrSessionNotFound`（裸发消息未过 session.create）也尝试提炼，但 `RenameSession` 需要 会话目录已存在 且 `Create` 无法指定 ID——此场景回填失败落补录兜底。UI 全部走 session.create，不受影响。
- 超时设计：`llmcaller.Call` 内置 4 分钟超时，阻塞式先行不可等——外层 12 秒兜底，Token 用量在调用协程内即时落库（超时放弃结果不丢用量），模式对齐 handler_commit_message（agent 名 `session_title`）。
- goharness `SessionStore` 接口**没有** RenameSession/GetSessionMeta（mindx 扩展方法，RoutedSessionStore 转发 FileSessionStore）；SessDB() 返回的是具体类型，可直接调用。

## 下次我看到「标题被覆盖 / 新会话标题不对 / 想改成异步」，我将

先核对本篇的补录互斥关系与门控条件；改异步时把 maybeGenerateSessionTitle 移入首轮完成后的钩子并补 sessions 变更广播（前端 TasksSection 订阅 agents_changed 同款模式）。
