# 仪表板设计规范（design-spec）

生成任何 `.dash` 前必须通读本文。仪表板是宿主应用的界面，不是自由发挥的网页——与主应用同一套设计系统，亮暗主题自动跟随。宿主已把 UIKit 全部 `--mx-*` 语义 token 注入卡内文档，写样式只用 token。

## 颜色（禁止写死色值）

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

## 文字与部件观感

- 层级三段式：大数字/标题（`--mx-font-title` 或 600 28px）→ 正文（14px 继承）→ 辅助行（12px `--mx-text-secondary`）。一张卡内层级不超过三级。
- 状态一律「徽标 = 语义色字 + 12% 同色淡底」，不用彩色大色块。
- 数字是仪表板的主角：关键指标用大数字卡，一屏 stat 行不超过 4~5 张。
- 空态 / 缺数据：用 `--mx-text-tertiary` 写明「暂无数据 + 原因」，不留空白卡。
- 技术标识（端口/路径/编号）用 `var(--mx-font-mono)`。

## 图表

容器定高 + `h` 属性给初值（240 常用）；轴文字、图例、网格线、系列色从卡内读 token：`getComputedStyle(document.documentElement).getPropertyValue('--mx-accent').trim()`——跟随主题。

## 共享样式（同款部件零重复）

同款部件（如 4 张统计卡）样式重复出现 = 错误做法。公共样式提升到 board 级 `<style>`，卡挂 `class`，卡内只写差异。机制见 `dash-spec.md` 的「元素与属性定义」。

## 红线级纪律

- 文字、状态数字、边框、底色一律 token——写死 `#12b76a` 这类固定色值是错误做法（暗色主题下不可读、与应用观感割裂）；辅助文字禁止 opacity 调灰（用文字层级 token）。
- 各场景指引的「语义色映射」表给出本场景约定，全板必须一致。
- 设计规范之外确需自定义色时，用 `color-mix` 从 token 派生，不引入新色值。
