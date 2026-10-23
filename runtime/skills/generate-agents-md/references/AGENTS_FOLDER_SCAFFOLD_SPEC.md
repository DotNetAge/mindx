---
updated: 2026-10-10
---
# .agents 脚手架目录社区规约
> 配套AAIF AGENTS.md规范，定义`.agents/`配套目录的约定，目录**不会自动加载生效**，必须由AGENTS.md通过`@`引用。

## 目录清单
### ✅ 按需目录（有内容才创建，禁止空文件夹）
- `.agents/rules/`：存放可复用规则片段，拆分成独立md文件，供AGENTS.md使用`@`引用。
- `.agents/skills/`：存放Agent技能，每个技能独立子目录，内部包含SKILL.md与技能的references参考文件。
- `.agents/subagents/`：子Agent定义文件，多角色场景才使用。
- `.agents/assets/`：辅助资源，架构图、环境说明等非规则类资源。

### ❌ 禁止自动创建
不要凭空创建上面目录，如果没有对应的规则片段/技能，不生成文件夹。

## 文件约定
1. 片段文件命名：语义化命名，如 `security-rules.md`、`test-commands.md`。
2. 引用语法：`@.agents/rules/security-rules.md`，放在AGENTS.md内。
3. 不允许把可执行脚本散放在`.agents/`顶层或 rules/ 等资源目录，如需脚本放到仓库其他标准位置；技能包内部（`.agents/skills/<name>/scripts/`）不受此限。
4. .gitignore提示：可选择把`.agents/*.local.md`加入gitignore，存放本地个人临时规则。

## mindx 生态扩展目录（非 AAIF 社区约定，mindx 环境专用）
运行环境为 mindx（`~/.mindx` 存在）时，`.agents/` 下另有三个按需治理目录，同样**有内容才创建**：
- `.agents/notes/`：随手记，执行过程中的决策/踩坑/经验/待办等原始素材；
- `.agents/memory/`：群体经验池，notes 经复盘聚类后的长期沉淀，内含 `.archive/` 归档；
- `.agents/references/`：外部权威资料原文，一个条目一个 `ref_XXX/` 目录（内含 meta.json 与原文），全局引用索引 `references/index.csv`。

项目级 AGENTS.md 不复制治理条款（由用户全局 AGENTS.md 兜底），只写项目独有约束。

## 边界说明
- `.agents`本身**不是自动生效目录**，仅仅是存放资源的容器；
- 仅AGENTS.md/AGENTS.override.md具备目录scope生效能力；
- 脚手架只是工程组织方案，不属于AAIF强制核心规范，属于社区配套最佳实践。
