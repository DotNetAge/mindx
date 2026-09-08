# PR : 由提示词带来的过度耦合


MindX 系统由核心框架（goharness）、应用逻辑（mindx）以及用户界面(mindx-app)三部分组成。

由于当时设计goharness 考虑得过于复杂，goharness 现时将 system prompt 的生成完全包含其中，带来的问题就是：

1. 只有部分系统提示词是继续使用 goharness 原有的逻辑，而在 mindx 想要重定义部分的系统提示就变得比较困难，从而就去改动 Agent注册器，Skill 注册器等组件。这样一来反而导致 Agent注册器，Agent的结构，Skill 注册器和Skill结构都完人与goharness 的内核紧紧地耦合在一起。
2. 现在我们想改变 Agent 与 Skill 的存储结构时，又必然会影响到 goharess 的逻辑，这就暴露出将 system prompt 的生成放在 goharness 是不合适的。最正确的做法应该是由 mindx 端将完整的 system prompt 传递给 goharness,这样应该可以将 Agent, AgentRegister, Skill , SkillRegister与 GoHarness 的耦合度大大降低。
3. 由应用引发的一个需求就是将 Agent 的文件结构与Skill的文件结构有机地结合起来。其实我们现在正在跑的关系本来在逻辑上也是走向了这样的方向：

以下为Agent, Skill, Tool 的逻辑从属关系：

```
- Agent
  - Skills
  - Tools
```

理由是，我将 Agent 看作一个“人”，工具是人天生的“本能”是由 goharess 提供的，Tools的定义与扩展是由我们官方掌握，因此改变的频次较低。而 Skill 却是可以根据对话历史、用户的新需求等动态生成，又或从外部下载引入；完全像人一个“技能”是由“后天”学习获得的。
 
Tools的提示词是由goharness自己管理这是没有问题，因为Tools的有非常多内置的机制需要由goharness 来管理。

重点在于 Agent 与 Skill 的定义。它们现在是被独立存放在安装后的用户目录下 `~/.mindx/agents` 和 `~/.mindx/skills` 中。 从文件的存储结构上它们是隔离的； Agents与 tools 及 skills 的关系建立都是通过 agent 文件来配置实现的。

说了这么多，以上的全部是背景信息，其实我是在考虑如果将Agent与 Skills 的本质入手，将 system prompt 的生成移动至mindx内，那么我们是不是可以将Agent 与 Skill进行更大结构上变化。如：将Agent 从一个文件定义变成改变成一个目录，如：

```
- agents
  - <agent-name>
    - AGENTS.md # 所有AGENTS都必须共同遵守的规则
    - IDENTITY.md # 身份定义 + frontmatter 嵌入 Agent的meta信息
    - SOUL.md # 核心逻辑定义又可以称为Agent的行为规则及负责范围
    - skills
      - <skill-name>
        - SKILL.md # Skill的定义
```

这样做的好处是，便于分发！同样也是前文说了这么多问题的主要成因。如果我们可以将Agent,Skill进行共同打包，那么我们就可以很方便地将构建 Agent市场与Skill市场；

但这样做同样有坏处，那就是“冗余”，因为 Agent 与 Skill 的最佳关系其实是“引用”而不是“包含”。不同Agent应该可以共享同一个Skill。而且，考虑到 Skill是可以更新或者被删除的，那么Agent 与 Skill 的关系也应该可以动态地建立与销毁。

包含关系的好处是不会因为本地Skill库中缺失某个Agent所需要的Skill而导致 Agent 的功能缺失；

最后 `AGENTS.md` 这个文件不一定要包含在 agent 目录内，毕竟它是全部Agent的共同规则，我们可以在mindx内将这个Agents.md的内容作为一个变量内嵌在包内；

---

## 讨论共识（2026-09-06）

以下为对上述问题的讨论结论，作为后续实施与继续深入讨论的基准。

### 一、System Prompt 职责切分（已定）

**原则：goharness 只保留 tools 机制段，其余段落全部由 mindx 拼装传入。**

- 根因确认：`goharness/agents/prompt_assembler.go` 的 `BuildSystemPrompts` 将身份、技能目录、行为规则、搜索优先级四段硬编码，直接依赖 `AgentConfig` / `skill.Skill` 的结构；`skillsCatalogBuilder` / `envsBuilder` / `searchStrategyBuilder` 三个 override 钩子正是"改提示词 → 动注册器"耦合拉锯的证据。
- 环境信息段（SessionID / SessionDir / ProjectDir）移交 mindx：这些数据本就是 mindx 创建会话时提供的，`~/.mindx` 的路径约定也是 mindx 的地盘，goharness 只是经手人，不应成为叙述者。
- **决定拆除 MicroCompact（微压缩 / 局部压缩）**：它通过修改上下文中间消息来去重，会破坏 KV 缓存，是负优化，偏离了压缩的原意。拆除后"压缩内容占位符"段随之消失。TryCompact（全量摘要 + 清空窗口）保留，因其不修改中间消息。
- mindx 侧的 system prompt 生成规则（已定）：
  - 生成逻辑放在**独立的类 / 方法**中，用**独立文件**保存，便于维护；
  - 拼接顺序：**IDENTITY.md → SOUL.md → Skill summary list（技能摘要目录）→ AGENTS.md（内嵌变量）→ Env（环境信息）**。该顺序满足"静态在前、动态在后"，KV 缓存前缀稳定。
- Memory 注入保留在 goharness（已定）：记忆注入走 Hook 路径（`hooks/loop/memory_hook.go`），与 base prompt 的静态拼装是两条不同路径；注入内容随轮次动态变化，适合由 Hook 在请求时机承载，不适合并入 mindx 的静态拼接。
- goharness 仅追加 tools 机制段（工具 schema、沙箱与命令白名单说明）。
- 实施注意：`memory_hook` 依赖 `SystemPromptSections` 末段的假设，段落布局定稿后需重新核对。

### 二、Agent 目录化存储（已定）

```
- agents
  - <agent-name>
    - IDENTITY.md        # 唯一 meta 入口：frontmatter（强类型一级字段）+ 身份正文
    - SOUL.md            # 核心逻辑：行为规则与负责范围，正文自带标题（不注入外部标题）
    - skills/            # Agent 独有技能库
      - <skill-name>/
        - SKILL.md       # 技能定义
```

- AGENTS.md（全体 Agent 共同规则）不放入 agent 目录，作为变量内嵌在 mindx 包内，与应用版本绑定；per-agent 覆盖机制暂不设计。
- frontmatter 只出现在 IDENTITY.md，SOUL.md 纯正文但**自带标题**（评审定案：基于文件的段不注入外部标题，标题由定义文件自身处理，避免标题空悬），meta 不散落两处。
- **Agent 属性强类型化**：现 AgentConfig 字段不足，Icon、Domains 等被迫塞进 Metadata map，修改麻烦且失去强类型语义。目录化重构时将这些提升为 frontmatter 一级字段（name / role / description / introduction / model / icon / domains / skills / exclude_tools ...），Metadata map 仅保留真正的自由扩展杂项。
- **Agent 模型所有权归 mindx**：完整的强类型 Agent 结构（含 icon / domains 等应用层属性）由 mindx 定义与解析。exclude_tools 也下放为**会话创建参数**（`[]string`）——goharness 在工具装配时只消费这个会话级过滤参数，对 Agent **零结构依赖**：`AgentConfig` / `AgentRegistry` 在 goharness 侧全部退役，Agent 结构变更不再触碰 goharness。
- **SkillRegistry 同构收窄（接口注入而非会话参数）**：goharness 保留一个运行时 SPI——Skill 工具的检索契约（`GetSkill(name) → Instructions / AllowedTools`），实现全部由 mindx 注入（三级库解析、同名覆盖、原子替换热加载都在 mindx 实现内）；skill 包的目录加载器与 SKILL.md 解析迁 mindx，`skill.Skill` 在 goharness 瘦身为运行时字段（存储字段归 mindx）。不走会话参数的原因：exclude_tools 是建会话时的一次性静态参数，而 skill 是 LLM 运行中主动检索且热加载需实时生效（`ReloadSkills` 原子替换即刻作用于所有会话），会话快照会改变热加载语义。顺带清理 `action.Defaults` 中未使用的 skillRegistry 死参数。
- Skill 引用声明放 IDENTITY.md frontmatter（沿用 `AgentConfig.Skills` 的思路，单一入口）。
- **分发格式 ≠ 存储格式**：存储 / 运行时是纯引用（共享、动态建立与销毁都靠引用列表操作）；"包含"只发生在传输层。分发包（agent 目录 + 所引用 skills 的副本，用于市场分发 / 下载的自包含产物）安装时展开：agent 进 `agents/<name>/`，skills 按名去重进全局库，agent 的引用列表指向本地。冗余只存在于传输层，落地即消除。
- 分发包同名冲突：Skill 暂无版本管理策略。分发包内的 skills 与本地全局库同名时，不展开进全局库，而是放进该 Agent 目录的 `skills/` 内；运行时同名 skill 只注册 Agent 内的版本（Agent 级覆盖全局级），实现"逻辑重写"的效果。
- 运行时技能缺失：加载降级（跳过 + 警告），不废 Agent。不使用 symlink（Windows / 打包工具处理 symlink 是坑）。
- 旧单文件 agent 一次性迁移为目录（转换 + 备份），加载器不留双格式分支。

### 三、技能三级库与经验晋升管线（已定）

Skill 的本质是一种**经验升级机制**，与人的经验获得过程非常相似：

| 层级       | 位置                           | 性质                                                      | 进入方式                                                        |
| ---------- | ------------------------------ | --------------------------------------------------------- | --------------------------------------------------------------- |
| 全局库     | `~/.mindx/skills`              | 被验证过的"最佳实践"经验，如图书馆，其它 Agent 可随意引用 | 声明式引用（IDENTITY frontmatter）                              |
| Agent 级库 | `agents/<name>/skills`         | Agent 在长期对话过程中获得的私有经验                      | 用户放置 / 分发包同名落地 / 自省生成（见 PR-SELF-EVOLUTION.md） |
| 项目级库   | `<ProjectDir>/.skills`（隐藏） | 一种"可能"的经验，尚未充分验证                            | 发现式，仅在该项目内可见                                        |

**晋升管线（一切晋升由用户裁决）**：

1. 项目级 → Agent 级 / 全局级：是否装配进 Agent 由用户说了算。两条交互路径：
   - 加载 Session 时发现 ProjectDir 下有隐藏技能，**批量确认，一次性载入**；
   - 在 Agent 管理器中展示当前目录下的技能，由用户选择技能的升级路径（Agent 级还是全局级）。
2. Agent 级 → 全局级：由用户决定是否有必要将其提取出来进入全局 Skill 库（在界面中支持，由用户操作）。
3. 全局级：任何 Agent 随意引用。

Agent 级与项目级技能的**生成机制（自省 / 自进化）不属于本次重构**，已拆分至独立 PR：见 [PR-SELF-EVOLUTION.md](./PR-SELF-EVOLUTION.md)。本节只固化三级库的存储结构与晋升管线。

### 四、安装与导出闭环（mindx-app，已定）

- **入口**：AgentBrowser 负责 Agent 分发包的**本地导入安装**与**在线安装**；SkillManager 负责 Skill 包的导入安装。两者同样提供**导出**能力，形成闭环。
- **导出保真原则**：Agent 导出时打包其**实际生效**的技能版本——引用的全局 skills 取全局库副本，Agent 级 skills（含同名覆盖的）按 Agent 级版本打包，保证分发包的行为与本地运行时一致（"逻辑重写"不漂移）。
- **包格式同构**：Skill 包是 Agent 分发包的子集（同一容器格式 + 清单），安装器复用同一套展开逻辑：Agent 包按第二节规则展开（agent 进 `agents/<name>/`，skills 按同名/去重规则落地），Skill 包直接进全局库。
- **在线市场（已定）：COS 静态市场**。托管于腾讯云 COS（bucket `repo-1257961037`），以 manifest 清单实现简单静态市场：清单列出可安装的 Agent / Skill 分发包（名称、描述、icon、下载地址、sha256 完整性校验），客户端拉取清单展示列表，下载分发包后复用本地安装的同一套展开逻辑。
- **发布流程**：导出分发包 → 上传 COS → 更新 manifest（人工 / 脚本化，无需服务端）。mindx-app 的发布文件现已托管于 COS（默认桶 `mindx-1257961037`，见 `mindx-app/scripts/publish-cos.mjs`），市场桶 `repo-1257961037` 与其同属一个账号，托管与上传模式直接沿用该先例。清单中的 sha256 仅用于传输完整性校验，不构成版本管理（与"Skill 暂无版本管理策略"一致）。
- **演进路径**：先观察市场反应，再考虑演进为动态服务；manifest 结构预留扩展字段即可，不做超前设计。清单拉取失败时降级使用本地缓存并提示。

### 五、可行性验证（2026-09-06 代码核对结论）

**总评：四个板块全部可行，无颠覆性障碍。**关键核对发现如下：

1. **职责切分（可行，且根因被实锤）**：
   - goharness `PromptAssembler` 七段结构与三个 override 钩子确认（`agents/prompt_assembler.go`）。
   - **mindx 已在使用这三个钩子**：`internal/svc/daemon.go:91-101` 调用 `SetSkillsPromptOverride / SetEnvsOverride / SetSearchStrategyOverride`，经 `internal/core/app.go:634-645` 转换为 goharness `agents/options.go:80-101` 的 `WithSkillsPrompt / WithEnvs / WithSearchStrategy`——"改提示词 → 动组件"的拉锯已实际发生。本次重构即把"逐段 override"收敛为"一次性 base prompt"，随后删除三个钩子。
   - goharness 尚无 base prompt 注入点，需新增一个 Runtime option（如 `WithBaseSystemPrompt`），`BuildSystemPrompts` 收窄为 `base 段 + tools 机制段`。
   - `memory_hook`（`hooks/loop/memory_hook.go:43-53, 111-118`）将"## 历史对话摘要"追加到最后一条 system 消息末尾，依赖末段布局——段落布局定稿后需核对该追加位置。

2. **MicroCompact 拆除（可行，触点清单明确）**：`agents/prompt_assembler.go:15-37`（阈值与判断）、`agents/base_rules.go:73-82`（占位符文案）、`agents/executor.go`（TryMicroCompact 调用）、`session/session.go:204-223`（会话层压缩状态与回调）及相关测试。TryCompact 链路独立，拆除互不影响。

3. **Agent 目录化（可行，现状吻合度高）**：
   - AgentRegistry 现为"Markdown + YAML frontmatter 单文件（`{name}.md`，正文即行为规则）"（`config/agent_registry.go`、`config/agent_loader.go`），拆为 IDENTITY.md + SOUL.md 是自然演进；
   - skill 加载器已是 `<skill-name>/SKILL.md` 目录结构（`skill/loader.go`），Agent 目录内 skills/ 与全局库同构、加载器可复用；
   - 需适配：agent_loader 改目录扫描、hotreload_watcher（`internal/svc/hotreload_watcher.go:149-231` 目前仅对 `.md` 文件事件触发）、`agent.get / agent.update` RPC 字段映射（`internal/svc/handler_agent.go`、`pkg/rpc/agent.go`）;
   - 强类型化痛点实证：`AgentConfig` 无 icon / domains 字段，前端从 `meta` 读取（`AgentBrowser/index.vue:60-75`）。

4. **技能三级库（可行）**：全局库与 Agent 级库直接复用 skill 加载器；项目级库（`<ProjectDir>/.skills`）为纯新增（扫描 + 批量确认 RPC/UI），ProjectDir 传递链路已存在（`session.create`）。

5. **安装导出闭环（可行）**：RPC 通道现成（`internal/svc/handler_registry.go:14-136` 已注册 agent/skill 全套管理方法）；前端数据流已有（`connectionStore.ts:278-352`）；市场上传脚本可照 `mindx-app/scripts/publish-cos.mjs`（默认桶 `mindx-1257961037`）扩展；"在线模型对话框"的拉列表 → 置灰 → 添加交互模式可复用。

### 六、重构计划

分四个阶段，依赖顺序 P0 → P1 → P2，P3 可与 P1 并行。

#### P0 Agent 目录化存储（地基）

1. mindx 新建 Agent 存储层：目录格式读写（IDENTITY.md 强类型 frontmatter + SOUL.md + skills/），并做字段消费审计（已确认 exclude_tools 可下放为会话参数，goharness 对 Agent 零结构依赖）；
2. goharness 侧 `AgentConfig` / `AgentRegistry` 退役：exclude_tools 改为会话创建参数（`[]string`），工具装配处（`AgentExcludeTools` 消费点）改从会话参数取；agent 存储读写整体迁至 mindx；
3. skill 注册表同构收窄：goharness 保留 Skill 工具检索 SPI（`GetSkill`），目录加载器与 SKILL.md 解析迁 mindx，`skill.Skill` 瘦身为运行时字段；清理 `action.Defaults` 死参数；
4. hotreload_watcher 适配目录级事件；
5. `agent.get / agent.update` RPC 映射新格式；
6. 一次性迁移：旧 `{name}.md` → 目录（原文件备份），加载器不留双格式分支。

验收：迁移后 `agent.list / get / update` 正常，TUI 与 mindx-app 均可用。

#### P1 提示词接缝 + MicroCompact 拆除

1. mindx 新建独立 prompt 组装器（独立文件）：IDENTITY → SOUL → Skill summary → AGENTS.md（内嵌变量）→ Env；
2. goharness 新增 `WithBaseSystemPrompt`；`BuildSystemPrompts` 收窄为 base + tools 段；删除三个 override 钩子及 mindx 侧 `SetXxxOverride`；
3. 拆除 MicroCompact 全部触点（清单见第五节验证 2）；
4. memory_hook 末段追加位置核对与适配。

验收：system prompt 内容与重构前等价（debug 日志对比）、段落顺序满足"静态在前、动态在后"、TryCompact 行为不变。

#### P2 技能三级库与晋升管线

1. Agent 级库装载：agent 目录 skills/ 加载 + 同名"Agent 级覆盖全局级"注册规则；
2. 项目级库发现：`.skills` 扫描 + Session 加载批量确认 RPC + UI；
3. 晋升操作：项目级 → Agent 级 / 全局级、Agent 级 → 全局级（Agent 管理器 RPC + UI）。

验收：同名覆盖生效、项目技能批量载入、晋升路径正确落库。

#### P3 安装导出闭环 + COS 静态市场（可与 P1 并行）

1. 分发包格式定稿（容器 + 清单；Skill 包为 Agent 包子集）；
2. 导出（保真原则）与本地导入安装；
3. COS 静态市场：manifest 拉取 / 下载 / sha256 校验 / 安装 / 缓存降级；
4. `publish-market.mjs` 发布脚本（照 publish-cos.mjs 模式）；
5. AgentBrowser / SkillManager UI 集成（导入入口 + 在线列表 + 导出按钮）。

验收：导出 → 发布 → 全新环境在线安装 → Agent 可用且技能行为与导出端一致。

#### P4 规则注册表退役 + Skill SPI 收窄（2026-09-06 评审新增，定案依据见 PR-PROMPTS-REVEIWS.md）

评审核实的事实：goharness `rule` 包（862 行）数据所有权全在 mindx（规则文件在 `settings.DataRulesFile()`、RPC 管理在 handler_rule.go、权限规则合并逻辑在 app.go），其在 goharness 内部的唯一消费方是 agents 包的提示词拼接（`prompt_assembler.go:52-56` 扩展规则段）——借住关系不成立。评审同时实锤一个真实缺陷：`app.go:644` 与 `app.go:709` 两次 `WithRuleRegistry` 注入同一 RuntimeConfig 槽位，权限规则存在时用户规则被静默丢弃。

1. rule 包整体迁入 mindx：`Rule` / `RuleRegistry` 接口与 File/YAML 实现、YAML 解析移至 mindx（pkg/rules，与权限规则类型同域存放），规则存储文件位置不变；
2. goharness 侧退役：`WithRuleRegistry` option、`prompt.ruleReg` 字段、`BuildSystemPrompts` 的「扩展规则」段全部删除；rule 包从 goharness 移除；
3. mindx 侧规则收口：用户规则（`a.rules`）与权限规则（permReg）+ agent-discovery 段在 mindx 内合并渲染，经 `PromptBuilder.Build`（baseBuilder）输出——顺带消解上述槽位覆盖缺陷；
4. Skill SPI 收窄：goharness `skill.SkillRegistry` 接口删除 `RegisterSkill`，只留 `GetSkill`（Skill 工具检索契约，P0 定案不变）；`DefaultSkillRegistry` 退役；session `skillOverlay` 的注册语义改由 mindx 侧类型承载（`SetSkillOverlay` 入参类型改换或上移）；
5. 机制段保留（重申 P1 定案）：`defaultBehavioralRules` / `buildOutputEfficiency` 与 goharness 运行时机制强同步，不外放，避免版本漂移；memory hook 动态注入路径不变。

验收：goharness 全仓无 `rule` 包、无 System Prompt 应用语义拼接代码；`agent.rule` RPC 正常；用户规则与权限规则同时生效（回归覆盖槽位缺陷场景）；`go test` 两仓库全绿。
