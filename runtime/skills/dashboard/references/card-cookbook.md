# 卡片控件写法手册（card-cookbook）

手写仪表板时的控件参考。每个控件给：结构、写法要点、骨架。骨架是起点——按对话场景改字段与视觉，不照搬。

**样式纪律（先读）**：颜色一律 UIKit token（`var(--mx-*)`，宿主已注入，亮暗自动跟随）——文字四级 `--mx-text/-secondary/-tertiary/-caption`，状态 `--mx-success/warning/danger/accent/business`，分隔线 `--mx-separator-soft`，间距 `--mx-space-1..7`。**禁止写死色值**（`#12b76a` 这类）。同款部件在多卡重复出现时，公共样式提升到 board 级 `<style>`，卡挂 `class`，卡内只留差异（下面的骨架为独立可读保留卡内样式，实板应上提）。

## stat 大数字卡

- 结构：主数字（600 28px）+ 副说明（12px，`--mx-text-secondary`）。
- 要点：数字用语义色表达好坏（好 `var(--mx-success)`、坏 `var(--mx-danger)`、中性继承）；副说明写趋势或口径（「环比 +3」「含 1 个 P0」）。
- 常用于仪表板顶部概览行（4 张 × span 6）。一行内各卡结构一致，只换数据与颜色——**它们的样式应写在 board 级 `<style>`**。

```html
<card id="s1" class="stat" span="6" title="待处理">
  <div class="s"><span class="n">14</span><span class="d">含 1 个 P0</span></div>
</card>

<!-- board 级 <style> 内（同款统计卡只写一次） -->
<style>
  .stat .s{display:flex;flex-direction:column;gap:4px}
  .stat .n{font:600 28px/36px var(--mx-font-family)}
  .stat .d{font-size:12px;color:var(--mx-text-secondary)}
</style>
```

## progress 进度条

- 结构：标签行 + 圆角条（高 10px）+ 内条按百分比。
- 要点：条底 `color-mix(in currentColor, 12%, transparent)`；里程碑刻度用绝对定位竖线；图例说明刻度含义。
- 变体：分段条（多目标各自进度）、环形（SVG stroke-dasharray，适合单一百分比指标）。

```html
<card id="p1" span="24" title="迭代进度">
  <style>
    .bar{position:relative;height:10px;border-radius:5px;background:color-mix(in currentColor,12%,transparent)}
    .bar i{display:block;height:100%;width:71%;border-radius:5px;background:var(--mx-success)}
    .mark{position:absolute;top:-4px;width:2px;height:18px;background:color-mix(in currentColor,45%,transparent)}
  </style>
  <div style="font-size:13px;margin-bottom:6px">总体进度 71%</div>
  <div class="bar"><i></i><span class="mark" style="left:25%"></span></div>
</card>
```

## bars 水平分布条

- 结构：行 = 名称（定宽）+ 轨道条 + 数值（定宽右对齐）。
- 要点：条宽 = 该项 ÷ 最大项 × 100%；多色时每行 `i` 单独指定语义色；行数 3~7 最可读。
- 适用：严重级分布、部门人数、技术栈构成。

```html
<card id="b1" span="12" title="分布">
  <style>
    .row{display:flex;align-items:center;gap:10px;margin-bottom:10px;font-size:13px}
    .row .name{width:64px;flex-shrink:0;color:var(--mx-text-secondary)}
    .row .track{flex:1;height:10px;border-radius:5px;background:color-mix(in currentColor,10%,transparent)}
    .row .track i{display:block;height:100%;border-radius:5px;background:var(--mx-danger)}
    .row .v{width:24px;text-align:right;font-weight:600;flex-shrink:0}
  </style>
  <div class="row"><span class="name">P0</span><span class="track"><i style="width:100%"></i></span><span class="v">1</span></div>
</card>
```

## funnel 漏斗

- 结构：步骤行 = 阶段名（定宽）+ 按占比递减的实心条 + 数值。
- 要点：条宽 = 阶段 ÷ 第一阶段 × 100%；末段（转化完成）换 `var(--mx-success)`，其余 `var(--mx-accent)`；条上文字 `var(--mx-text-on-accent)`；窄段给 min-width 保可读。

## flow 流转列

- 结构：flex 多列，列头（名称+计数）+ 任务卡（文本 + meta 行：优先级标签/负责人/日期）。
- 要点：列数 3~5；列头与 meta 文字用 `--mx-text-secondary/-tertiary`；优先级标签语义色固定（P0 danger/P1 warning/P2 business）全板一致，淡底 `color-mix(in srgb, var(--mx-danger) 12%, transparent)`；任务卡底 `color-mix(in currentColor, 6%, transparent)` 圆角 8。

```html
<card id="f1" span="24" title="流转">
  <style>
    .cols{display:flex;gap:12px}.col{flex:1;min-width:0}
    .col h4{margin:0 0 8px;font-size:12px;font-weight:500;color:var(--mx-text-secondary)}
    .t{padding:8px 10px;border-radius:8px;background:color-mix(in currentColor,6%,transparent);margin-bottom:8px;font-size:13px}
    .t .meta{display:flex;gap:6px;margin-top:6px;font-size:11px;color:var(--mx-text-tertiary);align-items:center}
    .tag{padding:1px 8px;border-radius:999px;font-size:11px}
    .p0{background:color-mix(in srgb,var(--mx-danger) 12%,transparent);color:var(--mx-danger)}
  </style>
  <div class="cols">
    <div class="col"><h4>待处理 <b>3</b></h4>
      <div class="t">导出报表超时<span class="meta"><span class="tag p0">P0</span><span>李响 · 09-30</span></span></div>
    </div>
  </div>
</card>
```

## table 数据表

- 结构：table + 表头（12px / `--mx-text-tertiary` / 底边框 `--mx-separator-soft`）+ 行（底边框同色）。
- 要点：技术标识（端口/路径/编号）用 `var(--mx-font-mono)`；状态列用状态点（7px 圆点）或徽标；行数控制在重点子集（TopN / P0P1），全量数据不给仪表板。

## timeline 垂直时间线

- 结构：左轴线（::before，`color-mix(in currentColor, 18%, transparent)`）+ 节点圆点（done `var(--mx-success)` / doing `var(--mx-accent)` + 光圈 `color-mix(in srgb, var(--mx-accent) 18%, transparent)` / 未开始 30% 透明）+ 文本 + 日期（`--mx-text-tertiary`）。
- 要点：节点状态最多三种；时间自上而下按发生顺序；适合里程碑、审批流、事件序列。

## gantt 甘特图（纯 CSS）

- 结构：日期刻度头（grid 列头）+ 行（名称定宽 + 轨道）+ 轨道内条形（grid-column 定位起止）+ 今日竖线。
- 要点：跨度按仪表板周期定刻度（日/周）；条形上写任务名（超宽省略），条文字 `var(--mx-text-on-accent)`；按人或按模块分行；今日线 `var(--mx-danger)` 细竖线贯穿。

```html
<card id="g1" span="10" h="300" title="排期">
  <style>
    .g{display:grid;grid-template-columns:72px repeat(14,1fr);font-size:12px;row-gap:6px;align-items:center}
    .g .h{color:var(--mx-text-tertiary);text-align:center;font-size:10.5px}
    .g .name{color:var(--mx-text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;padding-right:8px}
    .g .track{grid-column:2 / span 14;position:relative;height:14px;border-radius:4px;background:color-mix(in currentColor,6%,transparent)}
    .g .bar{position:absolute;top:2px;bottom:2px;border-radius:4px;background:var(--mx-accent);color:var(--mx-text-on-accent);font-size:10px;line-height:10px;padding:0 6px;white-space:nowrap;overflow:hidden}
    .g .today{position:absolute;top:-2px;bottom:-2px;width:1.5px;background:var(--mx-danger);left:50%}
  </style>
  <div class="g">
    <span></span><span class="h">09-24</span><span class="h">25</span><span class="h">26</span><span class="h">27</span><span class="h">28</span><span class="h">29</span><span class="h">30</span><span class="h">10-01</span><span class="h">02</span><span class="h">03</span><span class="h">04</span><span class="h">05</span><span class="h">06</span><span class="h">07</span>
    <span class="name">李响</span>
    <span class="track"><i class="bar" style="left:0;width:42%">报表优化</i><span class="today"></span></span>
  </div>
</card>
```

## members 成员墙

- 结构：auto-fill 网格（minmax 150px）+ 成员项（圆形头像 + 姓名/角色 + 状态点）。
- 要点：头像圆点底色轮换 token（`var(--mx-accent)` / `var(--mx-success)` / `var(--mx-warning)` / `var(--mx-business)`），字取姓氏、色用 `var(--mx-text-on-accent)`；状态点 success=在岗、warning=忙碌、30% 透明=休假/离线；真实人员数据必须来自用户提供的材料。

## echarts 图表卡

- 要点：容器 `div` 定高 + `h` 属性给初值（240 常用）；CDN 引 `echarts@5`；**主题色从卡内读 token**：`var c = getComputedStyle(document.documentElement); var accent = c.getPropertyValue('--mx-accent').trim()`——轴文字、系列色用它或 `color-mix` 派生，网格线用 `color-mix(in currentColor, 15%, transparent)`；`window.addEventListener('resize', chart.resize)` 必加。
- 适用：趋势双线（新增 vs 修复）、柱状对比、占比饼图。简单趋势用 sparkline（SVG polyline）更轻。

## 设计新控件

标准控件不合用时自己设计：先想「这个信息最有效的形态是什么」（数字？条？列？图？），再用样式纪律实现——token 取色、`--mx-space-*` 取距、`--mx-radius-*` 取角、系统字体。参考 `example-<场景>.dash` 范例的整体效果。
