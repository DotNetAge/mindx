## 2026-10-11 references 元信息头改为 frontmatter 强制（doubao）

- 问题：规则缺失——原「references 元信息头」是 markdown 引用行（`> 核验日期：... ｜ 来源：...`），无法作为机器可读的更新检查依据；用户要求"一切的引文都应该在 frontmatter 上加上出处(url)、更新日期"。
- 否决：不采用「保留引用行、仅追加 frontmatter」双格式（新旧口径并存会产生两种写法，扫描器要兼容两套正则）；不采用「只加 updated 不加 source」（出处缺失就无法按 URL 核验陈旧）。
- 修正：best-practices.md「references 元信息头」节改为 YAML frontmatter 强制（`source`+`updated` 必填、`scope` 可选），frontmatter 后直接进正文；新鲜度检查 5 步同步改为读 frontmatter `updated`/`source`；SKILL.md version 1.4.3 → 1.4.4。
- 验证：mindx skill validate 0 issue；skill-audit-scheduler 扫描正则同步改为 `updated:\s*\d{4}-\d{2}-\d{2}` 后实跑识别 unity-dev 9 个文件全部命中、超期候选 0。
