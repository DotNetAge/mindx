# mindx 仓库规则

## 提示词与技能

- 面向 LLM 消费的文本（runtime/AGENTS.md、SKILL.md 等）必须自解释：禁止引用 LLM 无法知晓出处的外部概念名（论文名、第三方体系名），每个术语随文给出判定语义，或删除术语只留语义本身。
- 新建/修订技能必须走 skill-creator 技能流程：`mindx skill list` 查重 → 读其 references/schemas.md 核对 frontmatter → validate 自检 → 建 evals/trigger_queries.json 触发评估查询集（约 20 条正负各半）→ 落位后 `mindx skill add` + `mindx reload skills`。直接手写 SKILL.md 即为缺陷。
- Agent / Team / Skill 的 `description` 一律是概览，仅三问：负责什么（或是什么）、能干什么（产出什么）、什么时候用（何时不用）。禁止在 description 中写路由：不得出现「X 归我；Y 归某实体」式指派、指代其他实体的话术、路由层术语、技术细节——LLM 同时阅读多条 description 靠对比决策，路由元语言是噪声，路由是对比多条干净概览的自然涌现。「是否独特可辨」的检验只存在于编写者自检，不产生写进 description 的文本。
- 生态进化决策先对照《决策路由》论文（全文见仓库根 `.agents/notes/glm-决策-Agent决策路由三层模型与论文方向.md`）：凡涉及本生态工具集（goharness / mindx / mindx-work / mindx-desktop）演进方向的设计取舍——路由分层、技能体系、记忆体系、团队协作机制的改动——先回答两个问题：该设计落在三层路由的哪一层（Team 职责路由 / Agent 领域路由 / Skill 方案路由）？如何被进化闭环（记录→复盘→蒸馏→棘轮）吸收？两问答不上来的设计默认不做。论文成文后引用路径更新为正式文档。

## daemon 更新与版本验证

- 让本地 daemon 用上新二进制的唯一路径是 `make restart`（构建 → 写入 `~/.mindx/bin` → launchd 强制重启 → 运行时同步）；禁止手动 kill/重启 daemon 或手动拷贝二进制。例外：HOME 隔离的测试 daemon（如 `-p :1319`）。写任何 daemon 启动/重启命令前先自问「这是不是该 make restart 的场景」。
- 声称「修复已生效」前必须证明运行二进制与源码一致：`go version -m ~/.mindx/bin/mindx` 核对 BuildTime 与 vcs.modified，再用 Makefile 同参（-trimpath + 同 ldflags）在 /tmp 重建并比对 sha256。只改源码、只重启、只看到修复相关日志都不构成证据。
- `~/.mindx/bin/.update/mindx` 是升级暂存包，可能是 clean-tree 旧构建（无 +dirty），应用后会把未提交修复回滚；排障「改了却没用」必须先比对暂存包的 BuildTime 与 vcs 标记。

## runtime 目录铁律

- `runtime/` 是 `~/.mindx` 的初装种子：初装时由 `ExtractWorkspace`（internal/core/workspace.go）整体释放到用户主目录，运行期由 `SyncRuntimeAssets` 按版本同步——runtime/ 下任何文件都是会到达用户机器的活配置，不存在「死配置」。
- 在 runtime/ 下新增或移动文件时，必须同步更新根目录 `embed.go` 的 `//go:embed` 清单——清单未收录的文件进不了嵌入 FS，初装释放不到用户目录。
- 用户可编辑的根级配置文件（如 `policy.json`）必须加入 `SyncRuntimeAssets` 的 `checkFiles` 保护列表（checksum 未变才随版本更新），防止版本升级覆盖用户改动。
- 判断 runtime/ 文件的引用/生效关系时，检索范围必须包含 `embed.go` 清单、`ExtractWorkspace`/`SyncRuntimeAssets` 的 WalkDir 逻辑与 `internal/setup` 的同步调用点，缺一处即结论无效；禁止未经核实断言其引用/生效状态。

## ~/.mindx 数据落点军规

- mindx 用户数据根唯一且永远是 `~/.mindx`。下表为全部合法落点清单，禁止未经用户明确授权新建任何目录或文件落点——新落点必须先经用户批准入清单，再写代码：

| 落点                                    | 用途                                                     | 唯一写方                            |
| --------------------------------------- | -------------------------------------------------------- | ----------------------------------- |
| `mindx.json`                            | 根级配置                                                 | mindx                               |
| `policy.json`                           | 用户可编辑策略（SyncRuntimeAssets 保护）                 | mindx 初装 / 用户                   |
| `AGENTS.md`                             | 根级 Agent 规则                                          | runtime 种子 / 用户                 |
| `bin/`                                  | mindx 主二进制                                           | make restart / mindx upgrade        |
| `.venv/`                                | Python 运行环境                                          | mindx doctor --fix                  |
| `logs/`                                 | 日志                                                     | mindx                               |
| `settings/`                             | providers.yml、models.yml、mcp.json、client-provider.yml | mindx                               |
| `agents/` `teams/` `skills/` `plugins/` | 生态实体                                                 | runtime 种子 / 市场安装 / mindx     |
| `data/`                                 | 运行数据（kv.db、schedules、studio）                     | mindx；令牌不在此（历史遗留待清除） |
| `.token`                                | 本机握手令牌，唯一落点                                   | 仅 daemon（核）                     |

- 令牌只写入 `~/.mindx/.token`，唯一写方是 daemon；壳（App）、CLI、桥一律只读——禁止读方自造令牌、补种兜底、多候选查找、双写。历史遗留 `data/daemon_token` 与 `~/.mindx/daemon_token` 由 daemon 启动时单点迁移到 `.token` 后作废，读方不得保留兼容分支。
- 「Gitee 主源、GitHub 兜底」军规仅适用于存在 Gitee 同步镜像的仓库（如 mindx-app）；无 Gitee 镜像的仓库 GitHub 即唯一事实源，其下载/升级链路走纯 GitHub 不构成违反——引用军规前必须先查证目标仓库是否真有 Gitee 镜像，禁止凭军规字面机械套用。

## 必须禁止

- CLI（cmd/ 输出、gateway 命令的机器消费字段）是 Agent 的数据源，禁止以「展示优化」为由改动。「显示给用户看的内容」边界 = GUI（chatflow 等）透传的错误文案与界面渲染层；改动前先定位每个点的消费方是人还是 Agent。
- 禁止凭上游摘要/注释/记忆复刻跨端共享的文件路径、协议细节——编码前必须先读事实源当前实现。
- 禁止在注释/TODO 中引用未经核实的 API——被注释停用的代码不参与编译，引用不存在的符号编译器不报错，启用当天才爆炸；启用任何长期停用的代码前，先确认其依赖真实存在。
- 启用任何新鉴权闸门（如握手层网络闸）时，必须重审既有 RPC 的可达性变化——闸门放行的每一类新连接都继承该闸下所有 RPC 的暴露面。本机专属凭据只对本机直通连接（devgate.LocalVerified）下发，外接设备一律用各自可撤销的设备令牌。
- 新增会拦截既有调用方的强制闸/中间件时，上线前必须拿一条真实客户端链路跑通（如 `go run ./cmd/rpctest`：握手 + 一次受闸调用），只跑 daemon 单测不等于本机可用；同时过一遍「客户端握手失败后系统处于什么状态」——失败无重试会把瞬时故障放大成全灭。
- 禁止删除 RPC handler / 功能代码时不同步清理对应测试段——`*_test.go` 与被删代码同包同编译单元，遗留段直接导致 `go vet` 编译失败；删除后必须以 `go vet ./...`（而非仅 `go build`）作为完成门槛。

## 测试纪律

- kvStore 由 `NewDaemon` 构造函数直接初始化（bbolt 打开并持锁），测试夹具 `newTestDaemon` 返回的 `d.kvStore` 已可用；测试内禁止再调 `initKVStore`——同进程二次打开同一 `kv.db` 会 flock 互斥，`bbolt.Open(Timeout:2s)` 随机超时。需要 bbolt 的 handler 测试直接用 `d.kvStore`，需要独立文件时用全新路径自行 `bbolt.Open`。
- 测试超时类失败先怀疑资源在测试进程内被重复持有（`lsof` 指定测试进程 pid 抓本进程句柄），不要先怀疑环境/沙箱。
- 凭据存储类测试：`SetTestDir` 会将 credStore 重定向到测试目录 `encryptedFileStore` 并清空内存三件套（token/username/password 在 DefaultApp 构造期已从真实钥匙串 resolve，只换 store 不清内存不够）。新测试若需凭据态，显式 `SetMarketCredentials/SetMarketAuthToken` 落 tmp 文件存储。
- 隔离 `MINDX_HOME` 起测试 daemon 做连接冒烟时，CLI 的本机握手读的是真实用户根令牌（pkg/rpc 的 defaultUserPrefsDir 不认 MINDX_HOME，与 daemon 侧 core.DefaultUserPrefsDir 不同源）——须把真实 `.token` 预置进隔离目录再起 daemon，否则 CLI 被强制闸判为外部设备卡 pending。

## 子进程与终端前台

- 任何 `exec.Command` 拉起可能交互化的 shell（`zsh -i`/`-lic` 等）必须加 `SysProcAttr{Setsid: true}` 脱离控制终端——交互式 zsh 会主动 open /dev/tty 并开启作业控制抢占前台，把宿主 TUI 挤到后台，触发 SIGTTOU 挂起。断 stdin 无效：zsh 抢前台靠的是自己打开控制终端，唯有 Setsid 可断。
- 排障信号：报 `error entering raw mode` 或被 zsh 报 `suspended (tty output)`，先 `ps -o pgid,tpgid` 对比自身 pgid 与 tty 前台 tpgid，再 `ps -t <tty>` 抓占前台的子进程名，禁止盲猜环境问题。
- Unix 专属 syscall 字段（SysProcAttr.Setsid/Credential 等）必须 build-tag 隔离（`//go:build unix` 与 `//go:build windows` 成对文件，或 GOOS 分支内联结构体字面量），禁止在平台无关代码里直接写——CI 全绿不等于可移植，绿可能只是从未构建过该平台。

## 调度任务会话落实

- 调度任务有两条写入通路，口径必须一致：RPC `schedule.add` 在 [handler_schedule.go](internal/svc/handler_schedule.go) 入口把空 ID 与 `"new"` 哨兵落实成真实会话；Agent 的 `Cron` 工具（`internal/tools/cron.go`）直写 store、原样落库。凡新增写入口，禁止假设「哨兵不会落库」，执行侧必须兜底（`scheduled_exec.go` 现把 `""` 与 `"new"` 同视为未绑定，运行前落实会话线）。
- 排障：用户说「定时任务没执行」时，先查 `~/.mindx/logs/daemon.log` 的 `schedule job fired`——有 fired 说明调度器正常，问题在随后几十毫秒内的会话落实/加载；无 fired 才查 reload、cron 表达式与时区。「到点即失败」和「根本没调度」是两类问题。
- 调度任务落库的 `project_dir` 不能为空（ensureScheduleSession 建会话要求工作目录归属）。`Cron` 工具按「显式传参 > 当前会话工作目录」回填，两者都没有就当场拒绝并给出人话指引，禁止静默落库留给执行期才炸。改动此处必须跑 `internal/tools/cron_test.go` 的 Cron 用例：缺目录拒绝、显式目录落库、会话目录回填三条都在。

## 发版与分发纪律

- 打 tag 前工作区必须全量干净：`git status --short` 输出为空（含 untracked 的 .agents 笔记）才允许 commit+tag 发版——tag 指向的 commit 必须包含全部修改，严禁把部分改动留在工作区只发「干净子集」。
- `git push origin main` 在本仓会报 refspec 歧义（存在游离 `refs/tags/main` 同名 tag）：推分支用 `git push origin refs/heads/main`，或先清理该游离 tag。
- 发版依赖升级（如 goharness）时 `go get` 后必须 `go build ./...` 验证通过再提交。
- mindx 不产出 Windows 原生构建物：Windows 用户的 daemon 由壳的 WSL 版本以 rootfs（内含 linux-amd64 二进制）分发，升级链在 WSL 内部走 linux GOOS。给 release.yml/打包矩阵加任何 Windows 目标前必须先核对分发载体；壳代码（daemon-installer.ts 等）里的 Windows 分支描述的是「壳在 Windows 上的操作」，不等于 daemon 有 Windows 原生形态。
- 发布工作流里消费构建产物或 Release 资产的 job，`needs` 必须显式写到对应上游 job（tag 推送取产物 → 等 `build`；从 Release 下载 → 等 `release`），禁止只写 `prepare`——job 会与 build 抢跑，`actions/download-artifact` 空手而归。校验和类步骤一律 `set -euo pipefail` 并显式 `test -n` 断言，禁止让空值流到下游产物。
- 装配链里所有涉及网络的外部命令（brew / apt-get / yum / dnf / winget / pip）必须带硬超时（`exec.CommandContext` + `context.WithTimeout`），超时后一律翻译成「暂时无法从 XX 下载…可到 python.org 手动安装」的可操作提示，禁止裸 `cmd.Run()` 无限等待；普通失败与网络失败用哨兵错误区分（`internal/setup/python_setup.go` 的 `errCommandTimeout`）。

## 文档修订纪律

- 功能退役修订文档时禁止只改任务点名的位置，必须同步核查三类隐性残留：
  1. 图片内容是否与现状不符（架构图等可能仍是退役前形态）；
  2. 总述句与数字是否联动（对比表行数、中间件层数、存储条目数、场景清单）；
  3. 文档宣称的工具行为是否与代码一致（对照 cmd/ 实现）。
