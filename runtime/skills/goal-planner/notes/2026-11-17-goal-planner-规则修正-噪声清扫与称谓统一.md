---
version: "0.5.1"
pub_date: 2026-11-17
sha: "b54a1e86312208166bf03e06aad8fff5802b83b988118a73c173fdec3b95abbc"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 噪声清扫与称谓统一（glm）

- 问题：用户要求站在 LLM 视角按 skill-creator 噪声五类判据复审。命中五处：①确认门段重复强调（「写完计划不等于开工授权」是前半句的反说）；②intro 与账本节标题逐字重复「支撑工件，唯一事实源」；③第四步括注「不死板用 Cron」与两分支同义；④estimation-method 六节标题带日期注记（正文禁日期）；⑤档位硬闸门在 estimation-method 全文复述（重复下沉）。另发现称谓歧义：SKILL.md「老板/用户」混用，对 LLM 是同一实体两个路由名。
- 修正：四文件「老板」全量改「用户」；确认门压缩为一句话；账本标题去重（intro 保留定义）；删括注；六节标题去日期（updated 已在 frontmatter）；estimation-method 档位复述改指针；bega 项目专名中性化为「本地计费实测」；「他的原话」改「用户原话」。version 0.5.0 → 0.5.1。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿；goal-planner 目录 grep「老板」零命中。
