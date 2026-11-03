# user.message 无视会话绑定 agent，全局当前 agent 抢答

- 分类：事故 / 根因
- 日期：2026-11-03
- 现象：mindx-work 输入框选定梁工发消息，应答者是大秘（截图实证，应答卡片 meta.agent_name=大秘）

## 根因链（实证）

- 客户端 `user.message` 协议只有 `{text, session_id, images}`，无 agent 字段（mindx-work store.ts notifyUserMessage 与 daemon defaultHandler payload 双侧核对一致）
- agent 选择经 `session.create {agent}` 绑进会话 meta.AgentName，但 daemon defaultHandler 旧逻辑只认文本 `@目标` 前缀，否则 `ResolveRuntime("") → CurrentRuntime()` = 全局当前 agent（LastAgent）——从不读会话绑定
- `selectAgent` 无命中时 clearActiveStream + 懒建会话，会话创建时 agent 绑定正确，缺口纯在 daemon 执行侧

## 修复与裁定顺序

defaultHandler：会话先于 agent 定案（先 resolveSessionID 再 ResolveRuntime），执行 agent = `@目标 > 载荷 agent_name（客户端显式选择）> GetSessionMeta(sessionID).AgentName > 全局当前 agent`。payload 新增 `agent_name` 字段，mindx-work 发送时随消息携带输入框选定的 agent（chatflow store.ts notifyUserMessage）。

用户点破的残余坑：只靠会话绑定 + 全局兜底，跨 agent 切换且该 agent 从未聊过（无会话）时仍会抖回大秘——显式信号（@目标 / 载荷 agent_name）优先于一切隐式回退才是根治。只改 daemon 一处 + 客户端发送载荷，全部客户端统一生效。

其它发送侧查证（无需改动）：
- mindx CLI（internal/client）：RPC 路径发送前自动给文本加 `@当前agent` 前缀（client.go:1260-1266，wire-only 不影响显示），已走 daemon 裁定的最高优先级信号；发送前强制 EnsureSession（client.go:1279），会话归属正确。
- goharness SubAgent：身份链全显式——父会话工具调用显式指定 req.AgentName + agentExists 校验，会话复用按 AgentName/ProjectDir/Sponsor 三元匹配，「身份由 SessionID 决定，agentName 从元数据恢复」（subagent.go:172），不经 daemon defaultHandler，无抖动面。

## 下次我看到「消息发给了非选定的 agent / 执行者与会话归属不一致」，我将

先查 daemon 侧执行 agent 的裁定链（@目标 / 载荷 agent_name / 会话 meta / 全局回退），再查客户端是否显式携带选定 agent；隐式回退（CurrentAgent/LastAgent/会话归属）永远会让跨 agent 场景抖动，显式信号优先才是根治。
