---
version: "0.5.0"
pub_date: 2026-11-17
sha: "4452fd7c1da3cc0f35b74eb201122090cd6f0ecdbd0264b63bf0f3b7b25078c5"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 价值链闭环与全量梳理（glm）

- 问题：用户要求按 skill-creator 系统化全量梳理。审计发现三硬缺陷：①价值链无闭环——账本状态有「已完成」但无任何步骤交代谁按成功判据关账，验收环节缺失；②v0.4.0 任务行改「执行：」后 ledger.py TASK_RE 只认「cron：」导致执行方式行整行解析失败（预算/验收/负责人全丢）；③版本漂移——SKILL.md frontmatter 0.4.0 而履历已 0.4.1。
- 修正：价值链扩为五步（+验收关账）并新增第五步节——逐条核对成功判据拿证据、老板确认后才能写「已完成」、未通过回第四步；账本状态规则补「已完成须经第五步、暂缓/取消须注明原因」；每日例行补受阻处置路由（记阻塞点与解锁路径）与达标目标指向第五步；description 与何时使用补验收关账触发；反模式补「未验收自行关账」「受阻装没看见」；ledger.py 正则兼容「执行：」与「cron：」两种写法（夹具实测两格式 problems 空）。version 0.4.x → 0.5.0。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿（119 行，soft_rules 0）；ledger.py 夹具含两种任务行格式实测通过。
