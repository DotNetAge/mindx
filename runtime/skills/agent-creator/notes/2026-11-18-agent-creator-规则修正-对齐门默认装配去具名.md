---
version: "1.3.2"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 对齐门默认装配去具名（glm）

- 问题：1.3.0 在职责-能力对齐门正文具名 goal-planner 作为管理者默认装配——用户可随时换用不同名称但能力等价的计划技能，具名与「等价可替换」自相矛盾，违反「正文只写能力维度」的对号纪律。
- 修正：改为能力维度声明——「计划规划类技能（目标拆解、估算定档、选岗派工）」承载默认装配表述；具体选哪个技能属装配时的对号决策，由 frontmatter 技能清单与用户裁决。version 1.3.1 → 1.3.2。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿（162 行，soft_rules 0）；SKILL.md 正文 `grep "goal-planner"` 零命中（仅 notes 履历留存）。
