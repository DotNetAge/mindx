---
name: dashboard
description: 仪表板构建技能（单技能全场景）。当用户要求"仪表板 / 数据面板 / 监控墙 / 可视化卡片"时使用，覆盖：项目跟踪 / 迭代进度 / 任务分工 / 敏捷 / 里程碑、个人 OKR / 目标跟踪 / 待办驱动、需求管理 / 需求池 / 优先级与状态跟踪、任务看板 / 看板视图 / 卡片流、人力盘点 / 团队画像 / 组织架构 / 招聘进度、缺陷管理 / BUG 跟踪 / 质量监控、架构总览 / 服务拓扑 / Archify 产物植入，以及任意自定义领域的数据仪表板。含工作流程、单文件协议、board/card 属性、设计规范（UIKit token 注入）、红线与场景路由。
metadata:
  name_zh: 构建仪表板
---


## 何时用

用户想要任何「仪表板 / 仪表板 / 数据面板 / 监控墙 / 可视化卡片」时。本技能单点收口全部仪表板场景：通用规范（工作流程、协议、样式、红线）在本文件，各业务场景的布局设计语言与内容引导在 `references/scene-*.md`——场景指引提供的是**设计语言与内容引导**，不是照抄的模板。

## 工作流程

1. **场景分析（强制先读场景指引）**：从对话判断业务场景，按「场景路由」读对应 `references/scene-<场景>.md`——该仪表板给谁看、布局设计语言、部件内容引导、本场景语义色映射、取数来源都在里面。**未读对应场景指引不得开工**；自定义领域仪表板参照最接近的场景指引自由设计。
2. **布局设计**：以场景指引的「布局设计语言」为起点（如项目跟踪的左右双通栏），结合对话场景增删部件、调整占比，样式遵守「设计规范」。一屏讲完核心信息，不要堆卡。
3. **取数**：按场景指引的取数来源收集真实数据内联为静态值。绝不编造数据；缺数据就留占位卡并告诉用户。
4. **生成**（二选一）：
   - **脚本生成**（默认首选，标准部件为主时）：写一份仪表板 spec（JSON，格式见 `scripts/new_board.py --help`），跑脚本产出 `.dash`——部件 HTML 由脚本内建渲染（已按本技能样式规范输出 token 引用），Agent 只写数据与结构，大幅节省上下文。
   - **手写**（布局/部件定制化强时）：直接写 `.dash`，控件写法按需查阅 `references/card-cookbook.md` 与场景范例 `references/example-<场景>.dash`。
5. **落盘**：`.agents/dashboards/<名字>.dash` 单文件即生效，用户端自动刷新，无需用户操作。更新 = 重写该文件（只改受影响卡片的标记）。

## 文件协议（单文件，HTML 方言）

一个 `.dash` 文件 = 布局 + 共享样式 + 全部卡片内容，自包含，无外部数据文件。

```html
<board name="缺陷总览" gap="12" class="qa">
  <style>
    .stat .n { font: var(--mx-font-title); }
    .stat .d { color: var(--mx-text-secondary); font-size: 12px; }
  </style>
  <card id="open" class="stat" span="6" title="待处理">
    <div class="n">11</div><div class="d">含 2 个 P1</div>
  </card>
</board>
```

| 位置      | 属性/元素        | 格式                 | 意义                                                                                                        |
| --------- | ---------------- | -------------------- | ----------------------------------------------------------------------------------------------------------- |
| `<board>` | name             | 任意字符串           | 仪表板显示名（面板头部与侧栏清单标题）；缺省回退文件名。取业务名而非文件名复读                              |
| `<board>` | gap              | 整数 px（0..48）     | 卡片列间距，缺省 12                                                                                         |
| `<board>` | class            | 空白分隔类名         | 注入**每张卡**的文档 body，配合 board 级 `<style>` 一次作用全部卡                                           |
| `<board>` | `<style>` 子元素 | CSS 原文             | **board 级共享样式**：注入每张卡文档 head（卡内自带样式靠后可覆盖）。同款部件的公共样式写这里，卡内只留差异 |
| `<card>`  | id               | 字符串，board 内唯一 | 卡内标识：宿主用它把卡内上报的高度对应回这张卡。取语义短名                                                  |
| `<card>`  | span             | 整数 1..24，缺省 6   | 24 栏栅格占几栏。6=四分之一行、8=三分之一行、12=半宽、16=三分之二行、24=整行                                |
| `<card>`  | h                | 整数 px，缺省 120    | **初始**占位高度，实际高度自动跟随内容（内置高度桥）。图表/架构图卡建议设 h 给稳定初值                      |
| `<card>`  | class            | 空白分隔类名         | 注入**该卡**文档 body（与 board class 合并），被 board 级 `<style>` 选择                                    |
| `<card>`  | title            | 字符串，可选         | 卡片头部小标题（宿主渲染）。纯装饰卡可省                                                                    |

- 解析按 HTML 宽容处理：未闭合标签、裸 `&`、属性不引号都不炸；同名 id 会导致高度回报错乱，必须唯一。

## 设计规范（全部仪表板统一遵守）

仪表板是宿主应用的界面，不是自由发挥的网页——与主应用同一套设计系统，亮暗主题自动跟随。宿主已把 UIKit 全部 `--mx-*` 语义 token 注入卡内文档，写样式只用 token：

### 颜色（禁止写死色值）

| 用途                             | 写法                                                                                                                             |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 正文 / 主要文字                  | `var(--mx-text)` 或直接继承（正文默认就是它，多数情况不用写）                                                                    |
| 次级 / 辅助 / 弱化文字           | `var(--mx-text-secondary)` / `var(--mx-text-tertiary)` / `var(--mx-text-caption)`（替代 opacity 调灰）                           |
| 数字大字 / 标题                  | `font: var(--mx-font-title)`；卡内小标题 `var(--mx-font-heading)`；辅助行 `var(--mx-font-caption)`                               |
| 成功 / 告警 / 危险 / 强调 / 信息 | `var(--mx-success)` / `var(--mx-warning)` / `var(--mx-danger)` / `var(--mx-accent)` / `var(--mx-business)`                       |
| 状态徽标淡底                     | `color-mix(in srgb, var(--mx-success) 12%, transparent)`（warn 有现成淡底 `var(--mx-state-warn-soft)`；配同色文字 = 亮暗都可读） |
| 分隔线 / 边框                    | `1px solid var(--mx-separator-soft)`（更弱）或 `var(--mx-separator)`                                                             |
| 间距                             | `var(--mx-space-1..7)`（4pt 网格：4/8/12/16/20/24/32）                                                                           |
| 圆角                             | 控件 `var(--mx-radius-control)`（8px）/ 卡级 `var(--mx-radius-card)`（12px）                                                     |
| EP 色阶派生（light-3/9 等）      | `color-mix(in srgb, var(--mx-accent) 70% / 10%, var(--mx-bg-window))`——`--el-*` 变量卡内不可用                                   |

唯一例外：ECharts 等图表的**数据系列区分色**允许少量自定义色板，但也应从 token 读值派生亮度基调（见「图表」）。

### 文字与部件观感

- 层级三段式：大数字/标题（`--mx-font-title` 或 600 28px）→ 正文（14px 继承）→ 辅助行（12px `--mx-text-secondary`）。一张卡内层级不超过三级。
- 状态一律「徽标 = 语义色字 + 12% 同色淡底」，不用彩色大色块。
- 数字是仪表板的主角：关键指标用大数字卡，一屏 stat 行不超过 4~5 张。
- 空态 / 缺数据：用 `--mx-text-tertiary` 写明「暂无数据 + 原因」，不留空白卡。
- 技术标识（端口/路径/编号）用 `var(--mx-font-mono)`。

### 图表

容器定高 + `h` 属性给初值（240 常用）；轴文字、图例、网格线、系列色从卡内读 token：`getComputedStyle(document.documentElement).getPropertyValue('--mx-accent').trim()`——跟随主题。

### 共享样式（同款部件零重复）

同款部件（如 4 张统计卡）样式重复出现 = 错误做法。公共样式提升到 board 级 `<style>`，卡挂 `class`，卡内只写差异。

### 红线级纪律

文字、状态数字、边框、底色一律 token——写死 `#12b76a` 这类固定色值是错误做法（暗色主题下不可读、与应用观感割裂）；辅助文字禁止 opacity 调灰（用文字层级 token）。各场景指引的「语义色映射」表给出本场景约定，全板必须一致。

## 基底环境（宿主注入，卡内直接依赖）

- 字体系统栈、14px、行高 1.5；背景透明（宿主卡片自带底色）。
- 脚本：`<script>` 允许执行。隔离沙箱**无同源权限**（读不到父页面/本地文件/localStorage）；外部 CDN（ECharts 等）可用。
- token：宿主注入 UIKit `--mx-*` 全集 + board 级 `<style>`，随主题切换自动刷新。

## 红线

- 卡内禁止再出现 `<board>` / `<card>` 标签（破坏布局解析）。
- 不要写 `<html>/<head>/<body>`——卡内 HTML 原样放入独立 iframe 文档。
- 每卡自带**差异样式**，公共样式上提 board 级 `<style>`（隔离文档互不可见，类名冲突无所谓）。
- 禁止写死色值（设计规范之外的场景用 `color-mix` 从 token 派生）。
- 未读对应场景指引（`references/scene-*.md`）不得开工。

## 场景路由（读哪份场景指引）

| 业务场景                                   | 场景指引                      | 版式要点                             |
| ------------------------------------------ | ----------------------------- | ------------------------------------ |
| 项目跟踪、迭代进度、任务分工、敏捷、里程碑 | `references/scene-project.md` | 左右双通栏：左全局甘特，右部件栈     |
| 个人 OKR、目标跟踪、待办驱动               | `references/scene-okr.md`     | 概览 stat 行 + 目标达成列 + 分布双列 |
| 需求管理、需求池、优先级与状态跟踪         | `references/scene-req.md`     | 概览 stat 行 + 分布双列 + 明细表     |
| 任务看板、看板视图、卡片流、泳道分列       | `references/scene-kanban.md`  | 概览 stat 行 + 状态列 flow 大通栏    |
| 人力盘点、团队画像、组织架构、招聘进度     | `references/scene-hr.md`      | 概览行 + 分布/漏斗双列 + 成员墙      |
| 缺陷管理、质量监控、问题跟踪               | `references/scene-bug.md`     | 概览行 + 趋势/分布双列 + 流转 + 明细 |
| 架构总览、服务拓扑、Archify 产物植入       | `references/scene-arch.md`    | 主体架构图大通栏 + 清单与分布        |

场景不匹配的自定义领域仪表板：参照最接近场景指引的布局语言自由设计部件，控件写法查 card-cookbook。

## 资源（按需读取，勿整读）

- `references/scene-project.md` / `scene-okr.md` / `scene-req.md` / `scene-kanban.md` / `scene-hr.md` / `scene-bug.md` / `scene-arch.md`：场景指引（给谁看、布局设计语言、部件引导、语义色映射、取数来源）。
- `references/card-cookbook.md`：标准控件写法手册（结构 + 要点 + token 骨架），手写或设计新控件时查阅。
- `references/example-<场景>.dash`：该场景完整范例仪表板——是「结构与效果参考」，**结合对话场景重新设计，禁止照抄数据与布局**。
- `scripts/new_board.py`：模板化输出脚本（内建 stat/progress/bars/funnel/flow/table/timeline/gantt/members/raw 渲染器，输出已符合本技能样式规范）。`python3 scripts/new_board.py --help` 看用法与 spec 格式。
