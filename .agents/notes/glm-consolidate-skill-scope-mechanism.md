# 技能位置机制与 skill-creator 统一定稿

日期：2026-10-05
背景：skill-creator 原工作流用 `mindx skill add` 安装（写全局库），与 runtime/AGENTS.md 军规的技能回观修正流程（fetch 到项目级）存在两套语义。用户定稿统一机制。

## 机制定稿（用户拍板）

- 三级作用域：项目 `<ProjectDir>/.agents/skills` → 智能体 `~/.mindx/agents/<name>/skills` → 全局 `~/.mindx/skills`，载入就近优先（同名覆盖）
- skill-creator 创建的技能一律落项目级；禁止智能体直接读写全局库；安装到全局完全交给用户走界面，智能体不执行任何全局安装命令（含 `mindx skill add`）
- 进化已有技能走 `mindx skills fetch`（仅全局→项目，同名默认拒绝，--force 覆盖时 notes/ 自动留档 notes.prev-<unix>/，项目级实时读盘无需 reload）
- 项目级技能创建后不用 `mindx skill validate` 验证（它只支持全局库），用 SKILL 内自检清单替代；用户拒绝为此补 CLI
- runtime/AGENTS.md 军规精简：触发条件 + 指向 skill-creator「技能位置与进化」流程 + 一句兜底（技能缺失时直接改项目级副本，禁写全局库）。安全底线保留在军规里，因为它无条件生效，不能完全寄生在按需加载的 SKILL.md

## CLI 事实（已核实）

- `mindx skills fetch/discovery` 在 cmd/skills.go；`mindx skill add/get/list/validate` 在 cmd/skill.go 且全部只操作全局库 skillsDir()
- fetch 的 notes 保护逻辑在 internal/core/skillstore/fetch.go（Fetch 方法注释完整）
- parse.go 第 9 行注释原写 `.skills` 是过时路径，已修正为 `.agents/skills`

## 改动落点

- runtime/skills/skill-creator/SKILL.md v1.3.0：新增「技能位置与进化」章节（何时使用之后）；步骤 1 补 discovery；步骤 3 落位项目级；步骤 6/7 从"安装阶段"改为"落位与验证阶段"（discovery + 自检清单）；审查模式步骤 A/B/D 全部对齐项目级语义
- runtime/AGENTS.md 第 20 行军规精简
- internal/core/skillstore/parse.go 注释修正

## 教训

1. 移植技能的工作流必须与宿主平台的位置机制核对——原版（Anthropic skill-creator）没有作用域概念，直接照搬会把"安装"写成写全局库
2. 军规（无条件生效）与技能（按需加载）分层：操作细节进技能，安全底线留军规
3. 【用户否定纠正】SKILL.md 篇幅两次膨胀被我压回：用户只要求写入"钉死项目级 + fetch 进化 + 全局安装归用户"三条决策约束；三级作用域表、载入顺序是用户告知的背景（明确说过不写入），fetch 实现细节（原子复制、notes 留档）是 CLI --help 可见的，都不该进 SKILL。写技能前先把"用户要写的"和"用户告知的"分开列清单
4. SKILL.md 内部重复也要避免：步骤 7 自检清单与审查模式步骤 B 几乎相同，正确做法是指向既有清单而非复制
5. 【用户共识】notes/ 是技能的进化履历（fetch Overwrite 时 notes 保护迁入 notes.prev-*，代码佐证履历语义）。进化记录编写规范落在 references/notes-convention.md：模板字段（问题/否决/修正/验证）从消费者视角倒推——失败尝试与用户约束原话是最高价值内容；履历只追加不改写
