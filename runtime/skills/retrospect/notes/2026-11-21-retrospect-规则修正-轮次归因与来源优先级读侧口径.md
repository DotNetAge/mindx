---
version: "2.6.1"
pub_date: 2026-11-21
modifier: "Ray"
---

## 2026-11-21 修正轮次归因与来源优先级的读侧口径（retrospect）

- 问题：上一版（2.6.0）把「判定只能落最新一轮」当作已知限制写进 SKILL.md，让使用者以为素材只能靠 note 正文判断、不能信任 `task_hint`。实际排查发现根因是写入时序而非「取最新一条」的策略选择：round 事件在整轮结束的 defer 里才落盘，Agent 却在轮内上报，所以判定必然错位一轮，新会话首轮更是直接丢失。同时 `LatestJudgment` 只比时间戳、来源不参与比较，与白皮书 7.4「用户反馈 > Agent 上报」相悖——一条晚到的 Agent 上报能抹掉用户当时的判定，而自省正是靠这个字段判断「哪次真失败了」。
- 否决：给 `metric.record` 加 `round_no` 参数让调用方指定轮次。否决理由：轮内上报时目标轮次的事件**还不存在**，参数无从落地；且要求 Agent 自己知道当前轮号，把内部时序细节外泄成 API 契约。另否决读侧继续按时间取最新以「与当时写侧一致」——写侧本就该改，让读侧跟随错误口径只会把偏差固化。
- 修正：写侧改为轮内暂存（`pendingJudgments`）、轮末 `recordRoundEvent` 落盘后 `flushPendingJudgments` 挂载；`round.rating` 共用同一机制。`JudgmentRecord` 生效判定改为两级比较（来源层级优先、同级比时间），`rework_trace` 与 `agent_report` 同级以保留「回溯覆盖乐观 success」的语义。读侧 `metric_scan.py` 的 `latest_judgment` 加 `SOURCE_RANK` 与之同口径。SKILL.md 步骤 2.3 第 6 条两条子项改写：`task_hint` 现在可信任（轮次内上报与判定同轮），并新增来源优先级的判读规则。version 2.6.0 → 2.6.1。
- 验证：三处修正各做变异验证——删除 `flushPendingJudgments` 调用后 `TestRecordRoundEventMountsStashedJudgment` 报「flush 未接线」失败；`judgmentBeats` 退回纯时间比较后优先级测试三项失败；关闭轮内暂存后归因测试报「判定丢失」。端到端探针走真实 store 验证判定精确落在 Agent 所处那一轮、`task_hint` 与 note 同轮对齐。读侧用构造数据验证晚到 `agent_report` 不再顶掉 `user_feedback`。全量 `go test ./...` 绿（`internal/discovery` 的 mDNS 广播测试在本机沙箱环境本就失败，与本次改动无关，未触及该包）。
- 踩坑：抽 `recordRoundEvent` 之前，测试只能手工调 `flushPendingJudgments`，那只能证明挂载函数本身正确，证明不了它被接上——把生产代码里的那行调用删掉，手工版测试照样全绿。凡「A 事件应触发 B 动作」的因果，测试必须走真实的触发路径，否则测的是复制品不是产品。