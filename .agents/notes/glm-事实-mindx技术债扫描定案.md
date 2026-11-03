# mindx 技术债扫描定案（2026-11-03）

- 分类：事实 / 待办线索
- 扫描范围：mindx 全部非 test .go，三维度：自认降级/TODO 注释、未实现与 v2 挂账、静默降级与死代码（方法论与 core/goharness 扫描同源，见 core/goharness/.agents/notes/glm-事实-goharness技术债扫描定案.md）

## 真债（三项均已于 2026-11-03 修复，build/vet/测试全绿）

1. **终端打开不落会话项目目录**：internal/svc/handler_terminal.go——已修：terminal.start 增加可选 `session_id` 参数，cwd 解析顺序 = 显式 cwd > 会话 project_dir（terminalCwd 经 SessDB().GetSessionMeta）> daemon 进程 cwd（launchd 下通常是 /，仅兜底）。daemon 无法自行知道「当前会话」，正确行为需壳侧调用时传 session_id（跨仓库采纳项）。
2. **mail.deliver「一期停用」口径失真**：handler_mail.go handleMailDeliver——已修：增加 devGate.LocalVerified 闸（对齐 handleDeviceTrustTicket 先例），外接设备凭握手连接权伪造 from_persona 投信的路径被拒；三处注释口径改为「仅限本机直通连接」。
3. **TUI 系统事件 Kind 占位**：conv/builder.go buildSystemNode + stream.go——实证后发现比注释更重：LLM 超时、子任务启动/失败通知全部误渲染为「已达最大轮数」。已修：Item 增加 NoticeSub 细类（max_turns/cancelled/timeout/subtask_spawn/subtask_fail）由五个发点打标，构建器精确分派（超时→ErrorNode llm_timeout、子任务→content 原文直渲）；无细类条目走文本启发兜底，仍未命中按原文直渲，不再误标。压缩事件实证不进对话流（只进通知栏 client.go:1095），CompactionNode 为对齐 Desktop 的未接线渲染器，未动。

## 已知限制（文档化，非潜藏债）

- metric v2 占位：metric_agg.go Yield/Perceived/e_cost 输出 Null=true，「空值显式声明，禁止以缺失冒充零值」，v2 交付口径文档化（含 cmd/metric.go 用户可见说明）。
- mail 直投二期重接线本身是明账；handler_online_models 价格拉取静默降级注释明说「已确认的降级策略」。

## 排除项（实证非债）

- handler_terminal.go `zsh --login -i` 无显式 Setsid：creack/pty v1.1.24 start.go L22 内部 `cmd.SysProcAttr.Setsid = true`，不违反子进程军规。
- tools/dynamic.go「not implemented」占位：有真实使用方（internal/mcp/tool.go MCP 工具动态包装），非死代码。
- TODO.md 现有三条（实机验收 / Windows 死代码清理 / Homebrew fallback 自建残缺 release）为既有挂账，与本次扫描发现不重叠。

## 下次我看到「终端开错目录 / 信件身份可疑 / TUI 事件显示成最大轮次」，我将

先回本文件三条真债定位，再查壳侧调用参数（terminal.start 是否总带 cwd）与二期 mail 核验进度。
