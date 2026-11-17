---
version: "0.1.2"
pub_date: 2026-11-17
sha: "940bf13282fb8bf57c9f2af941303e0085a70b0ca7a89f6711f7f1c551a71d3a"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 移除 scheduler 依赖指针（glm）

- 问题：用户判定 scheduler 技能无独立存在必要，已删除。本技能两处仍引用 scheduler（四件套出处、排程章法承载方），成断链。
- 修正：四件套定义就地自足（删「与 scheduler 技能的派工日志规范一致」括注）；「排程章法由 scheduler 技能承载」改为「排程用 Cron 工具承载」。version 0.1.1 → 0.1.2。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿。
