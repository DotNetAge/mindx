---
version: "0.2.8"
pub_date: 2026-11-28
sha: "d1d7a354ba92ef605608510f73ba3e967d814ffe61e6a13f8b2859653ba1042f"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-28 走查维度里的市场检索仍是关键词（default）

- 问题：版本兼容——行为走查的「市场货架」维度用 `mindx market list --kind plugin --q <关键词>`；实测 plugin shelf 支持语义检索（返回 com.mindxwork.* 系列带分数），关键词路反而漏召回。
- 修正：改为按 `finder` 技能执行 `mindx market search --kind plugin --k 5 "<功能整句>" --json`，并注明 ≥0.65 且对口才算命中。version 0.2.7 → 0.2.8。
- 验证：`mindx skill validate --path runtime/skills/plugin-planner` 全绿。
