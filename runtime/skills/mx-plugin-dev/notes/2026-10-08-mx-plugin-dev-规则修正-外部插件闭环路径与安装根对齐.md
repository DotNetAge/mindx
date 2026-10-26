---
version: "0.2.0"
pub_date: 2026-10-08
sha: "76095196b41b10c881be33d6044c1fdf949cc466fcb4c4981b4f04be7451e0db"
modifier: "Ray"
modifier_id: "1"
---

# 外部插件闭环路径与安装根对齐（glm）

## 2026-10-08 按 SKILL 逐字执行外部插件闭环在第 3 步卡死（glm）

- 问题：流程漏洞——SKILL.md 闭环示例与 contract.md §18.7 的 `mw plugin pack ./com.acme.widget` 仍是脚手架「缺省当前目录」时代的路径；权威源目录军规定稿为 `.agents/plugins/<id>/` 后示例未同步，按文执行报「插件目录不存在」。附带暴露双根断点：mw CLI 缺省根 = 发行版 userData，dev 实例根 = `…/@mindx-work/electron/plugins`，「app 即时感知激活」在 dev 环境不成立且技能未提示。
- 否决：把脚手架缺省输出改回当前目录——与「官方可选件与 LLM 开发插件统一权威源目录 `.agents/plugins/`」的用户定稿矛盾，否决。
- 修正：SKILL.md 闭环示例与 contract.md §18.7 的 pack/install 路径改为 `.agents/plugins/…`；contract.md §18.7 补双根事实。frontmatter 补 `metadata.requires`（python3 / mw / pnpm / node / python），version 0.1.0 → 0.2.0。
- 验证：scaffold → pack → install → list → uninstall 全链路实测通过（旧路径同轮实测失败即证据）；`mindx skill validate` ok=true。
