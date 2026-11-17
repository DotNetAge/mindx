---
version: "0.6.2"
pub_date: 2026-11-18
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 description 去路由指代与招募要点去重述（glm）

- 问题：skill-creator 全量检视发现两处机械缺陷——①description 末句「只招单个 Agent 走 agent-creator」指代其他实体，违反 description 军规（概览三问、禁路由话术，LLM 靠对比多条干净概览自然路由）；②team-design.md 招募要点第三次全文重述 leader 硬性要求（蓝图「负责人选定」节与建制前核对已两处承载），命中同义重复判据。
- 修正：①description 末句改「只招单个 Agent 时不适用」，去掉实体指代；②招募要点改纯指针「须过蓝图「负责人选定」节的 leader 硬性要求，缺一不建队」。version 0.6.1 → 0.6.2。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（130 行，soft_rules 0）。
- 遗留（结构性，待用户拍板）：三处正文与 references 的近全文重复——SKILL.md 注意事项 leader SOUL 条 vs 编制规范第 5 条；蓝图「与现成团队对比」节 vs 步骤 5 第 1 条检索指令；步骤 2 MECE 检验 vs 编制规范第 1 条。
