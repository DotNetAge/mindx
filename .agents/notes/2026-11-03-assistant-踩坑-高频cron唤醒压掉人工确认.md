# 高频 cron 唤醒压掉人工确认（T20 lane 验证现场）

- 时间：2026-11-03 22:51-22:53
- 场景：T20-lane-verify 验证「Cron 任务按 session_id + agent 精确投递到会话 lane」
- 实况：Cron `*/10 * * * * *` 每 10 秒向当前会话投递 `T20-lane-verify`。我发出 AskUser 询问如何处置该测试任务，用户界面返回的却是下一次 cron 的原文 `T20-lane-verify`——**定时唤醒的回声盖住了人的回答，确认链路被自己的噪声打断**。
- 处置：未删除，仅置 `enabled=false`（可逆），run_history 因此可从 stopped 状态复核；删除留待用户定夺。
- 判据提炼（供后续写 Cron 使用）：
  1. 自带验证/自唤醒任务周期不要低于 1 分钟，10 秒级只能用于「一次性观察落位」且必须配 one_shot；
  2. 需要用户在唤醒过程中做决策时，唤醒内容必须携带会话上下文（如「T20 lane 验证第 N 次，收到请忽略，勿回复」），否则用户回什么都会被下一次唤醒覆盖；
  3. 验证类任务优先 one_shot（scheduled_at）而非 cron 循环，天然无噪声、跑完自禁用。
- 验证结论本身：Cron → 会话 lane 的绑定投递生效，执行者未被抢答（assistant），落位 session 01M6GYA2S80V5S2STHC3K1DGSG 正确。
