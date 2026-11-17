---
version: "0.4.0"
pub_date: 2026-11-17
sha: "cda5f3de455a95481dcdeb984a136e3ff7086783d37654a24b730d66eacbf8b6"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 老板确认门与派发方式判定（glm）

- 问题：用户指出三处缺陷——①写完计划就能马上开工，缺老板认可门（计划是给人看的，认可了才能动手）；②计划全文字，抽象概念无图示；③派发死板绑 Cron——Cron 只该用于断开式长时等待，当场能收结论的实效任务应该用 SubAgent。
- 修正：第四步新增「老板确认门（派工前必过）」——计划含估算与派发方案呈现老板认可后才动手，未获认可不得派发；第二步计划质量判据新增图文并茂（抽象概念 mermaid 表达），templates.md 计划模板加「图示」节带 mermaid 示例；第四步派发改为方式判定（当场能出结论 → SubAgent 直发回收，断开等待 → Cron 排程落盘验收，判据：能当场收结论的不排程），账本任务行「cron 表达式」改「执行方式」；每日检查例行明确与 daily-report 技能边界——进度简报（数据源=账本核对，老板拍板）与工作日报（数据源=轮次日志收割，复盘交接）是两种产物不混做，老板要日报路由 daily-report（实证：daily-report 输入来源为 report collect 轮次收割/TODO/notes/git log，与账本无交集，故 goal-planner 不引入该技能）。version 0.3.0 → 0.4.0。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿。
