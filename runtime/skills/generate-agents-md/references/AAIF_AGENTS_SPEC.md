# AAIF AGENTS.md 社区规范参考文档
> 来源：Agentic AI Foundation（Linux基金会）轻约定标准，无强制Schema，约定如下：

## 1. 文件命名与作用域
1. 文件名：`AGENTS.md`，大写。放置在哪一个目录，就作用于该目录及其全部子目录。
2. 就近优先级：越深层目录，优先级越高。加载顺序：用户全局AGENTS.md → 仓库根 → 逐层子目录。
3. 默认策略：merge合并（继承上层规则；冲突条目，子目录覆盖父目录）。
4. 可选头部声明：`Mode: override`，声明本目录完全抛弃上层所有规则。
5. 同级高优先级兜底文件：`AGENTS.override.md`，优先级高于同目录AGENTS.md。

## 2. .agents/目录约定
`.agents/`为配套目录，存放规则片段、子Agent定义；不会自动生效。使用`@path`语法在AGENTS.md内导入片段，仅为社区约定，非所有平台支持。

## 3. 内容规范
- 文件载体：纯Markdown，无强制章节、无固定YAML schema。
- 推荐内容：目录职责、构建/测试命令、编码规范、文件修改边界、禁止操作清单。
- 不强制：不需要固定章节结构。

## 4. 平台兼容备注
- 现代平台：Windsurf、新版Cursor、Codex：完整支持嵌套merge/override。
- 老旧Agent：仅读取根目录AGENTS.md，不递归扫描子目录。
