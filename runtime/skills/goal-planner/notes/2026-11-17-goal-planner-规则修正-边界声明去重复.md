---
version: "0.5.3"
pub_date: 2026-11-17
sha: "d7c5da8a2f450b2eece28cda206d60e005ebb6624ea7ced9d05a38dd1bfccd66"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 边界声明去三连复述与跨文件双源（glm）

- 问题：用户抓出 daily-report 边界段仍是噪声——①段内同义重复：一句规则三连复述（「只产出进度简报不做日报」「要日报路由 daily-report」「不混做」）；②跨文件双源：该边界被原样复制进 templates.md 第五节。此前噪声清扫漏网原因：只扫了节间重复，未扫段内复述，也未对自己刚写入的内容做跨文件查重。
- 修正：SKILL.md 边界压为一句（产出什么 + 日报路由 + 一处判别依据）；templates.md 第五节删边界复述只留模板用途标注；grep 验证「不混做/数据源」全技能仅剩一处。version 0.5.2 → 0.5.3。
- 验证：`mindx skill validate --path runtime/skills/goal-planner` 全绿。
