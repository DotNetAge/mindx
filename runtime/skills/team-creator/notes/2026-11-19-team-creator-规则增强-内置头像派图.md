---
version: "0.8.3"
pub_date: 2026-11-19
sha: "26e83d08b1b56ff33f59d3e0ed64723ed41aee07c660d3c99943628d1336ace4"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-19 建好的团队没有头像，团队卡片只能显示首字占位（mainagent）

问题：流程漏洞——建制流程止于 hire，团队自身 icon 空缺；成员头像由 agent-creator 覆盖，团队那张没有任何分配动作。
否决：复用成员里负责人的头像——团队与成员是两个实体，共用一张会让人误以为点开的是个人；也让头像占用判定失效。
修正：步骤 5 新增第 7 条「团队头像」，调用 `~/.mindx/assets/bin/pick-avatar.py --group team --target ~/.mindx/teams/<team>/TEAM.md`，与成员头像同一套指纹占用判定。version 0.8.2 → 0.8.3。
验证：`--group team --dry-run` 出计划；写回后 TEAM.md 的 icon 为单行 data URI；`mindx skill validate --path runtime/skills/team-creator` 无 issues。