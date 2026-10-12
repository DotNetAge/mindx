---
name: generate-agents-md
description: 扫描仓库，生成/更新AGENTS.md、子目录AGENTS.md、.agents规则片段，产出物严格遵循AAIF AGENTS.md社区规范。当用户要求生成、新建、重构、更新项目的 AGENTS.md 或重建 .agents 目录脚手架时使用。
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

## 更新模式（存量修订）
触发：复盘产出的 AGENTS.md 修订提议（notes 中的缺口类记录）经用户授权，或用户直接要求更新已有 AGENTS.md。
流程：先读现有 AGENTS.md 与修订提议，逐条评估——与现有结构冲突的提议先与用户协商；采纳项按 AAIF 规范落地（根/子目录归位、@片段拆分、token 纪律），增量修订不整文件重写；写前展示 diff 预览（沿用工作流第 5-6 步确认机制）。

## 主题澄清（动手前必做）

用户未指明目标目录或范围时，禁止自行猜测。先用 AskUser 提供 2-4 个假设选项让用户选择，用户不选再开放提问：

- **目标目录不明**（"生成 AGENTS.md"未指明哪个项目）→ 列出候选：当前会话项目 / 最近活跃项目（`mindx project list` 结果）/ 用户口述的其他项目。
- **范围不明** → 列出候选：根目录 AGENTS.md / 根 + 子目录 / 只补 .agents 脚手架。
- **模式不明** → 列出候选：全新生成 / 存量更新（含复盘修订提议）。

每项给具体选项 + "其他（自由输入）"兜底。目标与范围明确后进入工作流。

## 工作流

进入工作流前先 `TaskCreate` 建立任务清单（扫描 → 脚手架规划 → 文件规划 → 自检 → 预览 → 确认写入，含每步产出），推进中用 `TaskUpdate` 标记完成状态，中断恢复时用 `TaskList`/`TaskGet` 读取进度，不凭回忆对齐工作点。

1. 仓库扫描：并行侦察，不逐文件通读——
   - 包清单与框架指纹：package.json/go.mod/Cargo.toml/pyproject.toml 等定位语言与依赖；next.config/vite.config/django 等定位框架；
   - 入口点：main/index/app/server、cmd/、src/main/；
   - 目录快照：顶层 2 级树，忽略 node_modules/vendor/dist/build/.git 等生成物；
   - 配置与工具链：lint/format/tsconfig、Makefile/Dockerfile、CI 工作流、.env.example；
   - 测试结构：tests/、*_test.go、*.spec.ts、pytest/jest/vitest 配置；
   - 约定信号：文件命名风格、错误处理模式、近期提交信息风格（历史太浅则跳过并注明）。
   最终判定：技术栈、敏感文件、禁止修改文件清单、模块划分（monorepo）。
2. 脚手架规划阶段【新增】
   - 读取 `AGENTS_FOLDER_SCAFFOLD_SPEC.md`，确定需要创建的`.agents/`目录骨架；
   - 环境适配：运行环境为 mindx（`~/.mindx` 存在）时，notes/memory/references 三个治理目录纳入按需规划（见 spec 的 mindx 扩展节）；非 mindx 环境只按社区 spec 规划；
   - 判断哪些目录是必选，哪些按需创建；**不创建空文件夹**，只有存在对应内容时才落地目录；
   - 规划规则片段拆分：把大块规则抽取为独立md片段，放到`.agents/rules/`；
   - 规划技能存放：如果后续需要新增skills，预留`.agents/skills/`路径。
3. AGENTS.md 文件规划
   - 根AGENTS.md：默认merge全局模式；
   - 识别独立模块，按需生成子目录AGENTS.md，子目录可声明`Mode: override`；
   - 在AGENTS.md中使用`@`语法引用`.agents/rules/`下拆分的规则片段；
   - 平台别名（可选）：AGENTS.md 是唯一正典；当用户环境需要 CLAUDE.md（Claude Code 平台的指令入口文件）等别名时，生成内容仅为一行 `@AGENTS.md` 的指针文件，禁止复制正文——避免双正典漂移。
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
5. 治理条款分层：notes/memory/references 使用规则、复盘路由等通用治理条款由用户全局 AGENTS.md 兜底，项目级 AGENTS.md 不复制，只写项目独有约束——防 token 冗余与双真理漂移。

## 注意事项
- 不同Agent平台对嵌套AGENTS.md支持不一致，重要红线规则必须放置根目录兜底。
- `@`片段导入是AAIF社区约定，部分老Agent不支持。
- 本技能**仅生成符合规范的文件，不负责强制运行时校验**；AGENTS.md只是指引，不是沙箱硬隔离。