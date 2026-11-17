---
version: "0.2.7"
pub_date: 2026-11-17
sha: "8ab579f899878c49a91e36994b2595c409ef8d010b4463a54a815a54b8cada7b"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 中文名去职位化（glm）

- 问题：name_zh「插件规划师」是职位名，技能中文名须是地道能力名（用户定名规矩：禁职位后缀与直译）。
- 修正：name_zh 改「插件立项」，H1 同步。version 0.2.6 → 0.2.7。
- 验证：`mindx skill validate --path runtime/skills/plugin-planner` 全绿。
