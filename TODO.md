# TODO

## 单元测试补缺（覆盖率规划 2026-11-15，基线：go test -cover 全量，svc 包 17.9%）

- [ ] 修 TestHandleSessionList_FilterByAgent 测试污染：全量跑 internal/svc 时失败、单跑通过，测试间共享状态泄漏；修完验收 `go test -count=1 ./internal/svc/` 连跑三次全绿
- [ ] P0 调度链：pkg/scheduler/scheduler.go（核心循环 12 函数 0%）+ internal/svc/scheduled_exec.go（new 哨兵兜底 0%）；用例覆盖到点触发、会话落实 ""/"new" 双哨兵、执行失败路径；验收 `go test -cover ./pkg/scheduler/ ./internal/svc/` 两文件覆盖 ≥60%
- [ ] P0 安全与信件链：handler_device.go（7.5% 设备信任闸）、handler_mail.go（1%）+ mail_dispatcher.go（0%）；用例覆盖 devGate.LocalVerified 闸放行/拒绝、外接设备伪造 from_persona 被拒；验收闸门关键分支全覆盖
- [ ] P0 token 用量：handler_token_usage.go（707 行 0%）；覆盖口径聚合与来源回填
- [ ] P2 低垂果实：internal/diffutil/normalize.go、internal/i18n（bundle+locale）、internal/mcp/env_path.go、pkg/scheduler/validate.go、internal/tools/dynamic.go、internal/commands——纯函数易测；验收各包覆盖 ≥80%
- [ ] P1 svc handler 批量：handler_online_models（995 行 0%）、handler_kvstore（6.3%）、handler_fs（6.4%）、ask_handler、handler_metric_judgment、handler_mcp、handler_team、handler_rule、harvest_intent、turnlog、diff_utils、daemon.go（14.1% 初始化/停机时序）
- [ ] P1 MCP 子系统：internal/mcp manager.go（0%）/handler.go/rpc.go/tool.go/client.go（6.2%）；可参照 pool/config 既有测试套路
- [ ] P1 基础设施：internal/core workspace.go（12.5% ExtractWorkspace/SyncRuntimeAssets，runtime 铁律执行者）、llmcaller、changes、file_version、bootstrap；pkg/session file_token_usage_store.go（0%）、migrate、routed_store
- [ ] 明确不强补：internal/client TUI 组件、internal/setup wizard、cmd/ 薄壳、pkg/rpc 声明层、examples/、appicon（低 ROI，规划内不做）

- [ ] 壳侧采纳 terminal.start 的 session_id 参数：daemon 已支持 cwd 解析顺序 = 显式 cwd > 会话 project_dir > daemon cwd，但壳不传 session_id 时仍落 daemon 进程 cwd（launchd 下通常是 /）——mindx-work / mindx-desktop 打开终端时带上当前会话 ID
- [ ] 会话标题提炼 + 轮词元统计已落地待实机验收：`make restart`（吃进 goharness v0.2.54 的 Usage 回填）后 ①新建会话发首问看侧栏标题为提炼标题；②跑一次未绑定会话的调度任务看任务会话标题；③对话含工具调用的轮次末尾出现词元统计行；④重新加载该历史会话，统计行仍在（恢复链路读 token_usage）。注意 make restart 会中断运行中的会话，挑空闲时机执行
- [ ] 会话绑定 agent 抢答修复待实机验收（与上条共用一次 make restart）：user.message 新增 agent_name 载荷（mindx-work 发送时携带输入框选定 agent），daemon 按「@目标 > 载荷 agent_name > 会话绑定 agent_name > 全局当前 agent」裁定执行者。验收：①与大秘聊完，AgentSelector 切到从未聊过的梁工直接发消息，应答者必须是梁工；②选中梁工会话内继续聊，应答者仍为梁工
