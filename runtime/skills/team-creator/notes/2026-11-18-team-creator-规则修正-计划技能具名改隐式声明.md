---
version: "0.6.1"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 计划技能具名改隐式声明（glm）

- 问题：0.4.0/0.6.0 在负责人选型、建制前核对、蓝图团队职责节四处正文具名 goal-planner，0.3.1 引入的两处「章法见 goal-planner 技能…」指针也是存量具名——用户随时可换用不同名称但能力等价的计划技能，具名声明与「等价可替换」自相矛盾，且违反「正文只写能力维度、技能名只进 frontmatter 清单」的对号纪律。
- 修正：全部改为能力维度隐式声明——「计划规划类技能（目标拆解、估算定档、选岗派工）」承载硬性要求与闸门表述，章法指针改「按其装配的计划技能执行」；grep 确认 SKILL.md 与 team-design.md 正文零具名残留。version 0.6.0 → 0.6.1。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（130 行，soft_rules 0）；`grep "goal-planner"` 在本技能 SKILL.md 与 references 正文零命中（仅 notes 履历留存）。
