---
version: "1.1.2"
pub_date: 2026-11-17
sha: "d8bfe149ecdece1e1ccc9d850815ffd68ee8132ba5bfca8d4c47c1e86189861e"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 去掉描述里的同义重申（workbuddy）

- 问题：输出格式——description 结尾「想一次性了解各项目进展时使用」与前三个问法（所有项目怎么样/盘点项目/各项目进展）表达同一意图，属同义重复；「呈现类技能」是分类标签而非触发条件。
- 修正：并为一句「收集全部项目概况并汇总成人话，只读不提炼、不产规则也不产技能。当用户问「…」时使用」。version 1.1.1 → 1.1.2。
- 验证：`mindx skill validate` 全绿（60 行）；description 单行三要素齐全。
