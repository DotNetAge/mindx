---
version: "1.10.0"
pub_date: 2026-10-25
sha: "3d9bbe378cf6a3077f0d641cfca69fb24d42580a8c8c3686d02491dc39d1e8b4"
modifier: Ray
modifier_id: "1"
---

## 2026-10-25 技能引用文件用了中文文件名（glm）

- 问题：规则缺失——skill-creator 只限定技能目录名/frontmatter name 用 kebab-case，references 等引用文件名无语言约束；建 org-builder 时沿 notes 履历的中文先例建了 references/样板库.md，破坏库内 42 个既有 references 文件名全英文的实践惯例。
- 否决：只在项目 notes 记纠正、不动技能——经用户拍板写入 skill-creator 成文（实践惯例升硬规则）。
- 修正：文件结构约定加「文件名语言」条目；创建工作流步骤 3 与审查 C 段各加文件名英文检查点；version 1.9.0 → 1.10.0。org-builder 的 references/样板库.md 已改名 templates.md。
- 验证：新建技能逐文件核对文件名；validate 不机检文件名语言（stats 无此项），靠建文件自查 + 审查 C 人工对照。
