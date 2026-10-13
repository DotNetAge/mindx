---
updated: 2026-10-13
---

# 预判模板：项目跟踪

## 适用信号

USER.md 或 Team leader 工作方向命中：软件开发、工程迭代、产品交付、多任务并行推进；工作记录出现"进度、里程碑、迭代、排期、分工"等词汇。

## 标配功能清单（场景标配，无需用户告知）

- 迭代进度总览（当前迭代完成率、剩余工作量）
- 任务分工与状态跟踪（待办 / 进行中 / 已完成，按负责人分列）
- 里程碑时间线（关键节点与实际偏差）
- 风险与阻塞项提示

## L1 方案（最低交付物，零开发）

路由 dashboard 技能生成项目跟踪仪表板：场景路由读其 `references/scene-project.md`，落 `.agents/dashboards/project-tracking.dash`。数据取自工作痕迹中的进度记录（TODO.md 与 memory/notes 显性陈述、reports 周期进度、logs 实际推进行为）；缺数据留占位卡。

## L2 插件方案（升级路径）

L1 承载不了时才立项：需要富处理（跨项目数据聚合、进度推演、与 daemon 任务数据实时联动）。

- 三方分工——Agent 操作面：任务状态流转、进度记录由 Agent 经 RPC 写入；用户知情面：总览与分工看板打开即读；协作点：里程碑验收确认、优先级裁决（可缺省）
- 席位初稿：Sidebar 导航入口 + Content 进度主视图 + Detail 任务详情（对照 mx-plugin-dev references/seats.md）
- 骨架场景：basic + detail
- 数据来源：daemon 任务 RPC + 项目内数据文件

## 数据来源

工作痕迹（按置信度）：TODO.md 与 `.agents/memory/`、`.agents/notes/`（进度陈述）、reports（周期总结中的进度模式）、logs（实际推进行为序列）；动态数据层：daemon 任务 RPC（L2）。数据从生态来，不从用户手里收。
