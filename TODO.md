# TODO

## 单元测试补缺（覆盖率规划 2026-11-15，基线：go test -cover 全量，svc 包 17.9%；P0 后 25.2%；P1 svc handler 批量补齐后 42.6%）

- [ ] scheduled_exec.go runBackgroundTurn 全链路（排队取消/真实轮次执行）与 handler_device.go 剩余分支（device.scan mDNS、互信凭据领取、handshake 外部令牌路径）、mail 收发正向路径需集成夹具（真实 Runtime/模型/市场服务端），单元层已覆盖可达分支；svc 现状：handler_device 49.4%、handler_mail 61.9%、mail_dispatcher 30%、scheduled_exec 32.9%；另 internal/commands 的 handleAgents/Models/Skills/JobList/Compact 成功路径与 handleHelp 同因（需真实 gateway Server + WS 客户端才能命中 RespondWithType/ctx.Server()），包覆盖 74.8%；P1 svc handler 补齐后新增集成账：fetchSiliconFlowPricing/fetchRate*（价格页与汇率 URL 硬编码，进程缓存分支已覆盖）、wireAskEvents/AskBuilder（需完整 agent 运行时）、handler_fs.go handleFSReveal 成功路径（真弹 Finder）
- [ ] i18n 缺陷待决策：系统 locale 为 en_US.UTF-8 时 ResolveTag 落回简体中文而非英文（locale.go matchTag 只特判中文，base "en" 匹配失败）——需用户拍板英文 locale 用户是否应得英文 UI；修复后同步更新 i18n_test.go 中标记「已知缺陷」的断言。已顺手修复：LanguageName 对 zh-Hant 标签误显示「简体中文」（设置页语言列表），已补 zh-Hant/zh-MO 前缀判断并测试
- [ ] P1 MCP 子系统：internal/mcp manager.go（0%）/handler.go/rpc.go/tool.go/client.go（6.2%）；可参照 pool/config 既有测试套路
- [ ] P1 基础设施：internal/core workspace.go（12.5% ExtractWorkspace/SyncRuntimeAssets，runtime 铁律执行者）、llmcaller、changes、file_version、bootstrap；pkg/session file_token_usage_store.go（0%）、migrate、routed_store
- [ ] 明确不强补：internal/client TUI 组件、internal/setup wizard、cmd/ 薄壳、pkg/rpc 声明层、examples/、appicon（低 ROI，规划内不做）

- [ ] 壳侧采纳 terminal.start 的 session_id 参数：daemon 已支持 cwd 解析顺序 = 显式 cwd > 会话 project_dir > daemon cwd，但壳不传 session_id 时仍落 daemon 进程 cwd（launchd 下通常是 /）——mindx-work / mindx-desktop 打开终端时带上当前会话 ID
- [ ] 会话标题提炼 + 轮词元统计已落地待实机验收：`make restart`（吃进 goharness v0.2.54 的 Usage 回填）后 ①新建会话发首问看侧栏标题为提炼标题；②跑一次未绑定会话的调度任务看任务会话标题；③对话含工具调用的轮次末尾出现词元统计行；④重新加载该历史会话，统计行仍在（恢复链路读 token_usage）。注意 make restart 会中断运行中的会话，挑空闲时机执行
- [ ] 会话绑定 agent 抢答修复待实机验收（与上条共用一次 make restart）：user.message 新增 agent_name 载荷（mindx-work 发送时携带输入框选定 agent），daemon 按「@目标 > 载荷 agent_name > 会话绑定 agent_name > 全局当前 agent」裁定执行者。验收：①与大秘聊完，AgentSelector 切到从未聊过的梁工直接发消息，应答者必须是梁工；②选中梁工会话内继续聊，应答者仍为梁工

## 市场包血统与升级链路（定案：mindx-market/docs/设计定案-市场包血统与升级链路.md）

已落地（L1 血统字段 / L2 打包自增与发布闸 / L2.5 按版本下载 / L3 升级 RPC 与 CLI / L4 市场货架可更新态 / L5 存量纳管 adopt + Agent 工具 BundleUpgrade），验证见 .agents/notes/assistant-事实-市场包升级链路落地与两道闸.md。以下是**明确留下的边界**，不是未做完：

- [ ] 团队升级粒度：当前是整队一个决定——任一成员被改过，整队走「保留 / 覆盖」。逐成员跳过（未改的成员照升、改过的单个跳过）需要在 `bundle.Install` 增加 per-member skip 语义，属安装器能力扩展；在「不做三方合并」的前提下，整队覆盖是保守正确的一侧（不会留下半新半旧的团队），要细粒度再单独立项
- [ ] 存量纳管的判据边界：`bundle.adopt` 只在「定义文件能对上某一版」时写版本号，对不上就不写（认不出来）；社区无同名包的本地自建包按 `1.0.0` 起版。已知取舍：①技能包内副本不参与比对（技能有自己的版本链），只改过 Agent 级技能副本不会被判为「改过」；②自建包起版 1.0.0 后若上架、且社区已有同号不同内容，服务端会按「版本号终身绑定」返回 409，届时需发 1.0.1
- [ ] 服务端发版顺序（已知事实，留作发布检查项）：按版本下载端点在 market-server 里，必须先部署服务端再发客户端；服务端未升级时 `DownloadVersion` 404 → 内容闸降级为「无法判定」→ 升级要求用户确认，不会静默误覆盖
