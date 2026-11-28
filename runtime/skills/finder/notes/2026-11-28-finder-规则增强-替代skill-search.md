---
version: "1.0.0"
pub_date: 2026-11-28
sha: "68c62bea5a6656bc029268e6a5534e53ec4b9ce5dc344f5adbe43fe08ecbcf6e"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-28 旧技能只覆盖一类对象，检索方式还是关键词（default）

- 问题：版本兼容——`skill-search` 只搜 skill 一类，市场侧早已提供 `mindx market search` 语义检索（五类 shelf 各自独立），旧技能仍停留在 `market list --q` 关键词匹配；「合同」「宠物」这类中文词实测 0 命中，遇到跨类需求（找角色、找班底、找插件）无能为力。另含两条失效指令：`mindx reload skills` 子命令不存在，只能走 `mindx skill add`。
- 否决：在 SOUL.md 里写「先查全部已装技能」的检索纪律。全量目录 237 条 description 约 3.5 万字，直接打爆上下文，且 SOUL 无法按需收窄范围。
- 修正：删除 `runtime/skills/skill-search`，新建 `runtime/skills/finder` v1.0.0（能力搜索）：不查本地全量库，直接 `market search --kind <skill|agent|team|plugin> --k 5 "<整句>" --json` 单类检索，分数 ≥0.65 且 description 对口才算命中，再用 `skill get` / `agent get` / `team get` 对照本机装没装，用户要使用时才 `market install`（plugin 只检索不安装，`install` 只认 agent/skill/team）。同步改引用：`agent-creator/SKILL.md:98`、`agent-creator/references/agent-schema.md:74-75`、`mindx-work/plugins/src/chatflow/input/index.vue:161` 常用技能名单，并把 default 的 `IDENTITY.md` skills 列表加入 finder（仓库模板与本机副本各一份）。
- 验证：`mindx skill validate --path runtime/skills/finder` 全绿（120 行、0 软化词）；`make restart` 重编后 `~/.mindx/skills/skill-search` 不再释放、`finder` 已释放，`mindx agent get default` 的 skills 含 finder。四类实测：skill「财报拆解」0.817 earnings-analysis；plugin「输入框加翻译按钮」最高 0.624 全部低于阈值（返回空，符合预期）；agent「iOS 上架审核」出现 0.701 的 opensource-maintainer 实则跑偏——据此补了「分数只是入场券，还要核对 description 对口」一条纪律。
- 备注：`mindx-market/docs/设计定案-市场语义检索五工具.md` 第 19/290/342 行仍以 `skill-search` 作为历史锚点，属定案记录，未改动；其中 `--q --top 5` 的命令写法与现网 `--k` 不一致，落地时以 `--k` 为准。
