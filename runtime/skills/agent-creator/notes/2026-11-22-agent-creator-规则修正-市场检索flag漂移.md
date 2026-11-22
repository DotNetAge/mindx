---
version: "1.4.2"
pub_date: 2026-11-22
sha: "513d7379bc24c512e28aa59a1f68e60cfb5250e5a9ca516a737b44428f6d5818"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-22 市场复用检索命令报 unknown flag（assistant）

- 问题：规则缺失——步骤 1 与检查清单写 `mindx market list --kind agent --filter "<关键词>"`，但 CLI 只注册 `--q`；`--filter` 属于 `mindx skill list`。执行即 `Error: unknown flag: --filter`，模型只能换写法重试，复用检索多烧一轮；若放弃过滤退回裸 list，则带回默认 50 条按 newest 排序的条目。
- 否决：只改技能文档、不加 CLI 别名——存量安装的技能包仍带旧写法，升级前必然报错，故两条都做。
- 修正：`SKILL.md` 步骤 1 与 `references/agent-schema.md` 检查清单统一改为 `--q`；`agent-schema.md` 的 updated 改为 2026-11-22；SKILL.md version 1.4.1 → 1.4.2。CLI 侧给 `market list` 增加 `--filter` 隐藏弃用别名，等价 `--q`。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿；`mindx market list --kind agent --q "测试" --json` 正常返回；`--filter` 打印弃用提示且结果一致。

