# MindX 凭什么

---

## 先说人话：它能干什么

### 你可能遇到过这些情况

跟 AI 聊代码的时候，聊着聊着它就忘了你前面说过什么。你提了一个需求，它给了方案，你说不对，它改了，改完你又发现它把第一版里唯一合理的那部分也丢了。来回拉锯几轮，上下文窗口满了，它开始胡说八道或者直接重复之前的回答。

或者你让它帮你改一个项目里的东西。项目有几百个文件，它只看了你贴过去的那两段代码就开始给建议——完全不知道你的项目结构、编码风格、之前做过的技术决策是什么。给出的方案理论上没问题，但放到你的项目里就是接不上。

再或者你想让它同时干几件事：查一下这个 bug 的根因、写几个测试用例、更新一下文档。它只能一件一件串行处理，而且每件都是浅尝辄止。

这些都是当前 AI 编程助手的通病——**它们没有记忆、不会协作、聊久了就失忆、对项目的理解停留在你贴进去的那点文本上。**

### MindX 不一样在哪

**它会记住你的项目。**

不是那种"你每次手动把代码贴给它"的记住。是你告诉它你的项目目录在哪，它自动把里面的文件读进来、解析、建立索引——PDF 文档、Word 需求文档、代码文件、配置文件、甚至邮件往来，全部变成可搜索的知识库。你改了代码，它几秒后就能搜到最新版本。这个过程是后台自动的，不需要你操心。

**它聊多久都不失忆。**

别的助手聊到一定长度就开始丢信息。MindX 不限制会话长度——聊得再长，早期讨论过的内容也能被精准召回。它不是简单地把旧消息存在那里，而是理解了语义相关性：当你问到相关话题时，之前聊过的内容会自动浮上来。

**它能自己判断什么时候该找人帮忙。**

你让它重构一个模块并写测试。它知道重构归后端工程师管、测试归测试工程师管。于是它自己把任务拆开，分给两个专门的 Agent 并行执行，最后把结果汇总给你。整个过程你没做任何调度工作。

**它在安全问题上不嫌麻烦。**

大部分 AI 工具的安全措施就是一个确认弹窗，点多了你就无脑点"允许"。MindX 不同——你可以写规则："禁止删除操作"、"git push 前必须问我"、"读取文件一律放行"。90% 的日常操作在规则层面就自动裁决了，真正弹窗问你的是那些确实需要人类判断的场景。

**关掉界面它还在干活。**

MindX 有一个后台守护进程（Daemon），不依赖任何界面运行。你可以设定定时任务——每天早上帮你看一下 git log、每周五跑一遍测试套件、监控某个目录的文件变化。只要电脑开着，它就在。

### 适合什么场景

| 场景                 | MindX 怎么帮                                                         |
| -------------------- | -------------------------------------------------------------------- |
| 接手一个 legacy 项目 | 自动索引全部代码和文档，随时问它"这个模块是干嘛的""谁调用了这个函数" |
| 长时间开发一个功能   | 会话不限长度，三天前讨论过的架构决策它还记得                         |
| 同时推进多个任务     | 自动拆分子任务分配给不同 Agent 并行执行                              |
| 维护项目文档         | 改了代码知识库自动更新，生成的文档始终跟代码同步                     |
| 定期巡检             | 设好 Cron 任务，定期跑测试、检查依赖、生成报告                       |
| 团队知识沉淀         | 多 Agent 共享同一个项目知识库，新成员上手更快                        |

---

## 技术细节：怎么做到的

> 下面这部分写给想了解实现细节的人看。如果你只是想知道 MindX 能干什么，上面那些已经够了。

MindX 由三个 Go 项目组成——goreact（思考引擎）、gorag（记忆与检索）、mindx（应用层）。下面拆开说。

### 它真的会"想"，不是在装

市面上的 Agent 框架大多套个 ReAct 模板就完事了：调用 LLM → 拿到工具调用 → 执行 → 把结果塞回去 → 下一轮。循环往复直到凑够轮数或者撞上 max_iterations 硬上限。模型在这个过程中基本是在梦游——它不知道自己上一轮做了什么判断，也不知道那个判断对不对。

MindX 的 Reactor 不一样。每一轮 Think-Act-Observe 循环结束之后，它的推理（reasoning）会作为下一轮的上下文保留下来。这意味着模型能看到自己之前的判断链：**"我上次觉得应该用 A 方案，但工具返回了错误 B，所以这次我改试 C"**。这不是 prompt 里写一句"请反思"就能骗到的行为，是架构层面强制执行的自省机制。

具体来说，[reactor/think_act_observe.go](../goreact/reactor/think_act_observe.go) 里每个周期产出的 Thought 对象包含 `decision`、`reasoning`、`confidence` 三个字段。reasoning 不是写给用户看的漂亮话，是喂给下一轮 Think 阶段的原始材料。模型在下一轮真正能"看到"自己之前想了什么、做了什么决策、置信度多少。

#### 还有个卡死救生员

LLM 在循环里卡住是常态不是异常。反复调同一个失败的工具、在两个选项之间反复横跳、空转好几轮不出结果——这些情况你用多了 Agent 一定见过。

MindX 内置了四个检测器，各自盯着一种卡死模式（[stuck_detector.go](../goreact/reactor/stuck_detector.go)）：

| 检测器              | 盯什么                 | 触发条件      |
| ------------------- | ---------------------- | ------------- |
| ToolLoopDetector    | 同一个工具连续调用     | 连续 3 次     |
| ErrorLoopDetector   | 相同错误反复出现       | 连续 2 次     |
| OscillationDetector | 决策在两个值之间来回跳 | 连续 4 轮交替 |
| NoProgressDetector  | 好几轮没产出答案       | 连续 5 轮     |

关键是它**不会一刀切杀掉进程**。检测到问题后，往系统提示里注入一条 nudge（提示），比如 *"你已经连续调了 3 次 grep 了，要不要换个思路？"*。给模型自纠的机会。同一种模式连续 nudge 三次还没改善，才硬终止。这比粗暴的 `max_iterations=10` 人性化太多了。

---

### 记忆系统不是贴牌的

很多框架的"记忆"就是个向量数据库的 wrapper：存进去、搜出来、完事。MindX 的记忆是**长在架构里的**，不是后加的功能。

#### 两层记忆，各管各的事

**长期记忆**——项目级的知识库。你打开一个项目目录，里面的文件会被自动解析、分块、索引进去。支持什么格式？[gorag/document/](../gorag/document/) 目录底下摆着：PDF（带 OCR）、Word、Excel、PPT、HTML、CSV、邮件（EML/MSG）、EPUB、纯文本……基本上你能想到的办公文档格式都覆盖了。

分块也不是傻傻地按字数切。[chunker/semantic.go](../gorag/chunker/semantic.go) 里的语义分块器会用嵌入模型算每个句子的向量，相邻句子相似度突然掉下来的地方就是话题边界——就在这里切。这样切出来的每块内容语义上是完整的，搜索的时候不会出现"一半在上半块一半在下半块"的尴尬。

**短期记忆**——会话级的临时存储。聊久了上下文窗口不够用，超出的部分"滑出去"。但滑出去不等于丢了，而是进了语义索引。下一轮如果你问的问题跟之前聊过的相关，这些内容会被自动召回，塞回系统提示的 `## Relevant Context` 区段。

#### 记忆闭环：写和读都是自动的

这个设计最巧妙的地方在于**对 LLM 完全透明**：

- **写半环**：上下文滑动时（`doSlide()`），被挤出的消息通过 SlideHandler 自动写入短期记忆（[memory_hook.go](../goreact/reactor/memory_hook.go)）
- **读半环**：每轮 Think 之前，MemoryThoughtHook 自动从记忆中检索相关内容，注入到系统提示的动态区域

模型不需要知道"我应该去查记忆"，也不需要调用任何工具。就像人一样——你不需要刻意回忆上周聊过什么，相关的记忆自然会浮上来。

长期记忆不一样，它是按需查询的。文件级的内容粒度太粗，自动塞进上下文太浪费空间，所以通过 MemorySearch 工具让模型主动搜。三条搜索路线按优先级排：先搜自己的长期记忆库 → 再上网搜 → 最后才问用户。每条路都走不通才承认不知道。

#### 混合搜索：不是只有向量检索

[hybrid.go](../gorag/hybrid.go) 里的 HybridIndexer 把三种检索方式捆在一起：

- **语义检索**（权重 0.7~0.8）：向量相似度，擅长理解意图
- **全文检索**（权重 0.2）：BM25 关键词匹配，擅长精确匹配专有名词
- **图检索**（权重 0.1）：知识图谱遍历，擅长发现隐含关联

三路结果用 RRF（Reciprocal Rank Fusion）算法融合排序，再过一遍重排序器（reranker）。这意味着搜"那个处理用户认证的函数"这种模糊描述时，语义检索能理解意图；搜"UserAuthMiddleware"这种精确名称时，全文检索能精确命中；而图检索能找到"哦这个函数调用了 PasswordValidator，PasswordValidator 又引用了 bcrypt 配置"这种链条关系。

---

### 权限系统是认真的，不是摆设

大多数 Agent 的安全措施就是一个弹窗："你要执行 rm -rf 吗？"用户点多了就麻木了，要么全点允许，要么烦了关掉。

MindX 的权限检查是一条**三级流水线**（[permission_rule.go](../goreact/core/permission_rule.go)，[permission_chain.go](../goreact/core/permission_chain.go)）：

```
请求进来 → SecurityLevel（自动判危险等级）
         → RuleBasedChecker（匹配预定义规则）
         → AskPermission（实在没法裁决才问用户）
```

RuleBasedChecker 这层最有意思。你可以定义类似这样的规则：

```yaml
always_deny:
  - tool_name: "Bash"
    content_pattern: "rm "
    description: "禁止删除操作"
  - tool_name: "Write"
    content_pattern: "/etc/**"
    description: "禁止修改系统文件"

always_allow:
  - tool_name: "Read"
    description: "文件读取一律放行"

always_ask:
  - tool_name: "Bash"
    content_pattern: "git push"
    description: "推送代码前需要确认"
```

匹配顺序是 deny → allow → ask。deny 优先级最高（安全第一）。规则支持工具名匹配 + 内容模式匹配（命令前缀、路径 glob、URL 前缀）。大部分日常操作在前两级就被自动裁决了，**真正弹窗问你的是那些确实需要人类判断的场景**。

---

### 上下文管理有两道防线

#### 第一道：无限会话

上下文窗口满了怎么办？别的框架：截断旧消息 / 用 LLM 做摘要压缩。两种都会丢信息，而且上下文越长，模型越容易出现"中间遗忘"效应。

MindX 的方案是**滑动窗口 + 语义召回**。维持一个舒适的上下文窗口大小，超出的部分滑入短期记忆（语义索引）。需要的时候自动召回来。效果就是：**理论上可以无限聊下去，而且不会丢信息**。

[compact.go](../goreact/core/compact.go) 里的 MicroCompact 做了一件聪明的事：从最新的消息开始往前保留，旧的优先截断。而且截断不是傻傻地砍半——它会保留消息的结构和角色信息，只裁剪内容体。

#### 第二道：Token 结果预算

就算窗口控制得再好，一次工具调用也可能搞崩一切。想象一下 Agent 读了一个 5MB 的日志文件，或者跑了个返回几万行结果的数据库查询——直接塞进上下文的话，这一轮就废了。

[tool_result_budget_enforcer.go](../goreact/reactor/tool_result_budget_enforcer.go) 实现了两层防御：

1. **软限（Per-Tool）**：单个工具的结果超过阈值 → 写入磁盘 → 替换成一个 `<persisted-output>` 标签（包含预览）。大结果没有丢失，只是不在上下文里占位置。
2. **硬限（Per-Action）**：一轮所有工具结果的总量还超限 → 从已替换的结果中挑最大的几个进一步压缩为 `[Result suppressed: XX KB]` 提示。

而且这套机制有状态记忆——已经被处理过的结果不会重复处理，需要完整输出时可以重新执行工具获取。

---

### KV Cache 能省则省

长对话场景下，每次请求都要把系统提示重新送给模型。系统提示动辄几千 token，其中大部分内容（身份定义、行为规则、工具说明）跨轮次是不变的。

[prompt.go](../goreact/reactor/prompt.go) 里的做法是把系统提示拆成多个分段，中间插一个 `__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__` 标记：

```
┌─────────────────────────────────┐
│ 身份定义 (Identity)              │  ← KV Cache 命中 ✅
│ 技能目录 (SkillsCatalog)         │  ← KV Cache 命中 ✅
│ 行为规则 (Behavioral Rules)      │  ← KV Cache 命中 ✅
│ 输出格式 (Output Format)         │  ← KV Cache 命中 ✅
│ 工具使用指南 (Tool Usage)        │  ← KV Cache 命中 ✅
│ 环境信息 (Environment)           │  ← KV Cache 命中 ✅
│ ...                              │
├─────────────────────────────────┤  ← DynamicBoundary
│ 相关记忆 (Relevant Context)      │  ← 每轮变化
│ 输出风格 (Output Efficiency)     │  ← 可能变化
└─────────────────────────────────┘
```

分界线以上的部分跨轮次不变，能稳定命中 KV cache。分界线以下的部分每轮可能不同，但不影响 cache 前缀。实测下来，长对话场景下能省下不少推理时间和 token 费用。这个优化是在框架设计阶段就考虑进去的，不是事后补丁。

---

### 多Agent协作不是纸上谈兵

很多框架提到"多 Agent 协作"时，实际上只是让你手动把任务分配给不同的 Agent。MindX 的协作是 **Agent 自主触发**的。

[coordination.go](../goreact/reactor/coordination.go) 和 [subagent.go](../goreact/tools/subagent.go) 实现了一套完整的子任务协调机制：

1. 主 Agent 判断某个任务超出自己的职责范围
2. 调用 SubAgent 工具，指定子 Agent 名称和任务描述
3. 子 Agent 在后台异步执行（`go func()`），主 Agent 不阻塞
4. 主 Agent 通过 CollectResults 收集结果

CoordState 维护了一个完整的生命周期状态机：Running → Interrupted → Cancelled / Completed。支持中断（Interrupt）、恢复（Resume）、全局超时自动取消。每个子任务有独立的 context，父任务取消时所有子任务级联取消。

实际效果就是你可以说"帮我重构这个模块并写测试"，主 Agent 会自己决定把"重构"交给 code-reviewer Agent、"写测试"交给 python-engineer Agent，两个并行跑，最后汇总结果给你。整个过程你没做任何编排工作。

---

### Hook 链：想扩展？不用改框架源码

[internal/reactor/hooks/](../goreact/internal/reactor/hooks/) 目录下的三层 hook 链是整个框架的扩展脊梁：

| 层级                | 执行时机       | 内置用途                            | 可扩展方向                           |
| ------------------- | -------------- | ----------------------------------- | ------------------------------------ |
| **ThoughtHook**     | Think 阶段前后 | 记忆召回、卡死 nudge 注入、前置校验 | 推理约束、内容审核、自定义思维链     |
| **ToolHook**        | 工具执行阶段   | 权限检查、预算控制、审计日志        | 工具调用过滤、敏感数据脱敏、计费统计 |
| **ObservationHook** | 工具结果返回后 | 收敛检测、日志记录                  | 结果后处理、自动重试、缓存           |

每层 hook 按 Priority 排序执行。内置 hook（记忆、卡死检测、权限） Priority 在 40-50，你的自定义 hook 可以插在 0-39 之间。想加功能？实现接口、注册进去，完事。不用 fork 框架、不用改核心代码。

---

### Daemon：关掉界面它还在干活

[daemon.go](../mindx/internal/svc/daemon.go) 定义的后台守护进程是 MindX 区别于普通 CLI 工具的关键。

**Daemon 不绑定任何前端**。TUI 可以关，浏览器可以关，MacUI 可以关——只要机器在跑，Daemon 就在。它同时跑着三个服务：

#### WebSocket 网关

JSON-RPC 2.0 协议，所有前端（TUI / WebUI / MacUI / 未来可能的 IDE 插件）都走这一个入口。Agent 的 ThinkingDelta（思考过程流式输出）、ToolExecStart/End（工具调用事件）、FinalAnswer（最终答案）、PermissionRequest（权限请求）等十几种事件类型实时推送到客户端。你在 TUI 上看到的思考过程闪烁、工具调用进度条，背后都是这些事件驱动的。

#### Cron 调度器

[scheduler.go](../mindx/pkg/scheduler/scheduler.go) 实现了秒级精度的定时任务。可以设定"每天早上 9 点帮我总结一下项目的 git log"、"每周五下午跑一遍测试套件"这类定时任务。调度器从持久化的作业存储中加载配置，每 5 秒热检一次变更——你加了新任务不用重启 Daemon。

#### 文件监控服务

[project_indexer.go](../mindx/pkg/memory/project_indexer.go) + [watch_service.go](../mindx/pkg/memory/watch_service.go) 组合起来做的事：监听你指定的项目目录，文件变了就自动增量更新索引。

具体流程：
1. 你在 TUI 里把一个目录加入 watchlist
2. Daemon 用 fsnotify 监听该目录的文件变更
3. 变更事件经过 .mindxignore 过滤（忽略 .git、node_modules、.venv 之类）
4. ProjectIndexer 只对变动的文件做增量索引（通过 mtime + size 缓存判断是否需要更新）
5. 新的 chunk 写入长期记忆的混合索引器

效果就是：**你改了代码，MindX 的知识库自动跟着更新**。不需要手动"重建索引"，不需要"同步"操作。你在编辑器里保存文件，几秒后 MindX 就能搜到你刚写的内容。

---

### 会话是持久化的，不是用完即弃

[session/file_store.go](../mindx/pkg/session/file_store.go) 实现的会话存储不只是聊天记录——它保存了完整的会话状态：SessionID、关联的项目目录、使用的 Agent 角色、元数据。

下次打开 MindX，可以选择恢复上次的会话。即使中间 Daemon 重启过、机器重启过，只要会话文件还在磁盘上，就能无缝接上。这对长任务特别重要——你不可能保证一次对话完成所有工作。

---

### 技能生态：40+ 即插即用的能力包

[runtime/skills/](../mindx/runtime/skills/) 目录下的技能不是 demo 级别的玩具，是实打实能用的工具集：

- **开发类**：code-reviewer（代码审查）、bug-hunter（Bug 定位）、architect（架构设计）、frontend-engineer（前端实现）、python-engineer（Python 开发）
- **创作类**：copywriting（文案写作）、canvas-design（视觉设计）、algorithmic-art（算法艺术）、changelog-generator（变更日志）、doc-coauthoring（文档协作）
- **效率类**：pdf（PDF 处理，支持表单填写/OCR/页面操作）、pptx（PPT 生成与编辑）、docx（Word 处理，含红线审阅模式）、file-organizer（文件整理）
- **集成类**：firecrawl（网页抓取）、agent-browser（浏览器自动化）、n8n-workflow-patterns（工作流模板）、docker-expert（容器化部署）
- **通讯类**：apple-notes / apple-reminders（Apple 生态集成）、slack-gif-maker（Slack 动图）、social-content（社交媒体内容）

技能通过 SKILL.md 声明式定义——包含触发条件、工具列表、使用指南。Agent 在运行时按需加载（lazy load），加载一次后就驻留在上下文中，不会重复加载浪费 token。

---

## 一句话总结

MindX 不是又一个 AI 聊天客户端。它是一个有自省能力的 Agent 运行时，带着一个自动增长的项目知识库，一套认真设计的权限体系，和一个不知疲倦的后台守护进程。goreact 提供思考和行动的能力，gorag 提供记忆和检索的能力，mindx 把它们组装成一个你真正能用来干活的系统。
