---
version: "0.8.0"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 Profile 受众自检——调度链写给不在场的人（glm）

- 问题：用户指出市场团队 Profile 的「调度链」段（发起方只对接负责人、不越过负责人直管成员）是重大噪声——Profile 只注入负责人 SYSTEM_PROMPT（prompt_builder.go L139 注释实证：成员侧不注入，与团队对话=与负责人对话），约束发起方的规则写给负责人看是废话，且诱导「门卫/转发器」自我定位，恰好助推「纯派发型管理者」反模式——写了比没写更差。
- 修正：①编制规范新增第 12 条「Profile 受众自检」——每条规则先问约束谁：只写约束负责人自己的（主理、定档、抽查验收、汇报升级）；约束发起方的靠架构保证不写；约束成员的写进成员定义。②反模式新增「调度链写给不在场的人」。version 0.7.0 → 0.8.0。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（132 行，soft_rules 0）；受众事实以 internal/core/prompt_builder.go buildLeaderSection 实证为准。
