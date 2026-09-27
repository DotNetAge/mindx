---
name: generate-agents-md
description: 扫描仓库，生成/更新AGENTS.md、子目录AGENTS.md、.agents规则片段；产出物严格遵循AAIF AGENTS.md社区规范。
tags: agents-md, scaffolding, meta-skill
metadata:
  name_zh: 生成AGENTS.md
---
## 何时用
用户指令：生成、新建、重构、更新项目AGENTS.md，或者重建.agents目录脚手架。

## 前提
1. 加载本技能配套参考规范：
   - references/AAIF_AGENTS_SPEC.md（AGENTS.md文件规范）
   - references/AGENTS_FOLDER_SCAFFOLD_SPEC.md（.agents脚手架目录规约）
2. 所有产出文件与目录结构，**必须严格遵守两份参考文档定义**。

## 工作流
1. 仓库扫描：遍历目录树、package.json/构建脚本、测试命令、源码、README，识别技术栈、敏感文件、禁止修改文件清单、模块划分（monorepo）。
2. 脚手架规划阶段【新增】
   - 读取 `AGENTS_FOLDER_SCAFFOLD_SPEC.md`，确定需要创建的`.agents/`目录骨架；
   - 判断哪些目录是必选，哪些按需创建；**不创建空文件夹**，只有存在对应内容时才落地目录；
   - 规划规则片段拆分：把大块规则抽取为独立md片段，放到`.agents/rules/`；
   - 规划技能存放：如果后续需要新增skills，预留`.agents/skills/`路径。
3. AGENTS.md 文件规划
   - 根AGENTS.md：默认merge全局模式；
   - 识别独立模块，按需生成子目录AGENTS.md，子目录可声明`Mode: override`；
   - 在AGENTS.md中使用`@`语法引用`.agents/rules/`下拆分的规则片段。
4. 自检校验
   对照两份spec自检：目录结构合规、AGENTS.md作用域/继承逻辑、token体积、无冗余废话。
5. 输出完整预览：
   - 列出将要创建/修改的**所有文件+所有文件夹**；
   - 展示AGENTS.md预览和`.agents`目录树预览；
6. 必须等待用户确认，才允许写入磁盘；绝不静默覆盖已有AGENTS.md或已有.agents目录内文件。

## 输出约束
1. AGENTS.md 只写Agent必须知道的约束，不复制README长篇项目介绍，减少token开销。
2. 禁止堆砌通用软件工程常识，只保留项目独有的约束、红线、可执行命令。
3. 子目录AGENTS.md仅写差异化规则，默认复用父目录规则，遵循merge默认策略。
4. 脚手架创建原则：**按需创建，不空建目录**；已有目录/文件不强制删除，只增量补充缺失项。

## 注意事项
- 不同Agent平台对嵌套AGENTS.md支持不一致，重要红线规则建议放置根目录兜底。
- `@`片段导入是AAIF社区约定，部分老Agent不支持。
- 本技能**仅生成符合规范的文件，不负责强制运行时校验**；AGENTS.md只是指引，不是沙箱硬隔离。