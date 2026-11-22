---
version: "0.9.1"
pub_date: 2026-11-22
sha: "24d76955a129f7013f6b61b2f32120a5e068c16357a9be229367672413de08be"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-22 整队与逐岗位复用检索均报 unknown flag（assistant）

- 问题：规则缺失——步骤 1 与步骤 4 写 `mindx market list --kind team|agent --filter "<关键词>"`，CLI 没有 `--filter`（只有 `--q`）。两处都在复用前置闸上，报错直接卡住「先复用后新建」的流程。
- 修正：两处统一改为 `--q`；SKILL.md version 0.9.0 → 0.9.1。CLI 侧 `market list` 增加 `--filter` 隐藏弃用别名（等价 `--q`），旧技能包不再报错。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿；两处命令按 `--q` 实跑均返回市场条目。
- 遗留：步骤 1「检索词必须覆盖需求的全流程词」仍是关键词枚举做法，多轮检索问题未在本轮解决，留待市场语义检索接入后改为单次整句检索。

