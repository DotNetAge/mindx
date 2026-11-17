---
version: "0.3.0"
pub_date: 2026-11-17
sha: "068915b8a0257430f433e343747e2d1494c8d16e972b42c079706b25dfc85243"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 更名组建团队、蓝图瘦身、leader 对齐门收编（glm）

- 问题：①目录名 team-planner 与英文名只覆盖「规划」，实际执行建制（install/add/hire）——用户拍板统一为 team-creator；②自包含违规：无 references/，编制规范靠 agent-creator 的 references，常规任务额外 Read ≥2 且跨技能目录；③蓝图「团队职责」占位符塞 90 字规则长段复述 team-design 第 7 条；④注意事项 3 条与正文重复、「重要说明」2 条均重复、步骤 6 尾句零信息；⑤L21「走 agent-creator 的审查流程」断链且与前提检查「告知用户并停止」冲突；⑥leader 岗硬性要求原在 agent-creator 步骤 3（上下文错位），收编到本技能；⑦name_zh「团队创建」直译被否决。
- 修正：①目录与英文名改 team-creator，name_zh 改「组建团队」，H1 同步；②team-design.md 自 agent-creator 迁入 references/（git mv 保留历史），编制规范就地引用；③蓝图占位符压缩为指针，加「蓝图是编制决策唯一交接物」句；④leader 硬性要求（1 门管理技能 + 1 门领域手艺技能，缺一不建队）写入蓝图「负责人选定」节并作为步骤 5 建制前核对项；⑤噪声清理；⑥L21 冲突统一为「告知用户并停止」。version 0.2.0 → 0.3.0。
- 验证：`mindx skill validate --path runtime/skills/team-creator` 全绿；evals 补单 Agent 误触发用例（招单个人不用建团队 → false）。
