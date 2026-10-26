---
version: "1.7.0"
pub_date: 2026-10-22
sha: "8e30a43b6de23cb569cb99e2765084bf09b29039b43fb4888c3b0dc0ada67bf0"
modifier: "Ray"
modifier_id: "1"
---

# 2026-10-22 skill-creator 规则修正：体积上限 150→200 行 + 正文瘦身 596→121

## 触发

全量体检 18 个内置技能时 skill-creator 本体 596 行超限 4 倍（上限 150）。用户定案（2026-10-22）：上限 150 行太少，定为 200 行；同时对 skill-creator 本体做优化。

## 修正内容

1. `internal/core/skillstore/quality.go`：`maxBodyLines` 150 → 200（校验实现唯一事实源，make restart 后全局生效）
2. SKILL.md 正文瘦身 596 → 121 行：脚本设计指南节删除（using-scripts.md 已完整承接，正文留索引路由）；「先给选项再问问题」示例删除（best-practices.md「主题不明先澄清」承接）；「评估输出质量主干」删除（evaluating-skills.md 承接）；六条原则压缩为一句式；反模式与重要说明合并为「纪律与边界」节
3. version 1.6.0 → 1.7.0

## 效果

- validate PASS（0 error），soft-rule 3 条均为「建议」词面误报（名词用法/词表引用行自指），逐条核对不动作
- agent-creator 184 行在新上限 200 内，自动合规免手术
- 校验语义修正：size 告警含义从「复盘蒸馏候选技能应短小聚焦」仍适用于候选技能，但上限基准统一定为 200
