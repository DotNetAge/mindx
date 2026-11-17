---
version: "1.4.0"
pub_date: 2026-11-18
sha: "7d922fafa93f2b8cd50b3be516dec0e4d27826cccd4e074e5dbfeb2459f830aa"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-18 技能配比上限 2-3 过紧，上调至 10（glm）

- 问题：用户裁决「上限太少了，上限最多 10 吧」——原 2-3 上限对多平台运营、全栈统筹类角色的真实装配面过紧，1.1.0 的否决理由（上下文开销）依然成立，但数值定得过于保守。
- 否决：无（用户直接裁决数值，未走备选评审）。
- 修正：步骤 3「最多 2-3 个」→「最多 10 个」；三级检索出路 1「守住 2-3 上限」→「守住 10 个上限」；反模式「最多 2-3 个是预算约束」→「最多 10 个是预算约束」。职责-能力对齐门的前置地位不变——先对齐职责再谈预算的次序不受数值调整影响。version 1.3.3 → 1.4.0。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿（issues 空，soft_rules 0）；grep 全技能库确认 2-3 上限表述仅存在于 agent-creator（team-creator 无此条），修订后正文零残留。
