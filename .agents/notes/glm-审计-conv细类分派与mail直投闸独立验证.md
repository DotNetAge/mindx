# 审计：conv NoticeSub 细类分派 / mail.deliver 直投闸 / terminalCwd 独立验证（第二人）

## 待提取事实

1. **MaxTurnsNode 计数零值是既有缺陷非本轮回归**：`MaxTurnsReachedMsg`（msg/types.go:129-134）有 TurnsCompleted/MaxTurns 字段，但 `Item` 无结构化计数通道，只折进 Text（stream.go:334-341）；构建新分支只设 Suggestion（builder.go:690-704）→ registry.go:254-257 显示 "0/0 轮"。git HEAD 旧逻辑（启发分支+默认 fallback）的 MaxTurnsNode 同样零字段，旧行为更差（badge 恒空）。badge 是本轮净改善。
2. **Item 是"纯文本+细类"通道**：cancelled 分支同构——`CancelledNode.Elapsed` 未解析（builder.go:706-717），registry.go:296-301 badge 永不显示，同为既有形态。若后续要让系统卡显示结构化数据，Item 需加字段而不是从 Text 反解。
3. **Status 在 content 渲染路径无消费点**：renderContentNode（render_tree.go:306-333）只区分 NodeExecuting/IsFinal/其他；NodeFailed 与 NodeSuccess 无渲染差异。subtask_fail 用 NodeSuccess 无可见错误，但属语义债——未来按 Status 过滤/统计 content 时失败态会失真。旧版把 subtask_fail 误标 Kind=max_turns（"已达最大轮数"），本轮实为修复。
4. **历史还原不产生 itemNotice**：StreamsFromMessages（stream.go:625-708）只还原 user/assistant/tool 三种 role；builder.go:675 注释"历史还原走文本启发兜底"与实际不符——实时流全带 NoticeSub，启发兜底目前无真实消费场景（纯防御）。
5. **mail.deliver 闸 fail-closed 是仓库既有惯例而非异类**：`devGate == nil ||` 先例在 handler_device.go:244（device.trust.ticket）与 handler_network.go:333（network.token.get）。语义区分：guard 的 deviceGateAuthorized（handler_device.go:61-66）fail-open 适用于连接准入闸；本机身份核验闸必须 fail-closed（放行=回到无核验状态，任何连接可伪造 from_persona 投信）。mail.deliver 全仓库无客户端调用方，收信唯一通道是 mailLoopOnce 轮询（mail_dispatcher.go:22,52-75）。
6. **terminalCwd 不校验目录存在性**（handler_terminal.go:241-251 + 96-97）：session project_dir 已删/移动时 cmd.Dir 指向失效路径，pty 启动直接报错，无回退 daemon cwd。非回归（旧版不认 session_id），新增功能边界缺口。
7. **测试遗漏**：terminalCwd 优先级、LocalVerified 闸、buildSystemNode 细类分派/兜底均无单测（conv/svc 的 *_test.go 零命中）；go build / go vet 通过。

## 下次我看到

- 「Item 只传 Text 导致节点卡结构化字段为零」→ 先查 git HEAD 确认是回归还是既有，再决定 severity；倾向在 Item 加结构化字段而非从展示文本反解。
- 「某闸与 guard 的 fail-open 惯例不一致」→ 先区分准入闸与身份核验闸：身份核验闸 fail-closed 才是正确姿势，查 handler_device.go:244 先例。

## 修复定案（2026-11-03 审计后同日修复，build/vet/测试全绿）

- 发现1 已修：Item 增 TurnsCompleted/MaxTurns 结构化字段，max_turns 发点与构建器透传，「0/0 轮」消除（不从展示文本反解）。
- 发现2 已修：subtask_fail 单独置 NodeFailed（渲染层当前无差异，纯语义矫正）。
- 发现4 已修：terminalCwd 对 project_dir 做 os.Stat 存在性校验，失效回退 daemon cwd。
- 发现5 已修：新增 handler_terminal_test.go（terminalCwd 三分支）、handler_mail_test.go（闸拒绝）、builder_system_test.go（分派表 + 兜底）。
- 发现3 维持现状：fail-closed 有 handler_device.go:244 / handler_network.go:333 同类先例，属本机身份核验闸的正确取向；二期 mail.deliver 重接线时须对齐降级语义并实测本机链路。
- 参考事实：renderContentNode 对 NodeFailed/NodeSuccess 无渲染差异（render_tree.go:306-333）；历史还原路径 StreamsFromMessages 不产生 itemNotice，文本启发兜底为纯防御代码。
