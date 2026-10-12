## 2026-10-11 大参考目录索引规范（doubao）

- 问题：规则缺失——≥300 行的参考文章无索引，LLM 检索大文件需全文扫描，定位低效；用户要求"第一段建索引，标题/锚点/行号全标明"。
- 否决：不采用"只标章节不标行号"（LLM 仍需扫描定位）；不采用"锚点仅跳转不标行号"（Read 工具按行号定位最可靠）。
- 修正：best-practices.md 新增「大参考目录索引」节——≥300 行 references 必须在 frontmatter 后插 `<!-- TOC-START/END -->` 索引块（章节/锚点/行号表格，行号=相对文件开头实际行号）；新增幂等脚本 scripts/gen_reference_toc.py 自动生成（跳过 <300 行与已有索引，正文变更后删标记重跑重算行号）；SKILL.md version 1.4.4 → 1.4.5。
- 验证：unity-dev 5 个 ≥300 行文件跑脚本生成索引（programming-patterns 2067 行 54 标题、scriptableobjects 1329 行 56 标题等），validate 0 issue，双目录 md5 一致；<300 行文件正确跳过。
