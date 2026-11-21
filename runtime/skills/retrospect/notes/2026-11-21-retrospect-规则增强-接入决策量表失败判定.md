---
version: "2.6.0"
pub_date: 2026-11-21
modifier: "Ray"
---

## 2026-11-21 接入决策量表失败判定作为技能候选主来源（retrospect）

- 问题：流程漏洞——本技能的素材源只有 notes / memory / 技能履历三处，全是「自己写下的推论」。决策量表里已存的 `fail` / `rework` 判定（含 `JudgmentRecord.note` 的四段式正文）从未被读取，导致「哪次实际做错了」这个最直接、最客观的技能来源被整条跳过。实测 `~/.mindx/data/metric_events.yml` 216 条事件中 `rating` / `overall` 为 0、`judgments` 仅 1 条，量表当时只产成本数字、不产可读失败文本。
- 否决：改 Go 聚合层让它消费 `Note`——`Note` 在服务端零消费方，但为自省专门改 daemon 聚合口径会污染 `metric compare` 的归因；且在技能侧读原文件更轻，不必等发版。另否决引入 pyyaml——量表是本项目 yaml.v3 自产（全量读写、无锚点别名、缩进仅 2/4/6），行级状态机够用，少一个部署依赖。
- 修正：新增 `scripts/metric_scan.py`（子命令 `failures` / `stats`），直接读量表 YAML 输出失败判定与四段式完整度；SKILL.md 步骤 2.3 增加第 6 条素材源，明确「比率数字不构成素材，note 才说明错在哪」「缺段先补齐再升技能」「同任务意图多条失败为强信号」。version 2.5.0 → 2.6.0。
- 验证：真实量表 216 事件解析与手工核对一致（round 216 / 判定 2 / fail 1）；边界用例五项全过——多值 `skills_used` 列表、`task_hint` 值含中英文冒号（实测 216 条中 36 条含冒号，只按首个冒号切分才不截断）、判定版本链取最新（success + rework 并存取 rework）、块标量 `|-` 多行 note 含空行、无 note 的失败判定；`mindx skill validate --path` ok=true、171 行、soft_rules 0、issues 空。
- 踩坑：`yaml.v3` 把多行 note 一律序列化成 `note: |-` 块标量，首版解析器只认行内标量，把四段式正文读成字面量 `"|-"` 并判成「0 段」——这会把全部真实素材误判为不可用，比不接更坏。凡解析本项目自产 YAML，必须先用手工核对过的真实样本验证块标量与嵌套列表两种形态。
- 已知限制（未修）：`metric.record` 的判定固定落在该会话**最新的 round 事件**（`handler_metric_judgment.go:213` `latestRoundEvent` 按 TS 降序取 events[0]），无法指定落到具体哪一轮。实测写入一条关于「误判能力缺失」的判定，它被记在了该会话最后一轮（任务为「用 ls 数一下顶层子目录有几个」）上——note 内容与 task_hint 因此可能错配。对自省的影响：失败素材本身可用（note 完整、归属会话正确），但不能据「判定所在轮次」反推是哪一步出的错，须结合 note 正文判断。