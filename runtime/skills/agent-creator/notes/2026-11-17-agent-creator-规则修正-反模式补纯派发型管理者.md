---
version: "1.3.1"
pub_date: 2026-11-17
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 反模式补纯派发型管理者（glm）

- 问题：反模式清单缺一类高发空心形态——职责只有「拆解→派发→收报告」、无任何自己动手动作的管理岗编制；对齐门与 leader 硬性要求从装配侧兜底，评审面缺一条显式否决依据。
- 修正：注意事项与反模式新增「纯派发型管理者」——管理岗必须保留可核验的自己动手动作（估算定档、抽查验证成员产出、亲自验收关键交付），缺手艺支撑禁止注册。version 1.3.0 → 1.3.1。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿（162 行，soft_rules 0）。
