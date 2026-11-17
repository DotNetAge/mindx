---
version: "0.1.1"
pub_date: 2026-11-17
sha: "7c216b0ae8e8063cd4f66429e3f4d4c4d141408d96e5772953a2d92aa5e097c9"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 中文名去职位化（glm）

- 问题：name_zh「目标规划师」是职位名，技能中文名须是地道能力名（用户定名规矩：禁职位后缀与直译）。
- 修正：name_zh 改「目标规划」、name_zh-tw 改「目標規劃」，H1 同步。version 0.1.0 → 0.1.1。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿。
