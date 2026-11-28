---
version: "1.14.1"
pub_date: 2026-11-28
sha: "6dd75709d7611c926ad38c772615f9443f6cbc1106f46bb79f553c3a7d8e83f0"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-28 查重只看本地，市场已有对口包时会重复创建（default）

- 问题：规则缺失——步骤 2 查重与纪律条目只有 `mindx skill list`（本地库），市场侧没有任何检索。市场已有 `contract-review` 这类现成技能而本地未装时，会判成"本地没有 → 新建"，产出与市场重复的包。
- 否决：把 `mindx skill list` 直接换成市场语义检索。本地查重是必要的一层（能进化既有技能就不新建），两层并存而非互斥。
- 修正：步骤 2 改为两层查重——本地 `mindx skill list` 查同类，本地没有再按 `finder` 技能执行 `mindx market search --kind skill --k 5 "<技能能力整句>" --json`，市场有对口包就改为安装；纪律条目「禁止重复造轮子」同步。version 1.14.0 → 1.14.1。
- 验证：`mindx skill validate --path runtime/skills/skill-creator` 全绿（267 行、0 issues）。
