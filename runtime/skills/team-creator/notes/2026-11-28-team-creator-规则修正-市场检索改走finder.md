---
version: "0.9.2"
pub_date: 2026-11-28
sha: "9ddd7255066e6e8617513634510b3f5f80c8f7f7752fdbb579828c5c66178904"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-28 团队与岗位的市场检索仍是关键词 list（default）

- 问题：版本兼容——步骤 1 整队检索与步骤 4 逐岗位检索用 `mindx market list --kind team/agent --q "<关键词>"`，服务端 q 只匹配 name/description 两项且中文关键词实测 0 命中（「合同」「宠物」均无结果）；市场早已提供 `mindx market search` 语义检索，技能未跟进。
- 否决：在技能内各写一套检索命令。三份 creator 技能各写一套是漂移的根源，改由 `finder` 技能单点定义阈值与裁决口径。
- 修正：步骤 1 整队检索改 `mindx market search --kind team --k 5 "<整句全流程需求>" --json`（≥0.65 且描述对口才算命中），query 口径从「全流程词」改为整句需求；步骤 4 逐岗位改 `--kind agent`，命中并确认后 `market install agent` + `agent hire`；反模式条目同步措辞。version 0.9.1 → 0.9.2。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿（117 行、0 issues）。
