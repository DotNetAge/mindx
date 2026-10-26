---
version: "1.4.7"
pub_date: 2026-10-11
sha: "f501c23b448cf92bf078e53bae71649f5c1c82990028eb0881822442d825098e"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-11 技能三层结构核心心智模型（doubao）

- 问题：规则缺失——原规范把 SKILL.md/references/scripts 当作分散的"可选目录"描述，未定义三层各自的本质职责；用户经长任务实践提出完整体会：SKILL.md=工作指导（决策路由）、references=作业指导书（怎么做，全量收集）、scripts=算力简化（高频重活脚本化），三者构成完备编写规范，全量参考可把 LLM 决策路径锁在最短路径上。
- 否决：不采用"三层平铺各自讲用法"的写法（失去主次）；不采用"把三层纪律全部写进 best-practices 而不进 SKILL.md"（核心心智模型必须进正文核心原则首位）。
- 修正：SKILL.md 核心原则新增「技能三层结构」节（表格定位 + 三层构成完备规范），文件结构约定同步标注三层职责；best-practices.md 开头新增同题详版（三层纪律 + 决策路径最短化检验法）；version 1.4.5 → 1.4.6。
- 验证：双目录 md5 一致（SKILL.md 3a760fab、best-practices.md 13b5155c）；validate 仅存量的 frontmatter `***`/size/soft-rule 问题，无新增。

## 2026-10-11 scripts 第三类场景补充（doubao）

- 问题：规则缺失——scripts 定位只写"高频重复、长时费算力"，漏了第三类：含技术含量的专业操作（一个程序能确定性解决复杂专业问题，即使频率不高也应优先脚本，不留给 LLM 现场推理）。
- 修正：SKILL.md 三层结构表格 scripts 行与正文扩为三类（高频重复/长时费算力复杂操作/专业操作），判断标准="有程序能确定性解决的操作，优先脚本"；best-practices 同步；version 1.4.6 → 1.4.7。
- 验证：双目录 md5 一致；无新增 validate issue。
