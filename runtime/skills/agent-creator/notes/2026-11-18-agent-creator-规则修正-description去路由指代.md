---
version: "1.3.3"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 description 去路由指代（glm）

- 问题：skill-creator 检视 team-creator 时的横向发现——本技能 description 末句「成建制团队走 team-creator」指代其他实体，与 team-creator 侧的对称话术互为镜像，同违反 description 军规（概览三问、禁路由话术）。
- 修正：末句改「需要成建制多成员协作时不适用」，去掉实体指代，路由由两条干净概览的对比自然涌现。version 1.3.2 → 1.3.3。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿（162 行，soft_rules 0）。
