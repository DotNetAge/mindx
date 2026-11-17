---
version: "0.3.1"
pub_date: 2026-11-17
sha: "317ff2116502e3efaa152a4282e96177691eca9f4aacb97060f8447616ad0faa"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 估算章法指针改指 goal-planner（glm）

- 问题：dispatch-gate 技能被用户否决删除（负责人基础能力应共用收编 goal-planner），本技能注意事项与 team-design.md 第 5 条的 dispatch-gate 指针成断链。
- 修正：两处改为「章法见 goal-planner 技能的『派工前估算定档』，共用能力」，leader SOUL 范式表述不变。version 0.3.0 → 0.3.1。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿。
