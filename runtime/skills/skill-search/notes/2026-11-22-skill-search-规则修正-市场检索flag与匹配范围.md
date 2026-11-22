---
version: "1.0.1"
pub_date: 2026-11-22
sha: "bf4e9463d7d8d99753b99ae11379055b77ab7b410eb8803d83fa76d0b7f5d9f4"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-22 市场检索命令与匹配范围说明均与实现不符（assistant）

- 问题：指令冲突——第二步写 `mindx market list --kind skill --filter <关键词>`，CLI 无此 flag（只有 `--q`）；同处注释称「匹配名称/描述/类别」，服务端实际只匹配 name/description，类别是独立 `category` 参数。执行者按文档调用会先报错，再按错误假设理解召回范围。
- 修正：命令改为 `--q`，匹配范围改为「名称/描述」；SKILL.md version 1.0.0 → 1.0.1。CLI 侧 `market list` 增加 `--filter` 隐藏弃用别名。
- 验证：`mindx skill validate --path runtime/skills/skill-search` 全绿；`mindx market list --kind skill --q "测试" --json` 正常返回。

