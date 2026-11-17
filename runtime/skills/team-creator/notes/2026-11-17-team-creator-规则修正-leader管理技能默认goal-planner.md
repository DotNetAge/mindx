---
version: "0.4.0"
pub_date: 2026-11-17
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 leader 管理技能默认 goal-planner（glm）

- 问题：leader 硬性要求「1 门管理技能（任务拆解/派工/验收类）」未指名默认选项——建制时靠执行者自行挑选，易漏装或装上与拆解派工不沾边的技能，空心领导判定形同虚设。
- 修正：「负责人选定」与建制前核对两处指名默认装配 goal-planner 目标规划（覆盖拆解、估算定档、选岗派工），等价技能可替换；验收取证仍由领域手艺技能承担——goal-planner 计划获批即终点、不做执行，不越界替代抽查验证。version 0.3.1 → 0.4.0。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（129 行，soft_rules 0）。
