---
version: "0.5.0"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 Team Profile 内容规格与任务闸门（glm）

- 问题：①Profile 写法只说「协作规则、验收口径、汇报链路」，未规定核心内容形态——团队承接的业务协作流程（阶段化：每阶段责任角色→产出→作为下一阶段的依据）无人要求写，团队定位与成员职能的重述却无人禁止；②任务闸门缺失——没有「任务必先到负责人、全员出动须计划经用户放行」的升级路径，任何任务都可能演变成全编制总动员，token 成倍燃烧且无人对结果收口。
- 修正：①编制规范第 6 条重写 Profile 内容规格——正文核心是分阶段协作流程，禁止重述团队定位（description 已承载）与成员职能（prompt_builder 成员名单区自动注入，重述即双份噪声）；②新增第 11 条任务闸门——负责人主理、成员按需协助、全员出动须具体执行计划经用户放行；③蓝图「团队职责」节与反模式同步（新增「Profile 重述成员职能」）。version 0.4.0 → 0.5.0。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（130 行，soft_rules 0）；成员职能自动注入已对照 internal/core/prompt_builder.go buildLeaderSection 成员名单区实证。
