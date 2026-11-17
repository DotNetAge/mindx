---
version: "0.7.1"
pub_date: 2026-11-17
sha: "5c8d47abaef9c5b430e67330c813d7afb8fb6ec9b0d383c02d8d614fa02b3ba6"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 六类判据全量噪声清扫（glm）

- 问题：用户要求删除全部输入性噪声、严格按 skill-creator 执行——六类判据逐文件审计五文件，命中 18 处：intro 辩论口气（「这是计划能力不是执行能力」）、说教句（「澄清不是确认仪式」「返工成本高于提问」「排日历没有意义」）、叙述性过渡（「分解过程本身是再消解」「这就是为什么」「列名本身就是计划」）、跨节同义重复（「Agent 不是人」两处、「执行进展由执行方回写」两处、标题括注「本技能终点」）、零信息括注（「示例仅为帮助对号」）、步骤残留（任务书标题仍写「第四步派工日志」旧结构）。
- 修正：18 处全部压为纯指令或删除；ledger.py STATUS 与 summary counts 同步 v0.7.0 计划态三态（待批准/已批准/已驳回）——状态枚举不一致属自包含缺陷一并修。version 0.7.0 → 0.7.1。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿；ledger.py 夹具实测三态合法/非法状态判定与 summary 计数。
