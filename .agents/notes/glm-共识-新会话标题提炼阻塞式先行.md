# 新会话标题提炼：先提炼标题，后创建会话

- 分类：共识 / 实现语义
- 日期：2026-11-03
- 涉及：mindx internal/svc/session_title.go、internal/svc/daemon.go（ensureScheduleSession）、pkg/session（WithTitleOption / Create）、internal/core/prompts.go

## 权威语义（用户原话，以此为准）

当 daemon 检测到当前是用户的首问需要创建全新会话时，先以阻塞方式调用 LLM（core.NewCaller + 默认模型 + PROMPT_SESSION_TITLE）把首问提炼成会话标题，得到标题后才开始调用创建会话——标题随创建落库。用户之后手动重命名会话与标题提炼无关。

## 实现落点（2026-11-03 实证后定案）

- 真正由 daemon 创建全新会话的路径是 `ensureScheduleSession`（调度任务空 ID 且无最近会话）：先 `generateSessionTitle`（12s 兜底超时），成功则 `mindxses.WithTitleOption(title)` 随 `CreateSession` 落库；失败静默创建不带标题。add/update/叫醒入口无首问，传 `""` 不提炼。
- defaultHandler 的会话由客户端先行创建（`goharnesssession.Load` 要求会话已存在，daemon.go:893；裸发路径本就卡在 Load），故此处只处理「空壳首问」：meta 存在且 `Title=="" && MessageCount==0` → 阻塞提炼 → `RenameSession` 在首条消息 append 前写入；会话不存在直接跳过。
- `pkg/session.WithTitleOption` 是 mindx 自定义的 `goharnesssession.SessionOption`，`FileSessionStore.createSession` 把 `sessionInfo.Title` 写进 meta；RoutedSessionStore 透传 opts，无需转发方法。

## 关键事实（下次改标题相关代码必读）

- 提炼失败 / 超时 / 无默认模型 → 静默跳过，回退 pkg/session Append 的补录（首条 user 消息截断为标题，仅 Title 为空时生效），不重试。
- goharness `SessionStore` 接口没有 RenameSession/GetSessionMeta（mindx 扩展方法，RoutedSessionStore 逐个转发）；SessDB() 返回具体类型可直接调用。
- Token 用量在调用协程内即时落库（agent 名 `session_title`），外层超时放弃结果不丢用量。

## 下次我看到「会话标题 / 提炼类流程」，我将

先按用户最新口述的定序实施，禁止引用旧笔记里与口述相悖的结论当依据；改前实证会话的真实创建路径（goharness Load 要求会话已存在）。
