# mindx 仓库规则

## 必须遵守

* 思考流与推理过程全部使用中文
* 结论先行，简短回答，像人类一样说话
* 禁止将假设或推测当作事实呈现；不确定时直接说明，不完整但诚实的答案优于完整但错误的答案
* 禁止使用表情符号
* 提示词规则（runtime/AGENTS.md、SKILL.md 等面向 LLM 消费的文本）不得引用 LLM 无法知晓出处的外部概念名（如论文名、第三方体系名）——规则必须自解释：每个术语随文给出判定语义，或删除术语只留语义本身（2026-10-06 用户纠正：AGENTS.md 写"EvoSkill 对齐"即缺陷，LLM 不知道 EvoSkill 是什么，外部引用只会分散注意力）
* 生态进化决策先参考《决策路由》论文（2026-10-06 共同复盘定稿，定稿全文见仓库根 `.agents/notes/glm-决策-Agent决策路由三层模型与论文方向.md`）：凡涉及本生态工具集（goharness / mindx / mindx-work / mindx-desktop）演进方向的设计取舍——路由分层、技能体系、记忆体系、团队协作机制的改动——先对照论文论点与成熟度模型回答两个问题：该设计落在三层路由的哪一层（Team 职责路由 / Agent 领域路由 / Skill 方案路由）？如何被进化闭环（记录→复盘→蒸馏→棘轮）吸收？两问答不上来的设计默认不做；论文成文后引用路径更新为正式文档
* 在 mindx 生态新建或修订技能必须通过 skill-creator 技能流程执行：创建前 `mindx skill list` 查重、读其 references/schemas.md 核对 frontmatter、validate 自检、建 evals/trigger_queries.json 触发评估查询集（约 20 条正负各半）、落位后 `mindx skill add` + `mindx reload skills`——直接手写 SKILL.md 即为缺陷（2026-10-13 用户纠正：plugin-planner 首建手写未走流程被要求返工）

## daemon 更新与 E2E 验证（2026-10-04 用户定稿）

* 本地改代码后要让 daemon 用上新二进制，**必须使用 `make restart`**（构建 → 写入 `~/.mindx/bin` → launchd 强制重启 → 运行时同步，一条命令完成）；禁止手动 kill/重启 daemon 或手动拷贝二进制——编译必须在 kill 之前，`make restart` 已内置该顺序（KeepAlive=true 会在 kill 后立刻复活，复活的就是新二进制）
* 惯性警示（2026-10-08 实锤）：做市场联调时因为搭了 HOME 隔离的测试 daemon（`-p :1319`），惯性延伸到「用户真实 daemon 也手动重启/手动改二进制」——**隔离测试场景之外，凡涉及让用户真实 daemon 吃到新代码，唯一路径是 `make restart`**；写任何 daemon 启动/重启命令前先自问"这是不是该 make restart 的场景"（2026-10-08 用户再次提醒"你又忘记军规了"）

## runtime 目录铁律（2026-10-04 用户定稿，违反即事故）

* `runtime/` 是 `~/.mindx` 的初装种子：初装时由 `ExtractWorkspace`（internal/core/workspace.go）将嵌入 FS 中 `runtime/` 下的一切整体释放到用户主目录，运行期由 `SyncRuntimeAssets` 按版本同步——**runtime/ 下任何文件都是会到达用户机器的活配置，不存在"死配置"**
* 在 `runtime/` 下新增或移动文件时，**必须同步更新根目录 `embed.go` 的 `//go:embed` 清单**——清单未收录的文件进不了嵌入 FS，初装释放不到用户目录（2026-10-04 实锤：policy.json 加入 runtime/ 后漏更新清单，被误判为死配置）
* 用户可编辑的根级配置文件（如 `policy.json`）必须加入 `SyncRuntimeAssets` 的 `checkFiles` 保护列表（checksum 未变才随版本更新），防止版本升级覆盖用户改动
* 判断 runtime/ 文件的引用关系时，检索范围必须包含 `embed.go` 清单、`ExtractWorkspace`/`SyncRuntimeAssets` 的 WalkDir 逻辑与 `internal/setup` 的同步调用点，缺一处即结论无效

## ~/.mindx 数据落点军规（2026-10-30 用户定案）

* mindx 用户数据根唯一且永远是 `~/.mindx`。下表为全部合法落点清单，**禁止未经用户明确授权新建任何目录或文件落点**——新落点必须先经用户批准入清单，再写代码：

| 落点 | 用途 | 唯一写方 |
| --- | --- | --- |
| `mindx.json` | 根级配置 | mindx |
| `policy.json` | 用户可编辑策略（SyncRuntimeAssets 保护） | mindx 初装 / 用户 |
| `AGENTS.md` | 根级 Agent 规则 | runtime 种子 / 用户 |
| `bin/` | mindx 主二进制 | make restart / mindx upgrade |
| `.venv/` | Python 运行环境 | mindx doctor --fix |
| `logs/` | 日志 | mindx |
| `settings/` | providers.yml、models.yml、mcp.json、client-provider.yml | mindx |
| `agents/` `teams/` `skills/` `plugins/` | 生态实体 | runtime 种子 / 市场安装 / mindx |
| `data/` | 运行数据（kv.db、schedules、studio） | mindx；令牌不在此（历史遗留待清除） |
| `.token` | 本机握手令牌，唯一落点 | 仅 daemon（核） |

* 核壳同源、禁止双写（2026-10-30 用户定案）：**任何令牌只写入 `~/.mindx/.token`，唯一写方是 daemon；壳（App）、CLI、桥一律只读**——禁止读方自造令牌、补种兜底、多候选查找、双写。历史遗留 `data/daemon_token` 与 `~/.mindx/daemon_token` 由 daemon 启动时单点迁移到 `.token` 后作废，读方不得保留兼容分支。

* 「Gitee 主源、GitHub 兜底」军规仅适用于**存在 Gitee 同步镜像的仓库**（如 mindx-app）；无 Gitee 镜像的仓库（实证 2026-10-30：DotNetAge/mindx 在 Gitee 无仓库）GitHub 即唯一事实源，其下载/升级链路走纯 GitHub **不构成违反**——引用军规前必须先查证目标仓库是否真有 Gitee 镜像，禁止凭军规字面机械套用（2026-10-30 实锤：升级源审查时未查证就断言 mindx upgrade「违反军规」，被用户纠正）

## 必须禁止

* 禁止凭上游摘要/注释/记忆复刻跨端共享的文件路径、协议细节——**编码前必须先读事实源当前实现**（2026-10-11 实锤：设备接入闸落地时凭摘要写 `~/.mindx/daemon_token`，真实路径是 `daemonTokenPath()` 的 `~/.mindx/data/daemon_token`（dataDir 运行期指向 data 子目录），GUI/CLI 读不到令牌全部握手失败，注册层强制闸把本机客户端一并挡死，用户"一起动就失去与任何 Daemon 的连接"；修复=LocalToken/readDaemonToken 两级查找 data/daemon_token 优先、根目录回落。**2026-10-30 用户否定了当时的修复方向**：事故后未归一数据源、反而以兼容层迁就偏离位置，正是数据源混乱的源头；同日落点已定案为 `~/.mindx/.token`，见「~/.mindx 数据落点军规」）
* 禁止在注释/TODO 中引用未经核实的 API——被注释停用的代码不参与编译，引用了不存在的符号编译器不会报错，启用当天才爆炸（2026-10-17 实锤：daemon.go 的 TODO 注释引用 `gateway.WithAuthenticator`，而该选项在 gort gateway 中从未实现，启用公网鉴权钩子时才暴露，被迫当场在 gort 补实现）；启用任何长期停用的代码前，先确认其依赖的真实存在
* 隔离 MINDX_HOME 起测试 daemon 做连接冒烟时，CLI 的本机握手读的是真实用户根令牌（现为 `~/.mindx/.token`；pkg/rpc 的 defaultUserPrefsDir 不认 MINDX_HOME，与 daemon 侧 core.DefaultUserPrefsDir 不同源）——须把真实 .token 预置进隔离目录再起 daemon，否则 CLI 被强制闸判为外部设备卡 pending（2026-10-17 冒烟实证）
* 启用任何新鉴权闸门（如握手层网络闸）时，必须重审既有 RPC 的可达性变化——闸门放行的每一类新连接（如配对码路径）都继承了该闸下所有 RPC 的暴露面：2026-10-17 审计实锤，配对码让"5 分钟窗口内的短码持有者"成为已验证连接，而既有 network.token.get 会向任何已验证连接下发本机 daemon_token（不可撤销、不受开关限制），组合成权限升级链；修复=本机专属凭据只对本机直通连接（devgate.LocalVerified）下发，外接设备一律用各自可撤销的设备令牌
* 新增会拦截既有调用方的强制闸/中间件时，**上线前必须拿一条真实客户端链路实测**（如 `go run ./cmd/rpctest`：握手 + 一次受闸调用），只跑 daemon 单测不等于本机可用；同时过一遍"客户端握手失败后系统处于什么状态"——失败无重试会把瞬时故障放大成全灭
* 禁止未经核实断言 runtime/ 下文件的"引用/生效"状态——embedding、初装释放、版本同步三层机制都要查证
* 禁止删除 RPC handler / 功能代码时不同步清理对应测试段——`*_test.go` 与被删代码同包同编译单元，遗留段直接导致 `go vet` 编译失败（2026-10-06 实锤：阶段二删 handler_graph.go 漏删 handler_test.go 中 250 行 graph 测试段，阶段三 vet 才暴露）；删除后必须以 `go vet ./...`（而非仅 `go build`）作为完成门槛，测试代码才会进入编译检查
* 禁止在功能退役修订文档时只改任务点名的位置——必须同步核查三类隐性残留：(1) 图片内容是否也与现状不符（2026-10-06 实锤：arch-en.jpg/arch.jpg 仍为双核 AgenticRAG 架构图，引用已删、文件待重绘单核版）；(2) 总述句与数字是否联动（对比表行数、中间件层数、存储条目数、场景清单）；(3) 文档宣称的工具行为是否与代码一致（实锤：INSTALL.md 宣称 doctor 检查 qwen3/bge-small-zh，cmd/doctor.go 早已无此检查）

## description 军规（2026-10-06 用户定稿，重复纠正 ≥20 次的惯性错误）

* Agent / Team / Skill 的 `description` 一律是**概览**：负责什么（或是什么）、能干什么（产出什么）、什么时候用（何时不用）。仅此三问。
* **禁止在 description 中写路由**：不得出现"X 归我；Y 归某实体"式指派、指代其他实体的路由话术、路由层术语；禁止写技术细节。原因：LLM 做选择时会**同时阅读多条** description 靠对比决策——概览里的路由元语言是巨大噪声，指代关系迫使 LLM 在每条上分散注意力做解析，反而破坏选择质量。路由是对比多条干净概览的自然涌现，不是写进某一条的逻辑。
* "这个实体是否独特可辨"的检验只存在于**编写者的自检**（写完看它是否与相邻实体概览可区分），检验动作不产生写进 description 的路由文本。

## internal/svc 测试纪律（2026-10-06 实锤）

* **kvStore 由 `NewDaemon` 构造函数直接初始化**（bbolt 打开并持锁，不是 Start 流程专属——initKVStore/mcp 装配/"Daemon initialization complete" 都在构造函数体内）。测试夹具 `newTestDaemon` 返回的 `d.kvStore` 已可用；**测试内禁止再调 `initKVStore`**——同进程二次打开同一 `kv.db` 会 flock 互斥，`bbolt.Open(Timeout:2s)` 随机超时（实锤：metric.consent 测试首版自调 initKVStore，DefaultDenied 偶过、SetAndRevoke 稳定 timeout，lsof 抓到本进程 fd 12u 持锁才定位）。需要 bbolt 的 handler 测试直接用 `d.kvStore`，需要独立文件时用全新路径自行 `bbolt.Open`
* 测试超时类失败先怀疑**资源在测试进程内被重复持有**（用 lsof 指定测试进程 pid 抓本进程句柄），不要先怀疑环境/沙箱——本次 sandbox 与缓存假设全部排查一轮后才发现是自撞锁
* **凭据存储类测试在已登录的 mac 开发机上曾必假失败（2026-10-20 已治本）**：`TestHandleMarketAccountGet_Anonymous` 期望匿名 `has_token=false`，但 macOS 下 `NewCredentialStore` 返回 `macKeychainStore{service:"mindx"}`——service 名全局固定不随测试临时目录隔离，本机 daemon 登录后令牌常驻真实钥匙串，测试进程读到登录态即 FAIL（2026-10-17 两次误判实锤）。治本已落地：`SetTestDir` 将 credStore 重定向到测试目录 `encryptedFileStore` 并清空内存三件套（token/username/password 在 DefaultApp 构造期已从真实钥匙串 resolve，只换 store 不清内存不够）。新测试若需凭据态，显式 `SetMarketCredentials/SetMarketAuthToken` 落 tmp 文件存储

## 子进程与终端前台（2026-10-28 实锤）

* 任何 `exec.Command` 拉起可能交互化的 shell（`zsh -i`/`-lic` 等）必须加 `SysProcAttr{Setsid: true}` 脱离控制终端——交互式 zsh 会主动 open /dev/tty 并开启作业控制 tcsetpgrp 抢占前台，把宿主 TUI 挤到后台，后续 bubbletea 进入 raw mode 时触发 SIGTTOU 挂起（实锤：skillstore resolveBinViaShell 的 `zsh -lic` 致 TUI 启动即 suspended）。注意断 stdin 无效：`exec.Cmd` Stdin 默认已是 /dev/null，zsh 抢前台靠的是自己打开控制终端，唯有 Setsid 可断
* 排障信号：报 "error entering raw mode" 或被 zsh 报 `suspended (tty output)`，先 `ps -o pgid,tpgid` 对比自身 pgid 与 tty 前台 tpgid，再 `ps -t <tty>` 抓占前台的子进程名，禁止盲猜环境问题
