# Agent 与 Skill 实现文档

本文档描述 mindx 中 Agent 与 Skill 的存储格式、加载规则，以及 System Prompt 的完整拼接过程。
对应重构设计见 [PR-PROMPTS.md](../PR/PR-PROMPTS.md)；分发包格式见 [AGENT-SPEC.md](./AGENT-SPEC.md)。

---

## 一、Agent 的加载规则

### 1.1 存储格式（目录化）

```
~/.mindx/agents/
  <agent-name>/
    IDENTITY.md        # 唯一 meta 入口：YAML frontmatter（强类型一级字段）+ 可选身份正文
    SOUL.md            # 行为规则与负责范围，正文自带标题（可缺省）
    skills/            # Agent 级技能库（Agent 私有经验 / 同名覆盖版本）
      <skill-name>/
        SKILL.md
```

**IDENTITY.md frontmatter 一级字段**（`agentstore.AgentMeta`，强类型）：

| 字段 | 作用 |
| --- | --- |
| `name` | Agent 唯一名（内存注册表 key） |
| `role` / `description` | 角色与职责描述（兜底角色定义、市场清单使用） |
| `introduction` | 备用说明：仅当文件无正文时生效 |
| `icon` / `domains` / `hired` | 图标、领域标签、雇佣状态（从旧 meta map 提升为一级字段） |
| `skills` | 声明式技能引用列表（全局库技能名） |
| `exclude_tools` | 工具排除列表（会话装配时经回调下发给 goharness） |
| `meta` | 真正的自由扩展杂项 |

> **模型归属**：Agent 不持有模型属性（无 `model` 字段，frontmatter 中定义也不生效）。
> 模型选择是用户级语义：`LastModel > DefaultModel`（`App.resolveModelName`），切换走 `model.switch` RPC。

### 1.2 加载流程

```
App 启动 (DefaultApp)
  └─ agentstore.Load(settings.AgentsDir())
       ├─ ① MigrateLegacyFiles(dir)              # 旧 {name}.md 一次性迁移
       │     ├─ 旧 frontmatter → IDENTITY.md（正文留空，meta.domains/icon/hired 提升为一级字段）
       │     ├─ 旧正文（行为规则）→ SOUL.md（自带标题原样保留）
       │     ├─ 同名目录已存在 → 仅备份旧文件，不覆盖
       │     └─ 迁移成功后原文件改名 {name}.md.bak
       ├─ ② os.ReadDir(agentsDir)                # 目录即注册表：只认子目录
       └─ ③ loadAgentDir(<name>/)                # 单目录损坏不拖垮整体，错误记入报告
            ├─ IDENTITY.md（必需）→ parseIdentity
            │     ├─ frontmatter → AgentMeta
            │     └─ 正文优先：正文非空时覆盖 introduction
            ├─ SOUL.md（可缺省）→ Agent.Soul
            └─ Agent{Meta, Soul, Dir} → 内存 map（key = Meta.Name）
```

**雇佣视图**：`AgentStore` 保持全量加载（目录即注册）；会话入口（TUI 选择器、ChatInput 下拉、
`resolveCurrentAgentName` 回退）只走 `HiredAgents()` 雇佣视图；管理入口（agent.* RPC、
AgentBrowser）走全量。

### 1.3 身份段生成规则（评审定案）

构建 System Prompt 身份段时的优先级：

1. **IDENTITY.md 带正文**（frontmatter 之外）→ 正文原样作为整个 Agent 的人设定义，
   标题由定义文件自身处理（不注入外部标题）；
2. **无正文** → 兜底生成：`## 角色定义 \n 我叫 {name} 是一名 {role}, {description}`
   （此段由 mindx 动态生成，标题由 mindx 注入）。

### 1.4 热加载

`hotreload_watcher` 以目录级事件监听 `~/.mindx/agents` 与 `~/.mindx/skills`（对 `.md`
变更去抖后触发）。Agent 文件变更 → `App.ReloadAgents()`：换入新 AgentStore +
置空 PromptBuilder 懒重建 + 清空 Runtime 缓存；Skill 变更 → `App.ReloadSkills()`
（仅原子替换，见下文）。

### 1.5 写路径

`agentstore.Save`（agent.update RPC / 招聘解雇）→ `renderIdentity` 全量强类型序列化
frontmatter（不丢手写字段）+ 正文同步 introduction → 写 IDENTITY.md 与 SOUL.md，
并更新内存注册表。

---

## 二、Skill 的加载规则

### 2.1 技能三级库

| 层级 | 位置 | 性质 | 进入方式 |
| --- | --- | --- | --- |
| 全局库 | `~/.mindx/skills/<skill-name>/SKILL.md` | 验证过的最佳实践，所有 Agent 可引用 | 声明式引用（frontmatter `skills`） |
| Agent 级库 | `agents/<name>/skills/<skill-name>/SKILL.md` | Agent 私有经验 | 用户放置 / 分发包同名落地 / 自省生成 |
| 项目级库 | `<ProjectDir>/.skills/<skill-name>/SKILL.md` | 尚未验证的"可能"经验 | 发现式，仅该项目内可见（Session 批量确认载入） |

技能目录结构：`<skill-name>/SKILL.md`（frontmatter：name/description + 指令正文），
目录加载器统一复用。

### 2.2 检索规则（同名覆盖链）

运行时检索由两条路径承载，检索顺序均为 **Agent 级 → 全局级**（Agent 级覆盖 = 逻辑重写）：

```
┌─ 运行时检索（Skill 工具，LLM 主动调用）───────────────────────────┐
│  SkillTool.GetSkill(name)                                        │
│    └─ Runtime.skillReg = skillstore.LiveRegistry（活视图，不快照） │
│         ├─ ① 会话 SkillOverlay（项目级技能，每轮重建重挂载）        │
│         ├─ ② Agent 级库：agents/<name>/skills/<name>/ 实时读盘     │
│         └─ ③ 全局库：ReloadGlobal 原子替换后的当前注册表指针        │
├─ 提示词目录（每次 Ask 构建 System Prompt 时）──────────────────────┤
│  skillstore.Catalog(agentName, declared)                          │
│    └─ RegistryFor 快照复制当前全局库 + Agent 级覆盖，按名排序        │
│       + 会话 SkillOverlay 项目技能并入（同名替换，标注"项目技能"）   │
└───────────────────────────────────────────────────────────────────┘
```

**热加载即刻生效**（PR 定案，禁止回退为快照）：

- `App.ReloadSkills()` → `Store.ReloadGlobal()` 原子替换全局注册表指针 →
  已存在的 LiveRegistry 下一轮 `GetSkill` 即读到新库，**无需重建 Runtime**；
- Agent 级技能落盘即生效（LiveRegistry 实时读盘）；
- `ReloadSkills` 不再失效 runtimeCache（快照消除后无需）。

`LiveRegistry.GetSkill` 对技能名做规范校验（小写字母/数字/连字符，≤64 字符），
阻断路径穿越；Agent 级解析失败按未命中回退全局库。

### 2.3 项目级技能的会话生命周期

```
session.create / 会话加载
  └─ 扫描 <ProjectDir>/.skills/ → 发现项目技能 → RPC skill.project.list
      → 前端批量确认（一次性载入）
       └─ 确认后：mindxses.Session 挂载 SkillOverlay（项目级 Registry 快照）
            ├─ 不持久化：每轮由 daemon 重建重挂载（会话级隔离）
            └─ 并入提示词目录（标注"项目技能"）+ 检索优先级最高
```

### 2.4 晋升管线（一切晋升由用户裁决）

`skill.promote` RPC（`skillstore.Promote`）：

```text
项目级(.skills) ──→ Agent 级(agents/<n>/skills) ──→ 全局库(~/.mindx/skills)
                    copyDir 复制目录                copyDir 复制目录
                                                     └─ 完成后自动 ReloadGlobal
                                                        （原子替换，即刻全会话生效）
```

目标为 Agent 级时无需重载（LiveRegistry 实时读盘）；晋升后源目录不变（复制语义）。

---

## 三、System Prompt 拼接过程

### 3.1 职责切分

- **mindx**（PromptBuilder，`internal/core/prompt_builder.go`）：组装全部应用语义段落，
  经 `WithBaseSystemPrompt` 注入 goharness；
- **goharness**：不生成、不追加任何文案段，输出即 base 的原样透传（单条 system 消息）；
- **Memory 注入**：保留在 goharness Hook 路径（动态内容随轮次变化，与静态拼装是两条路径）。

### 3.2 拼接流程图

```
mindx App.createRuntime
  └─ WithBaseSystemPrompt(PromptBuilder.Build)              # 接缝唯一入口
       │
goharness executor（每次 Ask 构建一次）
  └─ BuildSystemPrompts(sessionID, session)
       └─ base 输出原样透传（无任何追加）                    # 单条 system 消息
       │
goharness BeforeLLM Hook（当轮首轮注入）
  └─ memory_hook 追加「## 历史对话摘要」到 system 消息末尾    # 动态区
       │
  └─ AssembleMessages: [system] + [history 过滤孤立 tool_call] + [当前问题]
```

### 3.3 段落布局（静态在前、动态在后，保证 KV 缓存前缀稳定）

```text
╔══════════════════════════════════════════════════════════════╗
║  静态区（会话内逐字不变 → KV 缓存前缀稳定）                     ║
║ ┏━━━ mindx PromptBuilder.Build ━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓ ║
║ ┃ ① 身份段    IDENTITY.md 正文优先；无正文时兜底角色定义       ┃ ║
║ ┃ ② SOUL 段   SOUL.md 正文原样（标题由文件自带，无外部注入）   ┃ ║
║ ┃ ③ 能力段    Skill summary list + 执行前自检（动态生成）      ┃ ║
║ ┃ ④ AGENTS.md 全体 Agent 公共规则（内嵌常量，自带标题）        ┃ ║
║ ┃ ⑤ 环境/搜索策略  ProjectDir / 会话目录 / venv / 时间（动态） ┃ ║
║ ┃ ⑥ 扩展规则  权限规则 + Agent 发现引导 + 用户规则（动态渲染）  ┃ ║
║ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ ║
║  动态区（变化点与压缩窗口清空重合，无额外缓存损失）              ║
║ ┏━━━ memory_hook（BeforeLLM 首轮）━━━━━━━━━━━━━━━━━━━━━━━━━┓ ║
║ ┃ ⑦ ## 历史对话摘要  LongTerm 记忆检索 top-20                  ┃ ║
║ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ ║
╚══════════════════════════════════════════════════════════════╝
```

各段细节：

| 段 | 生成方 | 内容来源 | 空处理 |
| --- | --- | --- | --- |
| ① 身份 | mindx 渲染 / 文件正文 | IDENTITY.md 正文优先，否则 frontmatter 兜底 | Agent 必有 name/role，不空悬 |
| ② SOUL | 文件 | SOUL.md 正文 | 空则整段跳过 |
| ③ 能力 | mindx | Catalog（全局+Agent 级+项目覆盖） | 无技能整段跳过 |
| ④ AGENTS.md | mindx 常量 | `agentsCommonRules`（与应用版本绑定） | 常量非空恒在 |
| ⑤ 环境/搜索 | mindx | Session（ProjectDir 空时取 cwd） | 恒有 |
| ⑥ 扩展规则 | mindx（App.BuildRulesSection） | mindx.json 权限规则 + 固定引导 + rules.yml 用户规则 | 三者皆空才跳过 |
| ⑦ 记忆摘要 | goharness Hook | LongTerm 检索（确定性：压缩/显式写入才变化） | 无记忆不注入 |

### 3.4 效果示例

以预置 Agent `architect`（IDENTITY.md 无正文 → 兜底角色定义；SOUL.md 自带
「## 核心准则」；引用技能 research-pipeline / software-dev；用户规则含
「禁止强推主分支」、权限规则含 Always allow 运行 go build）为例，最终 system 消息：

```markdown
## 角色定义
 我叫 architect 是一名 软件架构师, 专职技术选型、架构设计、系统拆解、规范定义、
性能瓶颈治理、扩展性设计、技术风险把控与跨端统一方案。……

## 核心准则

1. 架构硬约束
   - 分层解耦：严格区分领域层、业务层、基础层、公共层、接入层……
2. ……

## 可用技能
以下专业技能是否能完成用户要求的任务。……
- pdf - 处理 PDF 文档的技能
- research-pipeline - ……
- software-dev - ……

### 执行前自检
在调用 Bash、Read 或 Grep 访问文件或目录内容之前，必须先执行此检查：
1. 角色门控 (P0)：此任务是否在我的职责范围内？……

## 行为准则

- **重要**：思考流与推理过程必须全部使用中文
- 结论先行，简短回答，像人类一样说话

### 角色门控 (P0)
（……全体 Agent 公共规则，随 mindx 版本绑定）

### 执行策略 / 知识诚实 / 回答对齐自检 / 可追溯决策 / 执行安全 / 兜底策略

## 沟通风格

冷启动时重建上下文。不使用表情符号。

## 配置环境

- **项目目录**: /Users/ray/workspaces/demo
 用户工作目录 — 文件在此永久保留，跨会话持续存在。……
- **会话目录**: ~/.mindx/sessions/2026xxxx-xxxx
  当前对话的临时工作区。……
- **会话ID**: 2026xxxx-xxxx-xxxx
- **本地时间**: 2026-09-06

## 搜索策略

1. 对于本地搜索，优先使用 Grep……

## 扩展规则

- Always allow 运行 go build
- Agent 发现：当需要查找或列出可用 Agent 时，运行 'mindx agent list'……
- 禁止强推主分支

## 历史对话摘要

以下为与用户过往对话的决策记录，作为延续上下文使用；

- [2026-09-05 14:30] [技术选型] - 选定 govector 作为向量存储……
```

### 3.5 KV 缓存稳定性要点

- ①-⑥ 在会话内逐字不变（动态段如时间只精确到日期，跨天才会变化）；
- ⑦ 的摘要文本具有确定性：记忆条目只在压缩（`persistCompactionChunks`）与显式
  `memory.add` 时写入，`RetrieveLatest` 按 Timestamp 倒序（唯一时间戳 → 排序确定），
  `FormatMemoryRecords` 只含绝对时间戳——记忆库不变则文本逐字节相同；
- 摘要变化仅发生在压缩时（新摘要条目写入 + 窗口清空），缓存失效点与窗口清空重合，
  无额外损失。
