---
version: "1.2.0"
pub_date: 2026-11-17
sha: "5c0a3d8b85c1e324a938434753023127d703186b8e09d9623971352a7bbe89c5"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 团队职责切出、噪声清理、更名招募智能体（glm）

- 问题：①双路径建团——本技能「团队招募」节与 team-creator 全流程是两套团队组建入口，编制规范/Profile 写法/成员招募两边都管，触发时靠 LLM 猜；②正文多处噪声：「智能体定义速览」整节纯重复、「重要说明」3 条全为说教或重复、注意事项 4 条与正文重复、假设选项示例 11 行过长、步骤 3 的 leader 岗要求出现在单 Agent 工作流上下文错位、L14 决策叙事违规（「deep-review E 路，用户已裁决」）；③description 功能介绍体、无触发词、不提边界；④name_zh「智能体创建」是英文名直译，用户否决（直译不是人话，中文名须是地道能力名）。
- 修正：①删除「团队招募」节与 team-design.md 引用（文件迁至 team-creator/references），先判定表所有团队判定路由 team-creator，步骤 1 删团队包检索——单一团队组建入口；②噪声清理按审计清单逐项落实，leader 岗硬性要求（1 门管理技能 + 1 门领域手艺技能）移至 team-creator 蓝图「负责人选定」节；③description 重写为触发场景+触发词+边界；④name_zh 改「招募智能体」，H1 同步。version 1.1.0 → 1.2.0。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿；team-design.md 迁移后三处 references 互链（agent-schema/agent-writing/persona-engineering）改为跨技能文字指针，无断链。
