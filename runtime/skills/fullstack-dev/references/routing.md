# 路由层：显式调用 / 领域路由 / 优先级 / 协同 / 专项映射

> 本文件由主 `SKILL.md` **强制加载**：进入 Step 1（意图识别与路由）前必须 `Read` 本文件。SKILL.md 只保留流程骨架与门禁索引，路由明细集中于此。

---

## 显式调用（`@<英文标识>`）

输入以 `@<合法英文标识>` 开头（须在输入开头或独占一段；邮箱、@提及不触发）→ 跳过关键词匹配，直接加载「子技能索引」表中该标识对应的子技能；拼写错误自动回退关键词路由。

**只跳路由**：意图三分法、安全闸门、澄清策略、验证闭环、复盘沉淀照常执行。

### 任务-技能不匹配提示

显式指定后，若任务实际特征与指定子技能存在强冲突，输出提示让用户确认，不擅自切换：

| 用户指定 | 任务强信号 | 提示动作 |
| ------------------ | ------------------------------- | ----------------------------------------------------- |
| `@code-generation` | 含"审查/review/检查问题/找漏洞" | 询问"任务更像代码审查，是否切换到 @code-review？" |
| `@code-review` | 含"实现/写代码/生成/补接口" | 询问"任务更像代码生成，是否切换到 @code-generation？" |
| `@code-generation` | 含"报错/异常/堆栈/崩溃" | 询问"任务更像 Bug 诊断，是否切换到 @bug-diagnosis？" |
| `@refactoring` | 含"新功能/实现/新增" | 询问"任务更像代码生成，是否切换到 @code-generation？" |
| `@bug-diagnosis` | 含"重构/优化/重写" | 询问"任务更像重构建议，是否切换到 @refactoring？" |

用户确认前不执行；用户回复"按原指定执行"时立即按显式指定走，不再追问。

### 关键词匹配规则

- 用户输入包含「领域路由表」子技能列的关键词 → 匹配该子技能
- 用户输入包含「领域路由表」功能说明列的场景 → 匹配该子技能
- 多个子技能同时匹配 → 先按「子技能优先级矩阵」组合路由；无法组合时选匹配度最高者
- 无法唯一确定 → 向用户确认意图

---

## 意图三分法（决定执行策略与自主度）

| 类型 | 判定标准 | 动作 |
| -------- | ------------------------------------------------------------------------ | ---------------------------------------------- |
| 信息查询 | 只问概念、比较、解释、建议或只读评估，未要求改文件 | 直接回答或只读检查，不修改文件 |
| 简单任务 | 目标明确、范围小、风险低、可用最小验证闭环 | 进入快速通道，直接执行并交付验证证据 |
| 复杂任务 | 涉及多文件、跨模块、数据库/配置/构建链、安全权限、批量治理或业务规则不明 | 先输出方案、影响范围和验证路径，必要时等待确认 |

意图分类只决定执行策略，不替代子技能路由；分类后仍需按本表加载对应 reference 模板。

### 模糊大型任务探索路径（Wayfinder 模式）

当任务同时满足「复杂」且「目标模糊」（用户只说方向未说具体交付物，描述 <30 字且无历史上下文）时，在需求澄清前先启动探索：① 现状快照（模块数/文件数/关键依赖/已知痛点一句话）→ ② 问题树展开（症状→可能原因→验证方式，2-3 层）→ ③ 候选路径排序（影响面/成本/风险，标推荐）→ ④ 对首选路径执行一个只读验证动作 → ⑤ 收敛为 1-2 句可验收任务。收敛后回到 SKILL.md Step 1.5 做验收口径对齐。此路径与 Step 1.5 不互斥：Wayfinder 只把模糊方向收敛为明确任务。

---

## 加载经济与执行集（按档位）

> 定位：本节管**读多少 / 何时读**与**走多少 / 做到哪**；SKILL「上下文延迟加载协议」管**默认不预读**。二者同族不重复——本节给**加载上限 + 各档最小执行集**。

### 档位 → 最小执行集（单一定义源）

**判定顺序（自上而下取首个命中，非严重度排序）**：

- **轻量** —— ≤1 文件 + ≤1 步 + 非安全，且不含内联 JS / HTML 结构 / CSS / 表单 / 布局 / 后台模板（同判据已内联于 `SKILL.md` Step 1.2 供零加载自判；两处不一致以本表为准）；
- **大项目** —— ≥10 文件或 ≥500 行新增；
- **复杂** —— ≥3 文件 / 跨模块 / 含状态机 / 新增守卫·拦截·下限·配额·锁类约束 / 涉缓存策略·并发或事务边界 / 业务或验收口径不明；
- **常规** —— 其余需执行任务（单文件多行 / 多步工具链 / 常规 Bug 修复）。

**安全闸门优先**：命中生产配置 / DB 写入 / 权限 / 凭证 / 隐私 / 不可回滚 → 无论判定为何，强制按**复杂**档执行。
**升降档**：执行中发现跨模块 ≥2 / 批量 >10 / DB 写入 / 不可回滚 → 立即升**复杂**；达 ≥10 文件或 ≥500 行 → 升**大项目**。
**别名归一**：本技能各处的「快速通道」即本表的**轻量档 + 常规档**，其执行集以本表为唯一准据。

| 档位 | Step 0 记忆 | Step 0.5 环境 | Step 2 规划门禁 | Step 4 验证 | Step 5 交付 | Step 6 复盘 | reference 加载 |
|---|---|---|---|---|---|---|---|
| 轻量 | 仅续做 / 恢复时载 | 免 | 免（模板 + 三池 + 规划自审） | ≥1 条真实证据（命令 + 输出） | 一行收尾（完成度 / 产出 / 待办） | 免 | 0 |
| 常规 | 续做 / 恢复时载 | 已部署则免 | 免三池，仅「需求→步骤」最小映射 + 引用点实码确认 | 清单化自审（安全 10 条仅命中项） | 精简自检（E2E 下限 / 安全 / 构建 / 回退 / 未验证项披露）+ 收尾报告 | 轻量（`daily.md` 一行） | ≤1 |
| 复杂 | 三层完整恢复 | 执行 | 全套三池 + 规划自审 + 方案成形自检 | 全套（含覆盖枚举 / 配对契约 / 真人功能验证） | 全套执行率自检 + 收尾报告 | 完整 | ≤3 |
| 大项目 | 三层完整恢复 | 执行 | 全套 + C 池 + 架构决策记录 | 全套 + Wave 级证据 | 全套 + Wave 交接 | 完整 + Wave 交接 | ≤3 |

> **表外剪裁（同源，勿遗漏）**：**Step 1** 轻量档免读本文件（判据已内联 `SKILL.md` Step 1.2）；**Step 1.5** 简单任务（轻量 / 常规档）豁免；**Step 3 实码确认** 轻量档＝`read_file` + 冲突检查；**写后即记** 轻量档免写 `daily.md`。

> 轻量 / 常规档免的是**仪式与门禁层**；安全闸门、安全红线、Step 3 写码前实码确认（轻量档为 `read_file` + 冲突检查）、Step 4「无证据 = 未完成」对**所有档位**不豁免。

- **每场景最小加载集（上限，非建议）**：信息查询 → **0 个** reference（只读 + 直接答）；轻量 → **0 个**（免载）；常规 → **≤1 个**（首选 reference）；复杂 / 大项目 → **≤3 个**（首选 + ≤2 协同），仍需更多则**按阶段切开分批做**。
- **禁预载与过度路由**：命中多领域只载**首选 + 直接协同**；禁"以防万一"预载、禁把整张路由表当清单逐个加载；发现已载 >3 而任务实为单文件 / 纯查询 → 按「对话流异常边界」只留首选。
- **按步增量加载**：Step 1 只读本文件 → Step 2 才读 `execution-safety`（规划门禁）→ Step 2–3 按命中读专项 → Step 4–5 才读 `delivery-assurance` → Step 6 才读 `project-memory-management`。**禁开局一次全载**。
- **单源不复制**：同一判据只留**唯一承载文件**，其余一律指针；新增内容写进既有章节而不再起（同族判据出现两处 = 缺陷，须收敛为一处）。
- **输出预算**：结论 + 证据优先，不复述计划、不重复已知（对齐 `agent-reasoning-patterns` 模式 16 输出经济性）；长任务只报阶段产出 / checkpoint / 收尾，不做每步播报。

**必载文件的分节取用（必载 ≠ 通读）**：先用 `search_content "^## "` 定位小节，再 `read_file` 目标节（offset / limit）。下表「Step 必读节」**该 Step 一到即须已读**（缺则视为该步门禁未生效）；「按需节」命中场景才读。

| 必载文件（≈字符） | Step 必读节（缺则门禁未生效） | 按需节（命中才读） |
|---|---|---|
| `routing.md`（18.6k） | **Step 1**：「意图三分法」+「加载经济与执行集」（含档位判定）；查表时定位读「子技能索引 / 领域路由表 / 优先级矩阵 / 协同顺序 / 专项映射」对应行 | 无（其余小节仅供查表） |
| `execution-safety.md`（15.2k） | **Step 2**：「规划门禁」+「方案成形自检」+「安全闸门」；**Step 3**：「写码前实码确认」（含诊断先行）；**Step 4**：「清单化质量自审」+「安全红线清单」+「覆盖完整性枚举闸门」+「配对契约与迁移完整性核验」+「真人功能验证硬约束」 | 「失败计数与升级」（Step 3 出失败时）；「澄清策略分级」（需求不明时）；「批量修改 7 防线」（批量时）；「架构一致性铁律 / 复用 vs 复制 / 防 AI 通病五戒」（改既有系统时） |
| `delivery-assurance.md`（9.6k） | **Step 5**：「执行率自检」+「收尾报告模板」+「抗合理化」+「未验证项强制披露」 | 「交付信心门控」（复杂 / 高风险交付）；「穷尽后的结构化失败报告」（失败穷尽时）；「确认超时默认规则」（连续超时）；「Graceful Abort」（中止时） |
| `project-memory-management.md`（13.5k） | **Step 0 / Step 1.1**：「第五步 记忆恢复」+「读取截断规则」+「记忆文件相对路径」；**Step 6**：「第一步 Session Summary」+「第二步 Decision Record」（写该类记忆时读对应模板） | 交接九字段模板（换机 / 暂停时）；「记忆维护协议」（超 30 天蒸馏时） |
| `task-decomposition-and-execution.md`（9.9k） | **Step 2 / Step 3**：「子Agent 边界（L0）」+「长任务执行可靠性（L0）」 | 「五要素结构」/「Wave 链式执行」/「委派全链路纪律」/「Task Summary」（长任务 / 批量子任务时） |

**补读触发（缺一即该 Step 未完成）**：① 进入新 Step 时按其「Step 必读节」核对一次；② 命中场景关键词（失败 / 批量 / 数据 / 权限 / 发布 / 中止 / 换机 / 高风险交付）时补读对应「按需节」；③ 判据冲突需裁决（取更严者）时补读被引用节；④ 补读后在交付证据中点出该节判据的落点。

> 效果：主导场景的固定加载面由整文件（≈57k 字符）降到「Step 必读节」（≈12k 字符）。**禁以"读过了 / 已加载"为由跳过命中场景的门禁**；安全闸门、L0 铁律与「未验证项披露」不因分节取用而豁免。
> 口径配合：Step 必读节属**强制加载集**（`routing` / `execution-safety` / `delivery-assurance` / `project-memory-management` / `task-decomposition-and-execution` 五份），**不计入**「单次 ≤3 协同 reference」上限；≤3 只约束领域协同 reference。

---

## 子技能索引（`@<标识>` → `references/<标识>.md`）

| 子技能 | 英文标识 | 文件 |
| ---------------- | ---------------------------------- | -------------------------------------------------- |
| 软件项目总控 | `software-project` | `references/software-project.md` |
| 网站项目总控 | `website-project` | `references/website-project.md` |
| API设计 | `api-design` | `references/api-design.md` |
| Bug诊断 | `bug-diagnosis` | `references/bug-diagnosis.md` |
| Karpathy编码规范 | `karpathy-coding-guidelines` | `references/karpathy-coding-guidelines.md` |
| Spec驱动开发 | `spec-driven-development` | `references/spec-driven-development.md` |
| 代码审查 | `code-review` | `references/code-review.md` |
| 代码生成 | `code-generation` | `references/code-generation.md` |
| 任务拆解与执行 | `task-decomposition-and-execution` | `references/task-decomposition-and-execution.md` |
| 技术选型 | `tech-selection` | `references/tech-selection.md` |
| 文档生成 | `doc-generation` | `references/doc-generation.md` |
| 测试用例生成 | `test-generation` | `references/test-generation.md` |
| 性能基准测试 | `performance-benchmark` | `references/performance-benchmark.md` |
| 重构建议 | `refactoring` | `references/refactoring.md` |
| 项目记忆管理 | `project-memory-management` | `references/project-memory-management.md` |
| CMS二次开发 | `cms-development` | `references/cms-development.md` |
| 前端设计 | `frontend-design` | `references/frontend-design.md` |
| MySQL数据库 | `mysql-database` | `references/mysql-database.md` |
| 项目知识图谱 | `project-knowledge-graph` | `references/project-knowledge-graph.md` |

---

## 领域路由表

先判任务领域，再选主模板；需跨领域时按协同顺序补载。领域路由后仍须执行意图三分法：信息查询只读、简单任务快速通道、复杂任务先方案确认。

| 领域 | 触发信号 | 首选 reference | 协同 reference | 路由边界 |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 代码实现 | 实现功能、补接口、写脚本、改逻辑、生成代码 | `code-generation` | `karpathy-coding-guidelines`, `test-generation`, `architecture-decision` | 如果需求未对齐或涉及完整项目，先转 `spec-driven-development` 或项目总控 |
| Bug 诊断 | 报错、异常、堆栈、日志、复现失败、运行时行为不符 | `bug-diagnosis` | `root-cause-debugging`, `code-review`, `test-generation` | 未读错误和上下文前不得直接改代码；修复前须过根因调试硬闭环（复现→假设→只修根因→回归留仓） |
| 代码审查 | review、审查、缺陷、安全风险、性能问题、可维护性 | `code-review` | `karpathy-coding-guidelines`, `refactoring` | 以发现问题为主，不默认重写实现 |
| 重构治理 | 重构、坏味道、结构混乱、重复代码、可维护性提升 | `refactoring` | `test-generation`, `code-review` | 未建立验证路径前不得扩大重构范围 |
| 测试补强 | 单元测试、集成测试、回归测试、边界用例、覆盖率 | `test-generation` | `code-generation`, `bug-diagnosis` | 先确认被测行为和预期结果 |
| 性能验证 | profiler、火焰图、benchmark、cProfile、耗时分析、内存分析 | `performance-benchmark` | `code-review`, `refactoring`, `mysql-database` | 先明确性能指标和阈值，不得无基线声称"显著提升" |
| 文档交付 | README、API 文档、部署说明、回滚说明、技术文档 | `doc-generation` | `software-project`, `api-design` | 文档不得替代实际验证证据 |
| API / 长任务接口 | REST、GraphQL、接口契约、AJAX、Init-Step-Poll、轮询 | `api-design` | `frontend-design`, `cms-development` | 长任务必须采用 Init-Step-Poll 架构 |
| Laravel / PHP框架 | Laravel、Eloquent、Blade、artisan、Migration、Form Request、Queue、PHPUnit、PHPStan | `laravel-development` | `code-generation`, `test-generation`, `laravel-testing`, `mysql-database` | 仅在确认 Laravel 项目后加载；不得套用到未确认框架的 CMS 项目 |
| Java / Spring | Java、Spring Boot、Spring、MyBatis、Hibernate、JPA、Maven、Gradle、JUnit、Mockito、JVM、GC、线程池、并发 | `java-development` | `code-generation`, `code-review`, `test-generation`, `performance-benchmark` | 仅在确认 Java 项目后加载；Java 17/21 特性必须先确认运行版本支持 |
| JavaScript | JavaScript、JS、ES6、ES module、CommonJS、JS 风格规范、JS 代码检查、全角符号修复、PHP 内联 JS、window.open、onclick 事件属性 | `javascript-development` | `code-generation`, `code-review`, `bug-diagnosis`, `api-design`, `software-project` | 仅在确认 JS 项目后加载；TypeScript 项目由其类型系统承担约束；Node.js 版本需 >= 15（推荐 18+）；JS 写在 PHP 文件内联时须额外走「CMS / PHP 内联 JS」三道强制校验（引号配对 / window.open features 非空收尾 / 批量全仓 Node 校验） |
| CMS / PHP | CMS、EmpireCMS、WordPress、ThinkPHP、PHP8兼容、插件、模板 | `cms-development` | `mysql-database`, `bug-diagnosis`, `code-generation` | 未确认 CMS 类型前禁止生成框架特定代码 |
| MySQL / 数据库 | 表结构、SQL、索引、事务、慢查询、EXPLAIN、迁移、回滚 | `mysql-database` | `cms-development`, `software-project` | 写操作必须先确认影响范围和回滚路径 |
| 前端 / 视觉 | UI、UX、页面、响应式、品牌、Banner、图标、浏览器验证 | `frontend-design` | `code-generation`, `website-project`, `design-critique` | 先输出设计约束，再进入实现 |
| 设计评审 | 评审、critique、设计打分、启发式评估、认知负荷、界面文案、UX 写作、可用性审计、设计走查、帮我挑毛病 | `frontend-design` | `design-critique`, `code-review` | 评审是只读活动，先出分级问题清单（P0-P3），不默认重写设计；存在未决 P0/P1 不得宣称"设计通过" |
| 项目总控 | 完整功能、项目交付、上线、发布、运维、监控、巡检 | `software-project` 或 `website-project` | `task-decomposition-and-execution`, `doc-generation`, `production-readiness` | 先定边界、验收和发布回滚，再拆任务 |
| 项目记忆 | 项目记忆、上下文恢复、决策记录、规范沉淀、继续上一轮 | `project-memory-management` | 当前主任务 reference | 项目记忆只辅助主任务，不替代主任务交付 |
| 领域建模 | 领域驱动、领域建模、限界上下文、聚合根、领域事件、统一语言、防腐层、复杂业务规则建模、遗留系统隔离 | `domain-driven-design` | `software-project`, `spec-driven-development`, `architecture-decision` | 仅复杂业务系统（多子域/术语多义/跨上下文）加载；CRUD 脚本/营销页不加载，避免过度设计 |
| 分布式系统 | 分布式事务、CAP、一致性、Saga、TCC、2PC、最终一致、幂等、分布式锁、消息驱动、事件驱动、跨服务事务 | `distributed-systems` | `code-generation`, `architecture-decision`, `api-design` | 仅跨服务/跨库/消息驱动场景加载；单机单库 CRUD 不加载，避免过度设计 |
| 根因调试 | 修 bug、排查报错、复现问题、回归失败、RCA、debug、生产事故定位、"为什么 X 不工作" | `bug-diagnosis` | `root-cause-debugging`, `test-generation` | 先复现失败再修根因；无失败测试的修复不闭环；禁止症状修补 |
| 阶段导航 | 下一步该做什么、现在在哪一步、帮我串起来、从需求到交付、完整跑一遍、失败后该回哪一步 | `dev-navigation` | `task-decomposition-and-execution`, `project-memory-management` | 只指路不代跑：不写代码、不产出 artifact、不调起其他子技能 |
| 事故复盘 / SLO | 事故复盘、线上故障、incident、SLO、错误预算、error budget、可用性、告警阈值、燃烧率 | `incident-review` | `root-cause-debugging`, `architecture-decision` | 复盘不甩锅；未落行动项的复盘视为未完成；SLO 是业务决策 |
| 威胁建模 / 供应链 | 威胁建模、STRIDE、安全设计、攻击面、依赖审计、供应链安全、SBOM、npm audit、composer audit、第三方库审查、引入依赖、加依赖、新依赖、装包、安装 xxx 包、引入 xxx 库 | `threat-modeling` | `code-review`, `cms-development`, `mysql-database` | 威胁建模是设计活动；引入依赖前先审查；未缓解高危威胁必须显式披露 |
| 生产就绪 / 发布 | 上线前检查、生产就绪、PRR、灰度发布、金丝雀、canary、分批发布、发布门禁、回滚方案 | `production-readiness` | `incident-review`, `performance-benchmark`, `threat-modeling` | 没有回滚方案不上线；PRR 未通过不得强行发布；渐进式发布是生产发布默认路径 |
| 技术战略 / 演进 | 技术路线图、技术规划、技术债、技术债台账、偿还节奏、平台化、中台、抽象时机、弃用、下线、deprecated、兼容窗口、版本演进 | `technical-strategy` | `architecture-decision`, `engineering-metrics`, `tech-influence`, `project-memory-management` | 只出判据与模板；组织级战略决策与资源调配不在范围；「下一步」写不出启动条件即移入「明确不做」 |
| 技术影响力 / 评审 | RFC、技术方案评审、架构评审、评审材料、技术标准、技术规范、规范草案、方案对齐、跨团队方案、评审意见 | `tech-influence` | `architecture-decision`, `ai-coding-governance`, `doc-generation`, `production-readiness` | 只产出载体（文档）与评审流程动作；无「替代方案 + 回滚路径」的材料不得送评审；不替代组织决策 |
| 效能 / 成本度量 | 研发效能、DORA、交付周期、部署频率、变更失败率、MTTR、技术债量化、容量、成本、ROI、投入产出、回收期 | `engineering-metrics` | `technical-strategy`, `performance-benchmark`, `incident-review`, `production-readiness` | 指标只用于定位瓶颈、禁用于个人绩效；与 SLO / 性能基线分工不重复计算；口径不清不给数字 |
| 项目知识图谱 / 依赖影响面 | 跨模块改动、接手陌生项目、依赖分析、影响面、画依赖图、模块关系、谁依赖谁 | `project-knowledge-graph` | `task-decomposition-and-execution`, `code-review` | 图谱**仅加速器**：动刀前 grep 复核不能省（G1/G2'/G3）；简单任务 / 单文件不加载 |
| 技术选型 | 技术选型、选型、技术栈、框架选型、用什么框架、用什么库、方案对比选型 | `tech-selection` | `architecture-decision`, `threat-modeling` | 只出候选与取舍判据；涉新依赖引入须先过威胁建模的引入前审查 |

---

## 子技能优先级矩阵

当用户输入同时匹配多个子技能时，按以下优先级路由：

| 场景 | 优先子技能 | 理由 |
| ------------------------------------------------------ | -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| "做网站/建站/企业官网/营销页" | 网站项目总控 | 网站项目需要先覆盖项目启动、站点规划、SEO、部署、验收和运维，再拆解执行 |
| "API服务/后端服务/CLI工具/数据脚本/插件项目/完整功能" | 软件项目总控 | 非网站类完整项目需要先覆盖边界、架构、验证、发布和交付，再拆解执行 |
| "帮我看看这段代码有什么问题" | 代码审查 | 通用审查优先于专项重构 |
| "这段代码有坏味道/代码异味" | 重构建议 | 专项关键词触发专项技能 |
| "帮我修复这个Bug" | Bug诊断 | 明确修复意图优先于审查 |
| "帮我写段代码" + 提到测试 | 代码生成 | 先生成主代码，再生成测试（代码生成→测试用例生成 协同） |
| "设计API" + 提到技术选型 | 技术选型 | 选型先于设计（技术选型→API设计 协同） |
| "重构" + 提到测试 | 重构建议 | 先重构，再补测试（重构建议→测试用例生成 协同） |
| "写文档" + 提到API | 文档生成 | 通用文档优先，API专项由 API设计 协同 |
| 任何子技能 + "记录决策" | 当前子技能 + 项目记忆管理 | 主任务优先，记忆作为附属步骤 |
| "CMS二次开发" + "写代码" | CMS二次开发 + 代码生成 | CMS规范优先，代码生成遵循CMS数据访问层和安全红线 |
| "帝国CMS/WordPress" + "报错" | Bug诊断 | CMS关键词触发Bug诊断时自动加载CMS常见Bug模式 |
| "PHP" + "代码审查" | 代码审查 + CMS二次开发 | 审查PHP代码时自动追加CMS安全审查清单 |
| "Laravel/Eloquent/Blade/artisan" + "写代码/改功能" | 代码生成 + Laravel专项参考 | Laravel 框架约定优先，按需加载 `laravel-development` |
| "Laravel/PHPUnit/Pest/Feature Test" + "测试" | 测试用例生成 + Laravel测试参考 | Laravel 测试优先使用 Feature Test、Factory、Facade fake 和数据库断言 |
| "Java/Spring Boot/MyBatis/JPA" + "写代码/改功能" | 代码生成 + Java专项参考 | Java/Spring 分层、事务、数据访问和异常处理规则优先，按需加载 `java-development` |
| "Java/JVM/GC/线程池/并发" + "性能/调优" | 性能基准测试 + Java专项参考 | JVM 与并发问题必须先采集耗时、GC、线程、堆或连接池证据 |
| "Java/JUnit/Mockito/Spring Boot Test" + "测试" | 测试用例生成 + Java测试参考 | Java 测试优先区分单元测试、切片测试、集成测试和外部依赖替身，按需加载 `java-testing` |
| "JavaScript/ES6" + "写代码/改功能" | 代码生成 + JS专项参考 | JS 架构规则、模块系统、JS语法检查优先，按需加载 `javascript-development` |
| "JavaScript" + "审查/检查/review" | 代码审查 + JS专项参考 | 审查 JS 代码时自动加载 JS 代码风格规范和代码质量检查流程 |
| "JS代码检查/全角符号/语法检查/兼容检查" | JS专项参考 | 加载代码质量检查 4 步流程（Node.js 版本检查→语法检查→修复→验证） |
| "MySQL/数据库/SQL/索引/慢查询/EXPLAIN" | MySQL数据库 | 数据结构、SQL安全和性能问题优先走数据库专项模板 |
| "代码优化/性能优化/架构优化/N+1/缓存/异步/性能瓶颈" | 性能基准测试 + 代码审查 | 先按性能反模式静态扫描定位嫌疑点，再用 benchmark/profile/EXPLAIN 验证 |
| "PHP/CMS" + "数据库/SQL" | CMS二次开发 + MySQL数据库 | 先确认CMS访问层和表前缀，再进行SQL/索引/迁移设计 |
| "部署/发布/上线/回滚/运维/监控/告警/巡检" | 软件项目总控 + 文档生成 + 生产就绪（发布/回滚） | 发布运维类请求必须输出发布步骤、回滚方案、观测指标、告警和巡检清单；上线前先过 PRR，发布走渐进式路径（`production-readiness`） |
| "前端/页面/UI" + "设计" | 前端设计 | 视觉、交互、响应式和可访问性优先于直接写代码 |
| "品牌/Banner/图标/社媒图" | 前端设计 | 视觉资产类请求由前端设计输出规格、风格、尺寸和验收标准 |
| "前端设计" + "写代码" | 前端设计 + 代码生成 | 先定义页面结构/组件状态/响应式，再生成实现代码 |
| "前端/页面/UI" + "评审/打分/挑毛病" | 前端设计 + 设计评审专项 | 复杂页面/关键流程先走 10 透镜 + 启发式评分卡 + 认知负荷审计，出分级问题清单（`design-critique`） |
| "界面文案/按钮文案/UX写作/空状态文案" | 前端设计 + 设计评审专项 | UX 文案规范优先：按钮动词化、错误三步结构、空状态带 CTA、术语全站一致（`design-critique` 第三部分） |
| "CMS模板" + "页面设计" | 前端设计 + CMS二次开发 | 同时约束视觉实现、模板变量、输出转义和缓存策略 |
| "前端安全/XSS/CSRF/CSP" | 前端设计 | 前端产物必须通过安全红线检查：输出转义、Token、敏感信息不入前端、接口权限后端兜底 |
| "spec/需求对齐/需求规格" | Spec驱动开发 | 编码前必须先对齐需求规格，生成 Scenario 和验收标准 |
| "任务分解/Wave执行" | 任务拆解与执行 | 复杂需求应拆分为原子任务按Wave分组，避免单次执行超限 |
| "Karpathy/编码哲学/简洁优先" | Karpathy编码规范 | 编码哲学优先于具体实现——思考先于编码、简洁先于完备 |
| "profiler/火焰图/benchmark/cProfile/耗时分析/内存分析" | 性能基准测试 | 定量性能验证优先于代码审查的静态推断 |
| "review" + "性能" | 代码审查 + 性能基准测试 | 先静态审查发现嫌疑点，再用 benchmark 定量验证 |
| "重构" + "基准/对比" | 重构建议 + 性能基准测试 | 重构前跑基线，重构后跑对比，量化收益 |
| "AJAX防卡死/Init-Step-Poll/长任务/轮询" | API设计 + 前端设计 + CMS二次开发 | 长任务必须先定义 Init/Step/Poll 接口契约，再实现前端轮询和 CMS 分批处理 |
| "跨模块改动/重构/审查/接手陌生项目" + "依赖/影响面" | 项目知识图谱 + 当前子技能 | 复杂任务（≥3 文件）先查图谱依赖闭包（P1 Step2 规划前置），改码前查上游影响面（P2 Step3），图谱仅作加速器、动刀前 grep 复核（G1/G2'/G3 硬门禁） |
| "修 bug/排查报错" + 要求根治 | Bug诊断 + 根因调试硬闭环 | 复现失败→假设列表→只修根因→回归留仓，禁止症状修补（`root-cause-debugging`） |
| "下一步该做什么/现在在哪一步/帮我串起来" | 阶段导航 | 只输出阶段检测与下一步建议，不写代码不产出 artifact（`dev-navigation`） |
| "事故复盘/线上故障/复盘" | 事故复盘 + 根因调试硬闭环 | 时间线→影响→根因→行动项闭环，不甩锅；未落行动项视为未完成（`incident-review`） |
| "SLO/错误预算/可用性/告警阈值" | 事故复盘（SLO 部分） | SLI 可采集→SLO 业务决策→错误预算治理→燃烧率告警（`incident-review`） |
| "威胁建模/安全设计/STRIDE/攻击面" | 威胁建模 | DFD 画信任边界→STRIDE 逐类过→缓解落代码→未缓解高危显式披露（`threat-modeling`） |
| "依赖审计/供应链/SBOM/第三方库审查" | 威胁建模（供应链部分） | 引入前审查门禁→lockfile 锁定→常态化 audit→SBOM 与签名校验（`threat-modeling`） |
| "引入依赖/加依赖/新依赖/装包/安装 xxx 包/引入 xxx 库" | 威胁建模（供应链部分） | 先过引入前审查门禁（作者/维护度/许可/已知漏洞）再装，lockfile 锁定（`threat-modeling`） |
| "上线前检查/生产就绪/PRR" | 生产就绪审查 | 可观测/可靠性/容量/安全/数据/流程六维检查；阻断项不发布（`production-readiness`） |
| "灰度发布/金丝雀/分批发布/回滚" | 生产就绪（渐进式发布部分） | 金丝雀/灰度默认路径→放量门禁→回滚条件预写→发布后观察（`production-readiness`） |
| "技术债/偿还节奏/什么时候还债/平台化/要不要抽象" | 技术战略 + 架构决策 | 先登台账（含利息）→ 20% 配额排序 → 平台化三条件同时满足才做；偿还后回归验证（`technical-strategy`） |
| "写 RFC/技术方案评审/评审材料/技术规范草案" | 技术影响力载体 + 架构决策 | RFC 七段（含替代方案与回滚）+ 评审前必答 8 问；缺第 4/6 段不得送评审（`tech-influence`） |
| "研发效能/DORA/交付周期/变更失败率/MTTR/成本/ROI" | 效能度量 | 四指标齐全且写清分母；指标只定位瓶颈、禁用于个人绩效（`engineering-metrics`） |

**互斥规则**：

- 代码审查 vs 重构建议：用户说"问题/缺陷/漏洞" → 代码审查；用户说"坏味道/异味/重构" → 重构建议
- Bug诊断 vs 代码审查：用户说"报错/Bug/崩溃" → Bug诊断；用户说"审查/检查/review" → 代码审查
- 代码生成 vs 重构建议：用户说"实现/写/生成" → 代码生成；用户说"重构/优化/改" → 重构建议

---

## 协同顺序规则

- 网站项目：网站项目总控 → Spec驱动开发 → 任务拆解与执行 → 前端设计/CMS二次开发/API设计 → 代码生成 → 代码审查 → 测试用例生成 → 文档生成 → 项目记忆管理
- 通用软件项目：软件项目总控 → Spec驱动开发 → 技术选型/API设计/MySQL数据库/CMS二次开发 → 任务拆解与执行 → 代码生成 → 代码审查 → 测试用例生成 → 文档生成/部署运维说明 → 项目记忆管理
- 需求→实现：Spec驱动开发 → 任务拆解与执行 → 代码生成 → 代码审查 → 性能基准测试 → 测试用例生成 → 文档生成
- 页面→实现：前端设计 → 代码生成 → 代码审查 → 性能基准测试 → 测试用例生成 → 文档生成
- 设计→评审：前端设计 → 设计评审（10 透镜 + 启发式评分卡 + 认知负荷审计 + UX 文案检查）→ 代码生成 → 代码审查
- CMS页面→实现：CMS二次开发 → 前端设计(含浏览器验证) → 代码生成 → 代码审查 → 测试用例生成
- Laravel功能→实现：Laravel专项参考 → 代码生成/API设计/MySQL数据库 → 代码审查 → 测试用例生成 + Laravel测试参考 → 文档生成
- Java功能→实现：Java专项参考 → 代码生成/API设计/MySQL数据库 → 代码审查 → 测试用例生成 → 性能基准测试/文档生成
- JS功能→实现：JS专项参考 → 代码生成/API设计 → 代码审查 → 测试用例生成 → 文档生成
- JS代码质量→验证：JS专项参考（代码质量检查流程）→ 代码审查 → 代码生成（修复）→ JS专项参考（验证）
- 代码优化→验证：代码审查(性能反模式) → 性能基准测试(L0/L1/L2) → MySQL数据库/重构建议 → 测试用例生成 → 项目记忆管理
- 治理→沉淀：Bug诊断/重构建议 → 代码审查 → Karpathy编码规范 → 项目记忆管理
- Bug修复→闭环：Bug诊断 → 根因调试硬闭环（复现→假设→只修根因→回归留仓） → 项目记忆管理
- 事故→治理：事故复盘 → 根因调试硬闭环 → 生产就绪（回滚/门禁复盘）→ 项目记忆管理
- 设计→安全：Spec驱动开发/API设计 → 威胁建模 → 代码审查 → 代码生成 → 测试用例生成
- 发布→交付：生产就绪（PRR）→ 渐进式发布（金丝雀/灰度）→ 事故复盘（发布后观察）→ 项目记忆管理
- 战略→演进：`technical-strategy`（路线图/技术债台账）→ `tech-influence`（RFC 承载公告与迁移期）→ 生产就绪（弃用移除走渐进式）→ 项目记忆管理
- 效能→治理：`engineering-metrics`（DORA + 债务利息）→ `technical-strategy`（排序与 20% 配额）→ 架构决策（偿还方案取舍）→ 项目记忆管理
- 方案→评审：架构决策 → `tech-influence`（RFC 七段）→ 威胁建模/性能基准（影响面证据）→ 生产就绪 → 项目记忆管理
- 陌生项目→摸底：项目知识图谱（P1 全量依赖闭包喂 plan）→ 任务拆解与执行 → 代码审查 → 项目记忆管理

---

## 专项 reference 映射

以下专项 reference 只作为领域协同资料按需加载，**不加入 `@英文标识` 显式调用索引，也不计入子技能总数**。领域路由命中这些标识时，按本表读取真实文件路径，禁止依赖裸标识自行推断。

| 专项 reference | 文件 |
| ------------------------ | ---------------------------------------- |
| `laravel-development` | `references/laravel-development.md` |
| `laravel-testing` | `references/laravel-testing.md` |
| `java-development` | `references/java-development.md` |
| `java-testing` | `references/java-testing.md` |
| `design-audit` | `references/design-audit.md` |
| `javascript-development` | `references/javascript-development.md` |
| `execution-safety` | `references/execution-safety.md` |
| `delivery-assurance` | `references/delivery-assurance.md` |
| `error-ledger` | `references/error-ledger.md` |
| `style-alignment` | `references/style-alignment.md` |
| `architecture-decision` | `references/architecture-decision.md` |
| `domain-driven-design` | `references/domain-driven-design.md` |
| `distributed-systems` | `references/distributed-systems.md` |
| `root-cause-debugging` | `references/root-cause-debugging.md` |
| `design-critique` | `references/design-critique.md` |
| `dev-navigation` | `references/dev-navigation.md` |
| `incident-review` | `references/incident-review.md` |
| `threat-modeling` | `references/threat-modeling.md` |
| `production-readiness` | `references/production-readiness.md` |
| `ai-coding-governance` | `references/ai-coding-governance.md` |
| `code-readability-for-agents` | `references/code-readability-for-agents.md` |
| `llm-application-security` | `references/llm-application-security.md` |
| `technical-strategy` | `references/technical-strategy.md` |
| `tech-influence` | `references/tech-influence.md` |
| `engineering-metrics` | `references/engineering-metrics.md` |
| `agent-reasoning-patterns` | `references/agent-reasoning-patterns.md` |
