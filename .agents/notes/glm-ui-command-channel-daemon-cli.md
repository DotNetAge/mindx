# Agent-Driven UI 命令通道：mindx 侧落点（2026-10-01）

背景与全局设计见 mindx-work/.agents/notes/glm-architecture-agent-driven-ui-command-channel.md（主文档）。

## 本次落地（ui 三命令 + 三事件）

- `internal/svc/handler_ui.go`：ui.open / ui.open_link / ui.run 三 RPC handler，`broadcastUI` 统一广播（envelope 对齐 permission_request 旁路 {type, data}，gw nil 静默跳过）。
- `internal/svc/handler_registry.go`：注册三方法。
- `cmd/ui.go`：cobra 子命令 `mindx ui open|open-link|run`；`resolveWorkspacePath` 工作区闸（filepath.Rel 越界拒绝，防 `..` 与同前缀伪装）；`callUI` 受理即返回。
- `pkg/rpc/ui.go`：UIOpenParams / UIOpenLinkParams / UIRunParams + typed Client 方法（对齐 skill.go 先例）。
- 测试：cmd/ui_test.go（路径闸五例）、internal/svc/handler_ui_test.go（受理语义：缺参报错 / 过闸受理 / gw nil 不炸）。

## 经验

- 参数闸放 CLI 侧而非 daemon：CLI 进程 cwd 就是 Agent 工作区，daemon 是多会话中枢不该知道单个会话的工作目录；daemon 信任已过闸参数只做广播。
- BroadcastNotification(method, params) 的 params 即客户端收到的整个 envelope；字符串 method 直接成为客户端订阅名（无需 gateway.ResponseType 枚举）。
- 临时 daemon 端到端验证会写 ~/.mindx/logs，沙箱内跑不了——端到端验证依赖正式 daemon 重启。

## 工具化落点（2026-10-01 追加：Open/Visit/TerminalRun 默认工具）

- `internal/tools/ui_tools.go`：Open（file_open）/ Visit（link_open）/ TerminalRun（terminal_run）三 FuncTool，构造注入 `UIBroadcast` 回调；工作区闸在会话 ProjectDir（`tools.GetToolContext(ctx).Session.ProjectDir()`，对齐 goharness fs 工具纪律），Visit 仅 http/https，TerminalRun cwd 默认 ProjectDir 可相对覆盖。受理语义与 CLI 版一致。
- `internal/core/app.go`：`uiBroadcast` 字段 + `SetUIBroadcast`（Daemon 注入）；createRuntime 剥离 TeamCreate/TeamDelete/TeamList/TeamGetTasks（ToolRegistry().Remove，对齐 Ls/Read 替换先例），广播回调就位时注册三工具（TUI 无 daemon 不注册）。
- `internal/svc/daemon.go`：NewDaemon 里 `app.SetUIBroadcast(d.broadcastUI)`。
- prompt_builder.go experienceSystemNote 移除「调用 TeamList 工具」引用（剥离后系统提示不得再指向不存在工具）；prompt_builder_test 加反向断言护栏。
- 客户端 toolmap/builder 的 TeamXXX 归一映射保留（展示层防御代码，无害）。

## include_tools 声明装配（2026-10-01 追加：TeamXXX opt-in）

TeamXXX 默认剥离但可按 Agent 声明装配回来（用户拍板）：

- `agentstore/meta.go`：AgentMeta 加 `IncludeTools []string`（IDENTITY.md frontmatter `include_tools`）；与 exclude_tools 对称——exclude 裁默认在场的工具（导出层过滤，goharness WithExcludeTools），include 唤回默认不在场的工具（注册层物理剥离的豁免）。
- `agentstore/store.go`：`IncludeToolsOf(name)` 查询（对齐 ExcludeToolsOf）。
- `app.go` createRuntime：剥离循环前构建 include 集合，命中条目跳过 Remove 并打「按声明装配」日志。
- 语义要点：goharness 的 exclude_tools 是导出时过滤（注册表仍在），无法表达 opt-in；所以「默认剥离」必须在 mindx 侧 Remove，include 豁免也在同一处。声明渠道 = 手写 IDENTITY.md frontmatter（Agent 可用 fs 工具自助修改），RPC/CLI 更新链路暂不加（避免过度工程，需要时按 exclude_tools 全链路先例增量）。

## 审计结论（2026-10-01，双子代理交叉验证）

- 修复：ui run 的 `os.Getwd` 吞错改明确报错；mindx-desktop openPath 两处 void 吞错改 errMsg 提示（与 mindx-work 对齐）；web-viewer 双外壳冗余改复用同一 provide 实例。
- 证伪不报（防重复审计再报）：ui.* RPC 无参数闸——与 terminal.* 等既有 RPC 同权限级，本地信任模型知情设计（闸在 CLI 是为约束 Agent，非约束本地进程）；broadcastUI gw==nil——Start 时 initGateway 必达，运行期不可达；terminal pending 单槽覆盖——消费链 onMounted+watch 双路闭环，仅理论窄窗口。
- 跨端契约实证：三事件 data 字段（path/url/command/cwd）三端一致无 typo。

