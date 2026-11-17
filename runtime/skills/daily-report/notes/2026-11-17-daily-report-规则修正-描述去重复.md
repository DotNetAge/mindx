---
version: "1.2.2"
pub_date: 2026-11-17
sha: "b398d6521ddac7196be6da0e868e8b90bc122d6c7799fcae93cceac9c5da864d"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 去掉描述里的重复表达（workbuddy）

- 问题：输出格式——description 里「生成复盘报告」与「复盘流程收尾需要落报告」是同一件事的两种说法，且「呈现类技能」四字属分类标签，对触发无帮助。
- 修正：改为「只呈现不提炼：把已有事实整理成人能读的汇报，不产规则也不产技能」；末尾重复句合并为「或自省流程收尾需要落报告时使用」（随复盘技能改名自省同步措辞）。version 1.2.1 → 1.2.2。
- 验证：`mindx skill validate` 全绿（97 行）；description 单行，含功能、边界、触发三要素。
