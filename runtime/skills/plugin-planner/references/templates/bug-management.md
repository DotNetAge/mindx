---
updated: 2026-10-13
---

# 预判模板：缺陷管理

## 适用信号

USER.md 或 Team leader 工作方向命中：软件开发、测试交付、质量保障；工作记录出现"缺陷、bug、回归、修复、验收"等词汇。

## 标配功能清单（场景标配，无需用户告知）

- 缺陷清单（编号、标题、严重度、状态流转：新建 → 处理中 → 已修复 → 已验证）
- 按严重度 / 模块分布的统计看板
- 趋势视图（新增 vs 关闭，暴露收敛速度）
- 阻塞性缺陷置顶提示

## L1 方案（最低交付物，零开发）

路由 dashboard 技能生成缺陷管理仪表板：场景路由读其 `references/scene-bug.md`，落 `.agents/dashboards/bug-overview.dash`。数据取自工作痕迹中的缺陷记录（memory/notes 显性陈述、reports 周期质量数据、logs 修复行为）；缺数据留占位卡。

## L2 插件方案（升级路径）

L1 承载不了时才立项：需要富处理（缺陷趋势推演、与代码仓库/测试报告的关联分析）。

- 三方分工——Agent 操作面：缺陷登记、状态流转、指派由 Agent 经 RPC 写入（用户口述"修这个"，Agent 修复后自动流转状态）；用户知情面：分布与趋势看板打开即读；协作点：修复验收确认、严重度裁决（可缺省）
- 席位初稿：Sidebar 导航入口 + Content 缺陷主视图 + Detail 缺陷详情（对照 mx-plugin-dev references/seats.md）
- 骨架场景：basic + detail
- 数据来源：daemon KV 存储（mindx-kv 技能可作数据层）+ 项目内缺陷记录文件

## 数据来源

工作痕迹（按置信度）：`.agents/memory/` 与 `.agents/notes/`（显性缺陷陈述）、reports（周期总结中的质量数据）、logs（修复行为序列）；动态数据层：daemon KV（L2，Agent 经 RPC 读写）。数据从生态来，不从用户手里收。
