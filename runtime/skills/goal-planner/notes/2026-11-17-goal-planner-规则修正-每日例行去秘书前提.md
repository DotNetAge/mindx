---
version: "0.5.2"
pub_date: 2026-11-17
sha: "b8ccddfb54269a078816f12fce4fe50854a140f2a6a8bfd359226db81e5b8b50"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 每日例行去秘书前提（glm）

- 问题：用户指出「每日检查例行（秘书自己的循环任务）」两处缺陷——①括注是角色叙事噪声，对执行 LLM 零信息；②整节写死「有秘书跑循环任务」的前提，没有秘书时该路径断档，决策路由缺输入不全分支。
- 修正：节标题与首句去秘书绑定，改为「承接本技能的执行者（专职秘书、负责人或任一 Agent 均可）」；排程改二选一路由——有专职执行者走 Cron 循环任务（session_id 用 new），没有则不建循环任务、用户问进度或下达新目标时现场过清单（缺的只是定时性，不是能力）。version 0.5.1 → 0.5.2。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿。
