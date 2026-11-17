---
version: "1.1.0"
pub_date: 2026-11-17
sha: "9689c541ce142581a023bab33c06f7e13d82aea629c7ad6ddcd5a908c18fc077"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-17 定位为呈现类、输入源改为不会被清理的目录（workbuddy）

- 问题：指令冲突——本技能第三处查找路径指向 `.agents/notes/`，而 notes 是待提取的低置信素材，项目规则与 retrospect 都规定提取后删除；呈现类技能依赖一个会被清理的目录，输入会持续变薄。同时 description 未声明本技能不产知识，容易被当作复盘入口而越界提炼规则。
- 修正：①description 加「呈现类技能：只读现成资料、汇总成人话，不提炼规则也不沉淀知识」；②查找路径第三处由 `notes/` 改为 `memory/`（长期资产，稳定存在），并注明确实要最新踩坑时才补查 notes；③第二步「最近动态」来源同步改为 reports/memory；版本 1.0.0 → 1.1.0。
- 验证：`mindx skill validate` 全绿（50 行）；`grep notes` 仅剩解释性说明一处；`mindx skill add` 后运行时与源 diff 为 0。
