# Agent 定义 Schema 与字段规范

> Agent 定义的结构事实：文件布局、frontmatter 字段、工具控制、样式规则。
> 编写方法论（description 怎么写、SOUL 怎么写）见 [agent-writing.md](agent-writing.md)；
> 团队实体见 [team-design.md](team-design.md)。

## 文件结构

智能体以**目录格式**存储（`agents/<name>/`），由两个文件组成：

```text
agents/<name>/
  IDENTITY.md   # 唯一 meta 入口：frontmatter（强类型一级字段）+ 可选身份正文
  SOUL.md       # 角色职责与负责范围，纯正文
  skills/       # Agent 级技能库（可选，由技能系统装载）
```

- **IDENTITY.md** 是身份卡：frontmatter 承载全部路由元数据；正文惯例留空。
- **SOUL.md** 是角色职责：Agent 的核心工作指令，自带标题结构，系统原样注入提示词。

## IDENTITY.md frontmatter 字段

| 字段            | 格式                     | 规则                                                                 |
| --------------- | ------------------------ | -------------------------------------------------------------------- |
| `name`          | 小写连字符               | 唯一，基于名词，体现角色（如 `code-reviewer`）                       |
| `role`          | 中文头衔                 | 人类可读的角色标题（如"代码审查员"）                                 |
| `description`   | 1-2 句中文               | **概览**（负责什么/能干什么/什么时候用），供 LLM 并列对比选择；写法见 [agent-writing.md](agent-writing.md) |
| `category`      | 中文业务分类             | 分组展示用（如"产品研发"、"办公提效"、"市场营销"）                   |
| `icon`          | 可选                     | 图标标识                                                             |
| `hired`         | 布尔，缺省 false         | 雇佣标记；新建智能体默认未雇佣，`mindx agent hire` 后才可用于会话    |
| `skills`        | 来自 `skill list` 的列表 | 仅领域相关技能 —— 每个技能都会增加上下文开销                         |
| `exclude_tools` | 列表                     | 内置工具裁剪清单；内置工具默认全量可用，按需排除，不支持通配符       |
| `allows_tools`  | 列表                     | MCP 服务白名单，条目格式 `mcp:<server>`；空清单 = 不注入任何 MCP 工具 |
| `include_tools` | 列表                     | 唤回默认不在场的内置工具（如 `TeamCreate` 等运行期组队工具）         |
| `introduction`  | 可选                     | 人设正文备用入口；IDENTITY.md 正文非空时覆盖此字段                   |
| `meta`          | 映射                     | 自由扩展杂项                                                         |

已废弃的旧字段（新定义中**不得出现**）：`allowed-tools`（允许列表）、`requires.bins` / `requires.env`、`meta.name_zh` / `meta.role_zh` / `meta.description_zh`、旧 `domains`（已迁移为 `category`）。技能（SKILL.md）的 frontmatter 与 Agent 定义对齐：同样不得出现 `allowed-tools`、`requires.*`、`meta.*_zh`。

## IDENTITY.md 正文

**惯例留空。** 正文为空时，系统由 frontmatter 的 `name` / `role` / `description` 自动生成标准角色定义（"我叫 X 是一名 Y, Z"）。

正文非空时，将**整体覆盖**自动生成的角色定义，作为整个 Agent 的人设（标题由文件自身处理）。仅在前述字段无法表达人设时才使用正文，并在正文中自带标题结构。

## 工具控制

内置工具默认全量可用，控制手段只有三个：

- **`exclude_tools`（内置工具裁剪）**：按角色排除确实有害的工具。工作者排除 `SubAgent`、`CollectResults`（不委派）；常见追加 `Sleep`、`PowerShell`。不支持通配符，只能指定具体工具名。`Team*` 运行期组队工具默认不在场，无需排除。
- **`include_tools`（唤回默认不在场的工具）**：`Team*` 运行期组队工具（`TeamCreate`/`TeamDelete`/`TeamList`/`TeamGetTasks`）默认剥离，需要运行期组队时在此列出唤回。
- **`allows_tools`（MCP 服务白名单）**：条目格式 `mcp:<server>`（server 粒度），决定注入哪些 MCP server 的工具；空清单 = 不注入任何 MCP 工具。内置工具不在此列。

工作者智能体保留 `SubAgent` 后可能开始委派工作而不是自己做 —— 只给明确需要协调他人的角色保留委派工具。

## 样式规则

- 使用直接、简短、祈使句式的语言。
- 优先使用绝对性用语：`每个`、`所有`、`总是`、`从不`、`禁止`、`不得`。
- SOUL 每条规则都要具体可执行，含明确边界。
- `role` / `description` 直接用中文，不设独立中文字段；智能体间命名保持一致。

## 创建检查清单

运行 `mindx agent add` 之前：

- [ ] 已与用户确认名称、角色、业务分类、工作范围
- [ ] 已运行 `mindx agent list --all --json` —— 确认无重复或重叠的 Agent
- [ ] 已运行 `mindx market list --kind agent --filter "<关键词>"` —— 确认市场无现成可复用的 Agent 包
- [ ] 已运行 `mindx skill list --json` —— 已识别相关技能；本地缺失的技能已按 `find-skills` 技能查找并安装
- [ ] `name` 为小写连字符格式，唯一，基于名词
- [ ] `role` 为简洁中文头衔；`description` 为 1-2 句中文概览（负责什么/能干什么/什么时候用）
- [ ] `category` 已确定为中文业务分类
- [ ] `skills` 列表精简 —— 仅包含领域相关技能
- [ ] `exclude_tools` 按角色裁剪；需要 MCP 服务时已列 `allows_tools`

创建之后：

- [ ] 已编写 SOUL.md（职责、方式、边界三段式，显式引用技能）
- [ ] description 是干净概览，无路由指派与指代其他实体的文本；并排对比与相邻 Agent 不混淆
- [ ] 规则条目含触发场景 + 具体行为 + 理据；单段规则 ≤8 条，工作流 ≤5 步
- [ ] 价值取向有冲突可能的角色已写价值观排序；面向用户的角色已写退化行为
- [ ] 边界段含职责边界与诚实边界两层
- [ ] IDENTITY.md 正文留空（除非确需自定义人设）
- [ ] 需要固定团队时：建 Team 实体（`mindx team add`）并雇佣（`mindx team hire`），团队职责写 Team Profile——不在 Agent 定义里写团队信息
- [ ] 已执行 `mindx agent hire <name>` 并通过 `mindx agent list --json` 验证
- [ ] 已做概览选择验证（见 SKILL.md 工作流"概览选择验证"步）
