# assets/ — tiangong-skill 静态资源目录

本目录存放供 Agent 生成流程引用的静态资源文件，包括但不限于：

| 资源类型 | 文件名 | 用途 |
|---------|------|------|
| 色标方案配置 | `color-schemes.yaml` | 定义多 Agent 编排界面的部门色标映射表，供岗位型 Agent YAML frontmatter 中的 `color` 字段引用 |
| 模板 JSON Schema | `template-schema.json` | 岗位型 / 人格蒸馏产物的统一 JSON Schema 校验文件 |

## 使用方式

- 资源文件由 `tiangong-skill` skill 在对应阶段按需读取。
- 新增资源后更新本 README。
