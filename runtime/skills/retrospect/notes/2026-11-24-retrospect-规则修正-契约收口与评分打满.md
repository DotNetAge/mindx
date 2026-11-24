---
version: "2.7.0"
pub_date: 2026-11-24
sha: "3e2e265b8afb2826b7da2d13d8f5ca83fcf3cb0e9ce79663cea015f782395eaa"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-24 自省技能指令互相冲突且质量门不可执行（retrospect）

- 问题：指令冲突——`SKILL.md` 对 Light 给出两种互斥定义（只减债 / 增量聚类），压缩与 notes 删除又同时写着“不用确认”和“不可逆要确认”；`scan.py debt` 扫全部项目会话、未知活动默认放行；质量门引用不存在的 `softscan.py`，150 行上限与正文 175 行冲突；`fapprox` 把本轮新记忆判 P0，报告模板没有待压清单；全局与项目规则各硬编码一次 20 条阈值。
- 否决：保留 Light 的两种语义，让执行者按上下文自行选择。否决理由：同一输入会得到不同模式，自省结果不可复现；也否决只改文档不改脚本，因为跨项目压缩与未知活动放行是代码层风险，文档约束不住。
- 修正：`SKILL.md` 重写为 143 行——Light 固定为只减债，Full 固定为全跑，快速整理走窄范围；压缩改为一次用户确认后批量执行，当前/活跃/未知活动三类一律 defer；报告必须写待压清单；技能履历同时扫项目与全局目录。`scan.py` 增 `--project-dir` 与 `--current-session-id`，未知活动改为 defer；`fapprox.py` 增 `--skip-newer-than`；`quality-gate.md` 删除 `softscan.py` 步，改读 `mindx skill validate` 的 `soft_rules` 与 `issues`，P0 改为提示而非删除。全局 `AGENTS.md` 与项目 `AGENTS.md` 的 20 条硬编码改为指针。version 2.6.4 → 2.7.0。
- 验证：`mindx skill validate --path .../retrospect` 输出 `ok: true`、`lines: 143`、`soft_rules: 0`、`issues: []`；合成会话验证 `debt` 过滤掉别的项目、未知活动与当前会话都进 `defer`，可压会话进 `compact_batch`；Bega 实跑 `scan.py debt` 返回 `checked 2`、无超阈；`fapprox.py ... --skip-newer-than 24` 输出 `skipped_new: 4`、不把四条新记忆判死。
