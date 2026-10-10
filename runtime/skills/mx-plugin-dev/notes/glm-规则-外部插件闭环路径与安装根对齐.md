# mx-plugin-dev 进化履历

## 2026-10-08 按 SKILL 逐字执行外部插件闭环在第 3 步卡死（glm）

- 问题：流程漏洞——SKILL.md 闭环示例与 contract.md §18.7 的 `mw plugin pack ./com.acme.widget` 仍是脚手架「缺省当前目录」时代的路径；权威源目录军规定稿为 `.agents/plugins/<id>/` 后示例未同步，按文执行报「插件目录不存在」。附带暴露双根断点：mw CLI 缺省根 = 发行版 userData（`MindX Work/plugins`），dev 实例根 = `…/@mindx-work/electron/plugins`，「app 即时感知激活」在 dev 环境不成立且技能未提示。
- 否决：把脚手架缺省输出改回当前目录——与「官方可选件与 LLM 开发插件统一权威源目录 `.agents/plugins/`」的用户定稿矛盾，否决。
- 修正：SKILL.md 闭环示例与 contract.md §18.7 的 pack/install 路径改为 `.agents/plugins/…`；SKILL 补「安装根对齐军规」（dev 验收时 CLI 侧显式 `MW_PLUGIN_ROOT` 指向实例根；主进程不读此 env，发行 app 与 CLI 缺省根天然一致）；contract.md §18.7 补双根事实、§18.8 启动注释补双根警告。frontmatter 补 `metadata.requires`（python3 / mw / pnpm / node / python），version 0.1.0 → 0.2.0。
- 验证：mindx-work 根下 `python3 .agents/skills/mx-plugin-dev/scripts/scaffold_plugin.py com.probe.hello --market "Probe" && mw plugin pack .agents/plugins/com.probe.hello && mw plugin install .agents/plugins/com.probe.hello-1.0.0.mindpkg && mw plugin list --json && mw plugin uninstall com.probe.hello` 全链路通过（实测 2026-10-08，SKILL 旧路径 `./com.probe.hello` 同轮实测失败即证据）；修复后 `mindx skill validate --path <技能目录>` ok=true。
