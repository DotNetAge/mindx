---
version: "1.1.0"
pub_date: 2026-10-25
sha: "26f0d2eeb4e1c60687537dfb2d6c7db922ba1ec6d3a0a537e3e6c3df7d73648b"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 CI 发布状态描述与代码脱节、本机发布路径漏收（glm）

- 问题：规则缺失——SKILL.md 写「产物经 upload-artifact 保留（Release 上传未启用）」，而 release.yml 早已设 `MX_PUBLISH=always` 直传 GitHub Release draft（electron-builder publisher 缺省 draft）；且 2026-10-24 新增的本机发布路径 scripts/release.mjs（`pnpm release` / `release:dry`）未收录，技能只覆盖一半发布工作流。用户调查打包发布全貌时发现此漂移，随即定案「mindx-work 发布必须调用 mx-release」写入 AGENTS.md 发布纪律节，技能必须与事实对齐。
- 修正：CI 打包段改为「MX_PUBLISH=always 直传 draft + artifact 留档」；新增「本机发布」段（release.mjs 流程、防呆、定位）；description 补触发词「走本机 release.mjs 发版」；frontmatter 补 metadata（name_zh + version）。version 初建无字段，自 1.1.0 起记。
- 验证：对照事实源 release.yml L78-85（MX_PUBLISH: always）、scripts/pack.mjs L94-95（发布开关）、scripts/release.mjs 全文（流程与防呆）；`mindx skill validate --path` 全绿。
