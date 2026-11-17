---
version: "0.3.0"
pub_date: 2026-11-17
sha: "9cfec94e21d5530f9ac4337b338f679f3ca1eaa901035586b2a87cb94810de75"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 价值重排与三层重建（glm）

- 问题：skill-creator 审查判定不合格——①单层（无 references/ 无 scripts/）；②除账本条目外全套模板缺失（任务书/意图卡/摸底计划/简报）；③确定性操作（账本解析/进度汇总/无预算核对）无脚本全靠裸算；④用户点名的真正价值（搞清需求/写计划/量化工作/Token 式分配）各只有几行雏形，无系统化方法论；⑤价值重心错位——60% 正文在账本管理，价值链只占 25%，记账技能假装成规划技能。
- 修正：SKILL.md 重写为价值链主干（搞清需求→写计划→量化估算→分配排程，账本降为支撑工件并加 ledger.py 脚本路由）；references/ 新增 requirements-clarification.md（话术三分类/意图卡填法/澄清提问规范/失败模式）、estimation-method.md（C 级信号清单/探测点计数规范/轮数基准表/每轮上下文体积估算与 KV 缓存认知/预算落盘闸门）、templates.md（意图卡/计划文档/任务书/摸底计划/每日简报五套成文模板）；scripts/ 新增 ledger.py（summary/tasks/validate 三子命令，JSON 输出，夹具实测通过：无预算任务正确标记）。version 0.2.0 → 0.3.0。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿（100 行，soft_rules 0）；ledger.py 以临时账本夹具实测三子命令，字段解析与告警正确。
