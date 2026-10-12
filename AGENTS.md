# mindx 仓库规则

## 必须遵守

* 思考流与推理过程全部使用中文
* 结论先行，简短回答，像人类一样说话
* 禁止将假设或推测当作事实呈现；不确定时直接说明，不完整但诚实的答案优于完整但错误的答案
* 禁止使用表情符号
* 提示词规则（runtime/AGENTS.md、SKILL.md 等面向 LLM 消费的文本）不得引用 LLM 无法知晓出处的外部概念名（如论文名、第三方体系名）——规则必须自解释：每个术语随文给出判定语义，或删除术语只留语义本身（2026-10-06 用户纠正：AGENTS.md 写"EvoSkill 对齐"即缺陷，LLM 不知道 EvoSkill 是什么，外部引用只会分散注意力）
* 生态进化决策先参考《决策路由》论文（2026-10-06 共同复盘定稿，定稿全文见仓库根 `.agents/notes/glm-决策-Agent决策路由三层模型与论文方向.md`）：凡涉及本生态工具集（goharness / mindx / mindx-work / mindx-desktop）演进方向的设计取舍——路由分层、技能体系、记忆体系、团队协作机制的改动——先对照论文论点与成熟度模型回答两个问题：该设计落在三层路由的哪一层（Team 职责路由 / Agent 领域路由 / Skill 方案路由）？如何被进化闭环（记录→复盘→蒸馏→棘轮）吸收？两问答不上来的设计默认不做；论文成文后引用路径更新为正式文档

## daemon 更新与 E2E 验证（2026-10-04 用户定稿）

* 本地改代码后要让 daemon 用上新二进制，**必须使用 `make restart`**（构建 → 写入 `~/.mindx/bin` → launchd 强制重启 → 运行时同步，一条命令完成）；禁止手动 kill/重启 daemon 或手动拷贝二进制——编译必须在 kill 之前，`make restart` 已内置该顺序（KeepAlive=true 会在 kill 后立刻复活，复活的就是新二进制）
* 惯性警示（2026-10-08 实锤）：做市场联调时因为搭了 HOME 隔离的测试 daemon（`-p :1319`），惯性延伸到「用户真实 daemon 也手动重启/手动改二进制」——**隔离测试场景之外，凡涉及让用户真实 daemon 吃到新代码，唯一路径是 `make restart`**；写任何 daemon 启动/重启命令前先自问"这是不是该 make restart 的场景"（2026-10-08 用户再次提醒"你又忘记军规了"）

## runtime 目录铁律（2026-10-04 用户定稿，违反即事故）

* `runtime/` 是 `~/.mindx` 的初装种子：初装时由 `ExtractWorkspace`（internal/core/workspace.go）将嵌入 FS 中 `runtime/` 下的一切整体释放到用户主目录，运行期由 `SyncRuntimeAssets` 按版本同步——**runtime/ 下任何文件都是会到达用户机器的活配置，不存在"死配置"**
* 在 `runtime/` 下新增或移动文件时，**必须同步更新根目录 `embed.go` 的 `//go:embed` 清单**——清单未收录的文件进不了嵌入 FS，初装释放不到用户目录（2026-10-04 实锤：policy.json 加入 runtime/ 后漏更新清单，被误判为死配置）
* 用户可编辑的根级配置文件（如 `policy.json`）必须加入 `SyncRuntimeAssets` 的 `checkFiles` 保护列表（checksum 未变才随版本更新），防止版本升级覆盖用户改动
* 判断 runtime/ 文件的引用关系时，检索范围必须包含 `embed.go` 清单、`ExtractWorkspace`/`SyncRuntimeAssets` 的 WalkDir 逻辑与 `internal/setup` 的同步调用点，缺一处即结论无效

## 必须禁止

* 禁止凭上游摘要/注释/记忆复刻跨端共享的文件路径、协议细节——**编码前必须先读事实源当前实现**（2026-10-11 实锤：设备接入闸落地时凭摘要写 `~/.mindx/daemon_token`，真实路径是 `daemonTokenPath()` 的 `~/.mindx/data/daemon_token`（dataDir 运行期指向 data 子目录），GUI/CLI 读不到令牌全部握手失败，注册层强制闸把本机客户端一并挡死，用户"一起动就失去与任何 Daemon 的连接"；修复=LocalToken/readDaemonToken 两级查找 data/daemon_token 优先、根目录回落）
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
