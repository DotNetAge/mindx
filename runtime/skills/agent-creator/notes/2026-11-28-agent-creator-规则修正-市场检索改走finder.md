---
version: "1.4.3"
pub_date: 2026-11-28
sha: "febbeef1d6710562d4a6ca4c7f35f8886a9090bd377c106fddee1ee6374dcaa4"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-28 步骤 1 的市场检索仍是关键词（default）

- 问题：版本兼容——步骤 1「本地与市场双重检索」的市场一侧用 `mindx market list --kind agent --q "<领域关键词>"`，关键词路对中文不稳，且未标注命中阈值，容易把沾边包当候选。
- 否决：只改命令不改裁决口径。命令换了但不给阈值判定，等于把噪声交给执行者。
- 修正：市场一侧改 `mindx market search --kind agent --k 5 "<岗位职责 + 领域整句>" --json`，并指明按 `finder` 技能的裁决规则（≥0.65 且描述对口才算命中）。version 1.4.2 → 1.4.3。
- 验证：`mindx skill validate --path runtime/skills/agent-creator` 全绿。
