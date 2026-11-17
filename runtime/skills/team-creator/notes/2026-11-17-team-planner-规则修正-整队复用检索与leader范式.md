---
version: "0.2.0"
pub_date: 2026-11-17
sha: "bd8352d78ee18c912de66079163d9de1381ba85fc9a5ea5212cb41b818a7701c"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 整队复用检索漏现成团队、leader 范式固化出空心领导（glm）

- 问题：规则缺失（场景未覆盖或规则模糊）——①步骤 5「能复用绝不新建」只搜 `--kind agent`，无团队级检索步骤：software-dev（2026-10-17 已上架市场，定位词「软件研发全流程」）被大秘漏掉，蓝图复用论证通篇停在 Agent 层与技能层，整队零字评估，导致重复建制；②注意事项原文规定「leader 的 SOUL 只写协调职责」并把「leader 埋头干活」列为失败态——按此配方造出的 leader 无手艺只能靠收全量报告获得确定感（bega 会议 182 次 LLM 调用烧 3.9 美元，其中约 160 次是 worker 摸底，无人拦）。
- 否决：把 leader 改为「纯协调、禁用工具」型——否决，无手艺的 leader 无从判断报告真伪，确定性全押在 worker 自述上；正确范式是抽查验证自己动手（小验证 ≤5 轮工具调用内出结论），协调职责保留。
- 修正：①步骤 5 团队级检索前置（本地 `mindx team list --json` + `mindx market list --kind team`，关键词覆盖需求全流程词，禁只搜岗位词）；②蓝图模板加「与现成团队对比」节，缺节不放行建制；③注意事项 leader 条目改写为「估算 + 阶梯派工 + 抽查验证自己动手」，协调职责保留；④反模式加「复用论证只下沉到 Agent 层与技能层」。version 0.1.0 → 0.2.0。
- 验证：`mindx skill validate --path runtime/skills/team-planner` 全绿（133 行，soft_rules 0）；用「给项目配个软件研发团队」复演步骤 5，首动作是整队检索而非逐岗位检索。
