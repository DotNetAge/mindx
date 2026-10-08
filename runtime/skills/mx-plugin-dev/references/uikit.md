# 界面样式规范

界面样式一律服从本规范；壳契约（八区/席位/注册 API）读 [contract.md](./contract.md)。

## 权威与纪律

1. **mx-UIKit 是唯一权威**：权威定义位于仓库 `ui-shell-vue/src/styles/tokens.css`（token）、`controls.css`（控件原语）、`docs/样式原语.md`（清单）。界面实现一律服从这三处定义，禁止绕开自行造样式。
2. **禁止字面色值**：一律 `var(--mx-*)`。新颜色先入 tokens.css（亮暗两块都写）再使用。
3. **禁止页内样式**：一律 CSS Modules；唯一例外是动态数值经 CSS 自定义属性注入。
4. **伪类全量显式**：`:hover/:active/:focus-visible/:disabled` 一个不许省。
5. **注释用中文**，只解释用法与意图。
6. **主题仅亮/暗双套**，根节点 `data-mx-theme` 切换（`createThemeController` 三档：亮/暗/自动）。组件永不感知主题名，只消费语义 token；写样式时两套主题都要成立。
7. **状态由属性驱动**：开关用 `aria-checked`、页签用 `aria-selected`、色调用 `data-tone`/`data-state`，视觉态与无障碍态不可分叉。
8. **几何勿改**：原语类名与几何是定稿契约，使用方只组合、不改内部尺寸。

## 色彩规范

所有颜色经语义 token 取用，禁止写死色值。选择路径：先定语义（它在界面里扮演什么角色），再取对应 token。

**功能色**（表达状态与品牌）：

| 角色        | Token                                       | 使用场景                                                               |
| ----------- | ------------------------------------------- | ---------------------------------------------------------------------- |
| 品牌主色    | `--mx-accent`                               | 主按钮、选中态描边、开关开启、链接强调                                 |
| 信息/业务蓝 | `--mx-business`                             | 信息类标签、Toast 动作字、复制成功反馈                                 |
| 成功        | `--mx-state-success`                        | 完成状态点、成功标签（`data-tone='success'`）                          |
| 警告        | `--mx-state-warn` + `--mx-state-warn-label` | 警告标签与连接指示（淡底用 `--mx-state-warn-soft`，深字恒为 label 值） |
| 危险        | `--mx-state-error`                          | 错误状态点、危险标签                                                   |
| 空闲        | `--mx-state-idle`                           | 无活动的状态点                                                         |

**墨色（文字四级）**——层级由深到浅，禁止跳级：

| 层级     | Token                 | 用途                                        |
| -------- | --------------------- | ------------------------------------------- |
| 主文字   | `--mx-text`           | 标题、正文、选中项文字                      |
| 次要     | `--mx-text-secondary` | 说明文字、未选中导航、行标题                |
| 三级     | `--mx-text-tertiary`  | 提示说明、节头、占位引导                    |
| 弱化     | `--mx-text-caption`   | 禁用文字、辅助角标                          |
| 品牌底上 | `--mx-text-on-accent` | accent/business 实底上的文字（随主题反转）  |
| 恒白     | `--mx-static-white`   | 恒暗面（tooltip/toast）上的文字，不随主题变 |

**中性底与边框**：

| Token                                                                              | 用途                                               |
| ---------------------------------------------------------------------------------- | -------------------------------------------------- |
| `--mx-bg-window` / `--mx-bg-surface` / `--mx-bg-elevated`                          | 三层底：窗底 / 侧栏填充 / 浮层与内容卡             |
| `--mx-module`                                                                      | 模块化面板底（分段控件槽、步进胶囊、neutral 标签） |
| `--mx-hover` / `--mx-active`                                                       | 悬停 / 按下的半透明灰底（列表行、菜单项、页签）    |
| `--mx-hover-solid` / `--mx-nav-hover` / `--mx-nav-active`                          | 不透明悬停底 / 侧栏导航悬停 / 侧栏导航选中         |
| `--mx-btn-elevated`                                                                | 新会话类按钮卡的凸起底                             |
| `--mx-separator-soft` / `--mx-separator` / `--mx-border-strong` / `--mx-border-l4` | 0.5px 分隔线 / 常规分隔 / 控件边框 / 强调描边      |
| `--mx-border-selected`                                                             | 卡片选中描边                                       |

**遮罩与浮层**：遮罩层 `--mx-mask` + `--mx-mask-blur`（2px）；浮层本体 `--mx-menu-bg` + `--mx-backdrop-menu`（blur 40 + saturate 150%）；恒暗面 `--mx-tooltip-bg` / `--mx-toast-bg` / `--mx-hovercard-bg`（亮暗同值，配 `--mx-static-white` 字）；交通灯红 `--mx-traffic-light-close`（macOS 系统色，双主题同值，Floater 浮窗模拟红灯）。

**阴影四档**：`--mx-elevation-stroke`（仅 0.5px 发丝描边）→ `--mx-shadow-panel`（浮层面板）→ `--mx-shadow-prominent`（菜单卡）→ `--mx-shadow-lv3`（toast / hovercard 恒暗面）。禁止 color-mix 手拼阴影。

## 字体规范

字族只有一套系统栈 `--mx-font-family`（macOS 上即 SF Pro + 苹方），禁止引入外部字体。字号只允许以下七档，禁止自造字号：

| 级别   | Token               | 值          | 用途                              |
| ------ | ------------------- | ----------- | --------------------------------- |
| 标题   | `--mx-font-title`   | 20/28 · 600 | 页面主标题                        |
| 品牌   | `--mx-font-brand`   | 18/24 · 600 | 侧栏品牌名                        |
| 小标题 | `--mx-font-heading` | 15/22 · 600 | 卡片标题、区块头                  |
| 正文   | `--mx-font-body`    | 14/22 · 400 | 默认正文（body 根字体）、控件文字 |
| 说明   | `--mx-font-caption` | 11/16 · 400 | 辅助说明、节头、Tag 文字          |
| 微字   | `--mx-font-micro`   | 10/14 · 500 | 徽标计数、极小角标                |

用法：`font: var(--mx-font-body);`；同一行内不要混用多档；数字列加 `font-variant-numeric: tabular-nums` 防跳动。字重不用裸值，跟随字阶 token（600 只出现在 title/heading/brand）。

## 布局规范

**间距走 4pt 网格**，只允许 `--mx-space-1..7`（4/8/12/16/20/24/32px），禁止 6/10/14 之类自由值出现在 padding/margin/gap（控件内部像素级微调除外，且须有原语出处）。

| Token            | 值    | 典型用途                         |
| ---------------- | ----- | -------------------------------- |
| `--mx-space-1`   | 4     | 图标与文字微距、标题与说明行距   |
| `--mx-space-2`   | 8     | 控件内边距、菜单行内距、紧凑 gap |
| `--mx-space-3`   | 12    | 卡片内边距、分组间距             |
| `--mx-space-4`   | 16    | 首选项行纵向节奏、卡内边距       |
| `--mx-space-5/6` | 20/24 | 面板分区、页面边距               |
| `--mx-space-7`   | 32    | 大区块分隔                       |

**圆角三档**：`--mx-radius-control`（8，按钮/输入框/菜单行）、`--mx-radius-card`（12，卡片/导航 cell/展开容器）、`--mx-radius-window`（14，窗口级浮层）。胶囊类（开关 10、pill 24、tag 999）用固定半高，不套三档。

**结构尺寸（壳级定稿）**：侧栏展开 248 / 折叠 80（rail 控制盒 36×36；折叠宽对齐 DeepSeek 官方桌面端实测 ≈78px，恰好容纳 macOS 红绿灯排）；导航行高 34 / 半径 8、节头行 36、按钮卡 38（0.5px 边框）、footer 行 42 / 半径 12；设置面板 800 宽 / radius 32 / 左导航 188 / 页项高 40；内容区卡片撑满容器宽度，禁止自加 `max-width`（modal 面板除外）。

**对齐原则**：文本块左对齐；设置行 = 左"标题+说明"列、右控件列，行间 0.5px 分隔线由壳容器统一画（行自身 `padding: 16px 0`，不再画线）；浮层用 `width: max-content` + `max-width` 上限防右缘换行。

**24 栏栅格（结构布局）**：容器加 `.mx-row`（CSS Grid 24 栏），子项加 `.mx-col-1..24`（占 N 栏）、`.mx-col-offset-1..23`（左侧空 N 栏）；gutter 缺省 `--mx-gutter`（16px），`.mx-row-gap-2/3/6`（8/12/24px）覆盖；`.mx-row-justify-start/center/end/between/around/evenly` 与 `.mx-row-align-start/center/end/stretch` 调分布与对齐（行缺省等高拉伸）。响应式三档断点，基础类任意宽度生效、断点类该宽度及以上覆盖：

| 断点 | 阈值      | 类名                                   |
| ---- | --------- | -------------------------------------- |
| sm   | >= 768px  | `.mx-col-sm-N` / `.mx-col-sm-offset-N` |
| md   | >= 992px  | `.mx-col-md-N` / `.mx-col-md-offset-N` |
| lg   | >= 1200px | `.mx-col-lg-N` / `.mx-col-lg-offset-N` |

组合语义：`.mx-col-24.mx-col-md-12` = 窄屏全宽、≥md 半宽。断点数值 token `--mx-breakpoint-*` 仅供 JS `matchMedia` 对齐（media query 内不可用 `var()`）。禁止在栅格外自造百分比/自由宽度做页面分栏。

## 控件原语用法

全部位于仓库 `ui-shell-vue/src/styles/controls.css`，各节头部注释就是用法说明。清单：

按钮 `.mx-btn`（`--primary` 变体）、图标按钮 `.mx-icon-btn`、输入框 `.mx-input`、徽标 `.mx-badge`、卡片 `.mx-card`、下拉 chip `.mx-pill`、首选项行 `.mx-pref-row`/`-title`/`-desc`、开关 `.mx-switch`+`-thumb`（`role="switch"` + `aria-checked`）、复选框 `.mx-checkbox`、菜单卡 `.mx-menu`+`.mx-menu-item`（宿主 `position:relative`）、工具提示 `.mx-tooltip`（fixed 定位由使用方算）、状态点 `.mx-dot[data-state]`（颜色 = `color`）、标签 `.mx-tag[data-tone=outline|solid|neutral|quiet|success|info|warning|danger]`、分段页签 `.mx-tabs`+`.mx-tabs-indicator`+`.mx-tab[aria-selected]`（指示器位移经 JS 写 `transform`）、展开行 `.mx-disclosure` 系列（双图标交叉淡切）、轻提示 `.mx-toast`+`-icon`+`-action`（停留时长传 `--mx-toast-hold`）、连接指示 `.mx-indicator[data-tone]`+`-icon`+`-dots`（三点省略动画）、微光文本 `.mx-shimmer`（必传 `--mx-text-shimmer-spread`）、悬停卡 `.mx-hovercard-host`+`.mx-hovercard`、步进数字框（范式见仓库 `plugins/src/demo/prefs/FontSizeRow.vue`：36 高 / radius 18 / 箭头 hover 显现用 `opacity`）、24 栏栅格 `.mx-row`+`.mx-col-N`+`.mx-col-offset-N`（断点变体与组合语义见上方「布局规范」）。

## 布局范式（仓库内 demo 路径，供 clone 仓库者查阅）

| 场景                               | 范式文件                                 |
| ---------------------------------- | ---------------------------------------- |
| 设置页标准行（标题+说明+右控件）   | `plugins/src/demo/prefs/InfoRows.ts`     |
| 设置行内交互控件（步进数字框）     | `plugins/src/demo/prefs/FontSizeRow.vue` |
| 三卡选择（图标上文字下、选中描边） | `plugins/src/demo/prefs/ThemeRow.vue`    |
| 侧栏节与行                         | `plugins/src/demo/index.ts`              |
| 图标                               | 见下节「图标（MxIcon）」                 |

## 图标（MxIcon）

图标是 Vue 组件原语（非 CSS 原语），纪律归界面军规第 9 条：

- **名称来源**：Iconify 图标库，名称格式 `{collection}:{name}`（如 `lucide:plus`、`lucide:panel-left`）。名称在 Iconify 图标站检索（icon-sets.iconify.design）。项目预打包 `@iconify-json/lucide` 整集作离线兜底，其余集合运行时经 Iconify API 在线加载——优先用 lucide 集。
- **用法**：`<MxIcon name="lucide:xxx" :size="16 或 20" />`。尺寸仅两档：16 常规、20 大位（如折叠钮）——`size` 是字面量联合 `16 | 20`（缺省 16），传其他数值（如 14）vue-tsc 直接报错，没有第三档；颜色经 `currentColor` 继承文字色，禁止写死 color。
- **纪律**：只用单色图标集；禁止手写内联 SVG 图标；禁止引入多色图标。

## 高频错误

- 浮层漏 `backdrop-filter`：遮罩层与浮层本体是两层模糊，都要写。
- fixed 气泡未写 `width: max-content` + `max-width` 上限，右缘会提前换行；不可断 token 另配 `overflow-wrap: break-word`。
- 阴影/颜色自造 color-mix 手拼：用四档 shadow token 与语义色 token。
- hover 才出现的按钮用 `opacity` 不用 `visibility`（保键盘 focus-within 可达）。
- 恒白/恒暗文本误用主题墨色：`--mx-static-white` 与双主题同值的底配套，不随主题翻转。
- **CSS Modules 引用全局类被哈希失配（静默失效，typecheck 全绿 UI 裸奔）**：`<style module>` 里组合选择器含 `mx-*` 全局类（如 `.composer .mx-input`）会把全局类当本地类哈希，规则整条失效不报错——全局类一律 `:global(.mx-input)` 显式声明。自查：`grep -n '\.mx-' <file>.vue | grep -v ':global'`（仅 style module 段）。
- **模板 `$style` 引用必须与样式定义一一对应**：压缩/合并样式类后漏改模板引用，`$style.xxx` 解析 undefined 被 Vue 静默吞掉。自查：`comm -23 <(grep -oE '\$style\.[a-zA-Z0-9]+' f.vue | sed 's/\$style\.//' | sort -u) <(grep -oE '^\.[a-zA-Z0-9]+' f.vue | sed 's/^\.//' | sort -u)` 必须为空。
- **script 内取 CSS Modules 类必须 `useCssModule()`**：`$style` 只在模板作用域自动注入，`<script setup>` 函数里直接写是 undefined（类型不报）；动态拼类名先 `const styles = useCssModule()`。
- **scoped/全局桥接选择器整体包 `:global()`**：`:global(祖先) .后代` 会被编译器丢弃后代选择器（产物只作用于祖先）——需祖先+后代一起 `:global(祖先 .后代)`；改完 curl dev server 的 style 子模块核对产物，不猜。

## 验证（仓库内开发语境）

在 mindx-work 仓库内开发时每轮必做：`pnpm typecheck`（仓库根目录，vue-tsc 校验 SFC）+ Python Playwright 直连 dev 服务 5273 断言几何与主题（浏览器用 `launch(executable_path=...)` 指向本机已缓存版本，勿硬编码版本号）。结论必须有断言或截图证据，禁止猜测。market 在线插件（无仓库 dev 流程）的验收方式见 SKILL.md「开发流程·验证」。
