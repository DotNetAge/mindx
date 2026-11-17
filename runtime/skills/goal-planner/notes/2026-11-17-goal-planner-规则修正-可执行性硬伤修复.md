---
version: "0.7.2"
pub_date: 2026-11-17
sha: "4a87c8bd0f0c1b2b561f8fbac8343aa262c4de5d029571d9d485108425f4b1cf"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 四处硬伤导致首次实跑必卡（workbuddy）

- 问题：指令冲突——审查发现四处会在首次执行时卡住：①「Task 工具」不存在（运行期注册的是 TaskCreate/TaskGet/TaskList/TaskUpdate 待办清单与 SubAgent，`grep -rn '"Task"' --include=*.go core mindx` 零命中），0 档「本 Agent 自己做」却被路由到一个做不了事的清单工具；②档位缺「C3 且探测点 ≤3」一行，「只有目标没有路径」的小任务无处可归；③档位轮数上限低于自带基准表算出的中位（0 档「≤5 轮」vs 3 点中位 9 轮、1 档「≤15 轮」vs 8 点中位 19 轮），闸门必然每轮误报；④账本状态与磁盘实例冲突（技能只认三态，`.agents/plans/goal-ledger.md` 头部仍是四态并带秘书/老板称谓，实测「进行中」被判非法）。同类另含规则缺失（档位表缺 C3 小任务）与流程漏洞（「一次 AskUser 提交完整问清单」物理不可行：AskUser 单次只带一问）。
- 否决：只把上限数字改大、不动定档表结构——0 档与 1 档各自独立写门槛，改一处必漏另一处，故改为置信度 × 探测点数查表，第四步只做映射不重算。另否决给账本任务行加 tokens 字段：`TASK_RE` 是行级正则，字段一加整行静默解析失败，体积只进任务书。
- 修正：第三步改查表（补 C3 两行，预算按基准表中位到最坏取区间 6-12 / 8-15 / 12-25 / 15-30）；第四步去门槛复述、工具名改正（自己做 / SubAgent / Cron，注明 TaskCreate 只建清单不代表干活）；第五步加计划落盘 `.agents/plans/G-<编号>.md` 与 mermaid 阈值（≥3 任务或有并行组才画）；账本模板补收益方与场景、澄清、计划文件、批准留痕四行，划清「可变状态字段」与「只追加的进展」；摘掉两处 deep-review 指名（复位 0.5.5 结论）；evals 按纯计划职责重写；requirements-clarification 改为单条消息内并发 AskUser；estimation-method 补 B = Σ探测点 + 汇总 + 返工余量的算法表、收尾同时核对轮数与 tokens、§六 精确值取整并标注未逐条复核；ledger.py 负责人正则不再吞掉「（执行：x）」标记，独立为 exec 字段。version 0.7.1 → 0.7.2。
- 验证：`mindx skill validate --path mindx/runtime/skills/goal-planner` 全绿（119 行、soft_rules 0）；`ledger.py` 对四种任务行变体（含执行标记 / 无标记 / 无预算 / 无验收）跑 tasks、validate、summary 三模式，负责人与执行方式分列正确、缺预算与缺验收均被捕获；账本头部改三态后 summary 计数正常。
