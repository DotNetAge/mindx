---
version: "1.11.0"
pub_date: 2026-10-31
sha: "0c9c87a8f6a64d98f4a52ee0af9ed7b1aac64bd34a1d872d6a9e19662ab3637b"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-10-31 skill-creator 规则增强-内生资源网络优先

- 问题：规则缺失——取材链只有本地 references 与外搜互联网两级，缺群体共识层：技能进化需要领域参考时直接外搜，圈内已沉淀的文章（社区三大文章库：官方/组织/个人节点）不在取材路径上；自产文章若全文粘贴进 references 会造成双源漂移（文章迭代后技能内副本过期）。
- 修正：SKILL.md 1.10.0 → 1.11.0。两处增强：(1) 六条写作纪律第 3 条取材顺序三级化——本地 references/ → 社区三大文章库（官方优先于组织、组织优先于个人，个人文章按旁证对待）→ 外搜兜底；自产文章只引结论并标注 `[article:slug]`，不全文粘贴；(2) 步骤 1 素材收集两条路扩为三条路，新增「从内生资源网络取材」——第一站不是互联网而是社区三大文章库（给出 `mindx market article list` 三种范围的命令形态），圈内共识最先沉淀的地方，命中即吸收，未命中再外搜。同轮 runtime/AGENTS.md 新增知识升级路由总纲，查证顺序与本技能取材三级化对齐。
- 验证：`mindx skill validate --path` 全绿；正文 grep `定案|定稿|用户提出|今天|实测` 0 命中；`mindx market article list --site --query <关键词>` 可实际检索官方库。
