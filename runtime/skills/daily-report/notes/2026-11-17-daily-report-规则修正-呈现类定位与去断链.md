---
version: "1.2.0"
pub_date: 2026-11-17
sha: "ea0849f8799b9d95af02d0c3632b1d97bf64d0c736f228c2ec6e38e9cb5002a6"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 定位为呈现类、去掉对已删技能的引用（workbuddy）

- 问题：版本兼容 + 指令冲突——①description 与正文两处指向 `deep-review`（「大复盘收尾需要落报告」「deep-review 传入的分析结论」），该技能已拆解删除，引用断链；②frontmatter 顶层 `version: 1.1` 与 `metadata.version: 1.0.0` 并存且不一致，版本号两处真源；③项目级 `.agents/skills/daily-report/` 存有一份旧措辞副本（「下一步建议与计划」），与全局版分叉，按项目级覆盖规则实际生效的是旧副本。
- 否决：保留项目级副本只同步措辞——分叉会持续产生两套真源，且项目级副本不进版本控制，丢失风险高于收益。
- 修正：①description 与正文输入来源改称「复盘流程」，不再指名具体技能；②顶层 version 删除，只留 metadata.version，1.1 → 1.2.0；③description 明写「呈现类技能：把已有事实整理成人能读的汇报，不提炼规则、不沉淀技能」，与 retrospect 的沉淀职责划清；④项目级旧副本归档至 `.agents/memory/.archive/skill-daily-report-projectcopy`，项目内回落使用全局版。
- 验证：`mindx skill validate` 全绿（89 行）；`mindx skill add` 后运行时副本与源 diff 为 0；`grep deep-review` 在本技能零命中。
