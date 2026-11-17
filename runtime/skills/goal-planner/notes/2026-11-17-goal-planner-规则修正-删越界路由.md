---
version: "0.5.5"
pub_date: 2026-11-17
sha: "e1ca4c6a400efe7522c189b0fee84ffd1c87e5a45a616cf7e9c5b34cc50eaffb"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 删除对 daily-report 的越界路由（glm）

- 问题：用户指出「用户要日报走 daily-report 技能」是在替执行 Agent 写死同伴技能的路由——技能路由归运行时注册表与执行 Agent 按触发词自行判断，同伴技能不该指名道姓。且这是耦合隐患实证复现：本技能曾引用 scheduler，scheduler 一删引用即断；team-planner 改名亦需联动修改。
- 修正：删除 SKILL.md 中整段 daily-report 边界声明（L116）；进度简报的数据源（账本核对）已由例行清单第 4 步隐含，无需另行声明。version 0.5.4 → 0.5.5。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿；正文与 references 中 daily-report 零命中（notes 履历除外）。
