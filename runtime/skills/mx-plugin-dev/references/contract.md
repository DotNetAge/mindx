# mindx-work 八区契约（完整版）

本文件是 `ui-shell` 内核契约的完整说明，签名与实现一一对应（来源：`ui-shell/src/views.ts` / `createApp.ts` / `services.ts` / `changes.ts` / `registry.ts`）。SKILL.md 只留速查，用到哪个区的细节来这里查。

## 1. 定位与三条硬边界

本契约描述一个构建类 Mac 应用的 UI 框架的组件层：AppShell 的视图区构成、各视图区的条目模型、壳与插件之间的全部接口。界面由预置插件运行时组装而成。

1. **零业务词汇**——框架 API 不出现任何业务概念。换一个业务领域，框架代码零改动。
2. **渲染库无关**——内核对组件类型不透明（`component: unknown`），由薄适配器（ui-shell-vue）收窄类型并负责渲染。
3. **框架不做状态管理**——插件内部状态用所在生态惯用法（本项目定稿：Pinia）；跨插件只有 `services` 一条通道；框架自身只持有视图区条目与机制状态（壳唯一写）。

## 2. 通用条目模型

```ts
// 内核级通用条目，C 由适配器收窄为具体渲染类型（Vue 组件）
interface Entry<C> {
  id: string          // 区内唯一，注册冲突启动期抛错
  order?: number      // 缺省 100，小者在前
  component: C        // 对内核不透明，适配器负责解释与渲染
}

// 图标引用：Iconify 名称 "{collection}:{name}"，如 "lucide:settings"
type Glyph = string

// 壳传给条目组件的 UI 事实（按视图区选用，宁可少传；业务数据不走 props）
interface ViewProps {
  active?: boolean    // Content / Detail：当前是否呈现
  compact?: boolean   // Sidebar：折叠成 rail
}
```

**order 排序语义（十一席位通用，实证：`registry.ts` / `views.ts`）**：全部注册表（Sidebar 节 / Header / Footer、Content、Detail、Overlay、Sheet、Floater、Toolbar、Preferences 页 / 行）统一按 order 升序排列，缺省 100，小者在前；同 order 保持注册先后（`Array.prototype.sort` 稳定排序）。例外：Sidebar 节内的行（SidebarRow）无 order 字段，按给定数组顺序渲染。

**order 保留范围（硬约定，违反即错误实现）**：1–1000 为保留值——壳内置条目与预置插件（core）专用；market 在线（扩展）插件注册任何条目必须从 1001 起。该范围是**约定而非运行期校验**（registry 与动态加载器均不做区间拦截，违反不报错），但扩展条目侵入保留段会与未来预置条目抢位，禁止。

各视图区 `entries` 均为**只读快照**：registry 每次返回数组拷贝（`list.slice()`），外部无法改写内部数组，持有旧快照也不受 remove 影响（实证坑：曾返回内部数组本体，remove 换新数组后旧持有者永远读到旧数据且无任何报错——凡 getter 转发必须逐次调用 getter，禁止对象字面量固化 `entries: registry.entries`）。

## 3. Sidebar — 分节导航

行是**数据模型**而非组件，由壳统一渲染。侧栏分三段：Header 固定区（Logo、折叠按钮）、滚动区（节 + 行）、Footer 固定区——Header / Footer 经组件席位注册，不随内容滚动，折叠成 rail 时仍渲染（组件经 `compact` 自适配）。侧栏宽度由壳承载（右缘拖拽手柄调节、双击重置，折叠后禁用拖拽）。

```ts
interface SidebarViewApi<C> {
  add(entry: SidebarEntry<C>): void
  remove(id: string): void
  has(id: string): boolean
  readonly entries: readonly SidebarEntry<C>[]
  addHeader(entry: Entry<C>): void   // 顶部固定区席位
  addFooter(entry: Entry<C>): void   // 底部固定区席位
  readonly headers: readonly Entry<C>[]
  readonly footers: readonly Entry<C>[]
}

type SidebarEntry<C> = Entry<C> & {
  title?: string       // 分节标题，零文案：不传就不渲染节头
  icon?: Glyph         // 折叠成 rail 时的替身
  rows: SidebarRow[]   // 必填；缺 rows 或行缺 id/label 启动期抛错
}

interface SidebarRow<C> {
  id: string            // 必须对应一个 Content 条目 id，启动期校验
  label: string
  icon?: Glyph
  badge?: C             // 可选富元素：未读数、状态点
  variant?: 'button'    // 按钮卡形态（如"新会话"）：动作行不承载选中底
}
```

## 4. Content — 单活动视图

```ts
interface ContentViewApi<C> {
  add(entry: ContentEntry<C>): void   // title 呈现于 Toolbar
  remove(id: string): void            // 移除活动条目后 activeId 回落至 order 最小条目
  has(id: string): boolean
  readonly entries: readonly ContentEntry<C>[]
  readonly activeId: string | null    // 壳唯一写；未选择时 getter 动态回退 order 最小条目（装配期不受注册顺序影响），activate 后固定
  activate(id: string): void          // 仅供壳的 Sidebar 行点击调用；条目不存在抛错
}
```

## 5. Detail — tabs + 轨道

tab 归属解决多插件共存，轨道解决 Content 让位。

```ts
type DetailEntry<C> = Entry<C> & {
  title: string
  icon?: Glyph
  /** 层归属：指向 Content 条目 id——该插件激活时轨道联动切换到此层；
   * 省略 = 独立唤起层（仅经 Detail.show 唤起） */
  owner?: string
  /** 是否呈现壳固有默认工具组（全屏/隐藏/关闭三按钮）：缺省 true；
   * 插件自带 DetailToolbar 按钮组时可关 */
  defaultTools?: boolean
  /** 声明式满宽：该条目激活拉出轨道时宽度默认取当前上限（用户仍可拖窄） */
  openMax?: boolean
  /** 声明式理想宽（px）：激活时轨道取此值（clamp 到拖拽边界）；
   * 宽度仲裁优先级 openMax > preferredWidth > 壳缺省——"激活谁用谁的宽" */
  preferredWidth?: number
}

interface DetailViewApi<C> {
  add(entry: DetailEntry<C>): void
  remove(id: string): void
  has(id: string): boolean
  readonly entries: readonly DetailEntry<C>[]
  readonly shown: boolean
  readonly activeTabId: string | null
  show(id?: string): void             // 编程开合，缺省激活首个 tab；激活即隐式切换（无 Tab 组件）
  hide(): void
  setActiveTab(id: string): void      // 编程式激活条目的唯一通道；条目不存在抛错
  popActiveTab(): void                // 关闭当前层：退到激活历史上一不同条目，无则收起轨道
  addToolbar(entry: DetailToolbarEntry<C>): void  // 见下
  removeToolbar(id: string): void
  hasToolbar(id: string): boolean
  readonly toolbarEntries: readonly DetailToolbarEntry<C>[]
}

/** DetailToolbar 尾段按钮组：owner 必填且归属 Detail 条目——每条目一套，
 * 仅归属条目激活时呈现（先 Detail.add 条目再 addToolbar，归属不存在启动期抛错） */
type DetailToolbarEntry<C> = Entry<C> & { owner: string }
```

owner 归属语义（实证：video-editor / gitgraph 的保存导出与刷新按钮组）：按钮组按条目私有，激活谁显示谁的，条目 remove 时级联移除；`defaultTools: false` 供自带按钮组的条目关掉壳固有工具组。

## 6. Overlay — 全局浮层

只收全局层；popover 这类贴着触发点的局部浮层由插件组件自己渲染。

```ts
type OverlayKind = 'modal' | 'banner'

interface OverlayViewApi<C> {
  add(entry: Entry<C> & { kind: OverlayKind }): void
  remove(id: string): void   // 关闭 = 移除
  has(id: string): boolean
  readonly entries: readonly OverlayEntry<C>[]
}
```

- `kind` 非法值抛错。
- **modal 互斥**：已存在 modal 条目时再 add modal 直接抛错（不是静默覆盖）。编排惯例：重复打开同 id 时先 `has(id)` 守卫再 `remove`（`remove` 对不存在条目同样抛错，直接裸 remove 首开即炸）。
- banner 顶部可堆叠，用自增序列号区分 id。

## 6.1 Sheet — 全屏抽层

由下向上滑入覆盖整个界面的浮层级视图（抽屉语义），与 Overlay / Settings 同为壳级公共能力。头部是壳固有 chrome：注册者提供的 Title 居中呈现，尾端固有关闭钮（关闭 = 移除条目）；本体为注册者组件，填充头部以下全部空间。

```ts
interface SheetViewApi<C> {
  add(entry: Entry<C> & { title: string }): void  // title 由壳渲染于头部居中；缺失抛错
  remove(id: string): void   // 关闭 = 移除
  has(id: string): boolean
  readonly entries: readonly SheetEntry<C>[]
}
```

- **sheet 互斥**：同时至多一个，已有条目时再 add 直接抛错（对齐 modal 互斥）。
- 关闭通道三处：右上角固有关闭钮、Esc（modal 在场时 Esc 先关 modal）、注册者自行 `remove`。
- 条目 component 只写本体内容排版；头部（Title + 关闭钮）由壳容器提供，禁止自绘头部。

## 6.2 Floater — 可拖动浮窗

无阻塞浮动件：风格与主窗浮层族一致（elevated 底 / window 圆角 / shadow-lv3 锐利边界），**无传统 titlebar**——头部为细带，左侧模拟 macOS 交通灯（仅红灯，点击关闭，`--mx-traffic-light-close` token）+ 注册者标题小字；按住头部拖动（pointer capture），clamp 保证头部始终可抓取；右下角手柄可拖拽调节尺寸（最小 240×240），注册声明的 `width` / `height` 只是初始值，用户调整由壳本地持有。

```ts
interface FloaterViewApi<C> {
  add(entry: Entry<C> & { title: string; width?: number; height?: number; animation?: FloaterAnimation }): void
  remove(id: string): void   // 关闭 = 移除
  has(id: string): boolean
  readonly entries: readonly FloaterEntry<C>[]
}
```

- **允许多开**（不互斥）：每个条目独立成窗；点击任意浮窗将其置顶（同层 DOM 顺序）。
- 无遮罩不阻塞、不抢 Esc；关闭通道 = 红灯 / 注册者自行 `remove`。
- 条目缺 `title` 抛错（零文案壳）；`width` / `height` 可选（px，缺省 360×480 适配层本地常量），语义是**初始尺寸**——用户可经右下角手柄拖拽调节（min 240×240），插件不得假设浮窗尺寸恒定。
- `animation` 出现/消失动画预设（缺省 `'zoom'` 放大出现、缩小消失，对齐对话框 Apple 动画参数 scale 0.82 + 标准曲线）；`'fade'` 纯淡入淡出；`'none'` 无动画（瞬时出现/消失）；非法值抛错。
- 位置归适配层本地状态（刷新重置；持久化为契约开放点）。
- 条目 component 只写本体内容排版；头部（红灯 + 标题）由壳容器提供，禁止自绘头部。

## 7. Toolbar — 窗口工具栏

```ts
interface ToolbarViewApi<C> {
  add(entry: ToolbarEntry<C>): void   // slot 非法抛错
  remove(id: string): void
  has(id: string): boolean
  readonly entries: readonly ToolbarEntry<C>[]
}

type ToolbarEntry<C> = Entry<C> & {
  slot: 'leading' | 'trailing'
  /** 层归属：指向 Content 条目 id——仅该条目激活时呈现（对齐 DetailToolbar）；
   * 省略 = 全局层（恒显，如文件夹/浏览器/终端图标） */
  owner?: string
}
```

两席位全空时整个 Toolbar 不渲染。

## 8. Settings — Preferences

设置面板由壳自动生成，注册者只提供页与行。

```ts
interface PreferencesApi<C> {
  page(entry: PrefPageEntry<C>): void   // PrefPageEntry = Entry & { title: string; icon?: Glyph }
  row(entry: PrefRowEntry<C>): void     // PrefRowEntry = Entry & { page?: string }
  remove(id: string): void              // 先查页再查行；"通用"页不可移除（抛错）
  hasPage(id: string): boolean
  readonly pages: readonly PrefPageEntry<C>[]
  readonly rows: readonly PrefRowEntry<C>[]
  open(): void
  close(): void
  readonly isOpen: boolean
  readonly activePageId: string
  setActivePage(id: string): void       // 页不存在抛错
}
```

- 壳自带"通用"页（id 为 `general`），承接未指定 `page` 的行；该页不可移除，齿轮图标（`lucide:settings`）为其固有视觉。
- 行的 `page` 指向不存在的页 → 启动期校验抛错。

## 8.5 文件类型接管（fileTypes）

**「针对特定文件类型的查看器 / 解释器插件」的注册通道**：插件装配期声明自己接管的扩展名 → 打开服务名；路由方（explorer 文件树、chatflow file_open）按注册表分派，路由代码零改动。渲染无关，同 services 通道的插件间共享性质。

```ts
interface FileTypesContext {
  /** 注册接管扩展名（ext 归一小写；同一 ext 重复注册 = 编程错误，抛出）；
   * 返回摘除函数（插件停用清理时调用） */
  register(exts: readonly string[], serviceId: string): () => void
  /** 查扩展名对应的服务名（未接管返回 undefined） */
  serviceOf(ext: string): string | undefined
}
```

- **注册在插件内**（`ctx.fileTypes.register(['md', 'markdown'], 'markdown.store')`），不在壳配置——壳只提供通道；ext 不带点（`'svg'` 不是 `'.svg'`），归一小写。
- 未接管扩展名一律 codeeditor 兜底，无需注册「文本类」扩展名。
- market 插件装入即注册、停用即摘除（清理函数语义，军规 8）；重复注册装配期抛错。
- 实证消费方：markdown（md/markdown）、svgboard（svg）、image-viewer / video-viewer / docpreview（按后缀集合）、video-editor（vedit）、dashboard（dash）——服务名即插件 provide 的 store 服务，路由方解引用后调其打开 action。

## 9. AppShell 总面

```ts
interface AppShell<C> {
  readonly Sidebar: SidebarViewApi<C>
  readonly Content: ContentViewApi<C>
  readonly Detail:  DetailViewApi<C>
  readonly Overlay: OverlayViewApi<C>
  readonly Sheet:   SheetViewApi<C>
  readonly Floater: FloaterViewApi<C>
  readonly Toolbar: ToolbarViewApi<C>
  readonly Settings: PreferencesApi<C>
  readonly fileTypes: FileTypesContext  // 文件类型接管（§8.5）
  readonly services: ServiceContext
  readonly version: number            // 结构版本：任何条目 / 机制状态变更后递增
  subscribe(listener: () => void): Unsubscribe
  readonly sidebarCollapsed: boolean  // 壳机制：Sidebar 折叠（壳唯一写）
  toggleSidebar(): void
  dispose(): void                     // 逆序执行全部插件清理函数
}
```

## 10. 服务上下文（跨插件唯一共享通道）

```ts
interface ServiceContext {
  provide(name: string, impl: unknown): void  // 空名 / 同名重复提供抛错
  use<T>(name: string): T                     // 未提供时抛错——依赖缺失在启动期暴露
}
```

通道里流动的是 **Pinia store 等响应式本体**，不是快照副本：提供方放 store，消费方 `useService` 拿到的就是同一份响应式数据（Vue 适配器经 `useService(name)` 取用）。

**调用时机硬边界**：`useService` / `useShell` 基于 Vue inject，**只能在组件 setup 同步上下文调用**——事件回调深处、Pinia store 的 action、setTimeout/async 续体里调用会抛"壳上下文缺失"。

需要壳能力（服务本体、`Detail.show` 等）的 store，按创建时机二选一：

- **普通页面 store**（首次创建由组件 setup 触发）：在 store 初始化（setup store 函数体）时 `useService` / `useShell` 捕获一次，action 里用捕获的引用。
- **以服务 provide 的 store**：首次创建可能发生在非 setup 上下文（消费方 store 的 action 经 service 解引用触发），store 函数体内 inject 全部失效——走**装配期绑定**：插件函数体 `bindXxxShell(ctx)` 捕获壳本体，store 内 `theShell()` 取用；`shell.services.use()` 是内核方法、无 inject 依赖，拿到壳后随处可调（实证：markdown / explorer / image-viewer / web-viewer 四插件）。

**装配时序坑（实证）**：插件函数体在 `createApp([...])` 时执行，`createPinia()` 在 `mountVueApp` 内才安装——**插件函数体内调用 `useXxxStore()` 必炸（无 activePinia）**。装配期 provide 一律用**延迟外壳**：`createXxxService()` 返回 `{ get store() { return useXxxStore() } }`，把 store 创建推迟到消费方首次解引用（须在挂载后）。

### 10.1 跨插件响应的双形态（MVVM 译法，禁止事件总线）

跨插件联动只有两种合法形态，按语义选型：

| 形态            | 适用语义                                   | 实现                                                                                                                                        |
| --------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **A. 命令调用** | 消费方对提供方有明确意图（打开/聚焦/执行） | `useService('xx.store').store` 调 store 的 **action**（如 `open(path)`）；提供方在 action 内完成状态写入 + 壳编排（`shell.Detail.show` 等） |
| **B. 状态订阅** | 无方向的同步/多播（状态变了大家都动）      | 多方对**同一份 store** 各自 `computed` / `watch`；瞬时事件物化为状态迁移（布尔标志、append 记录、版本号）                                   |

判断口诀：事件总线广播"发生了什么"（无契约）；MVVM 表达"要做什么"（形态 A 的命令）与"现在是什么"（形态 B 的状态）。**不存在第三种**——真需要多对多通知时，是形态 B（同一份事实记录，各方 watch），不是互发消息。

### 10.2 职责规则

- store 归**提供方所有**（定义、类型、字段演进），消费方只调用 action 与读状态，禁止绕过 action 直改对方状态。
- 消费方 `import` 的是提供方的**类型与 store 标识**（或纯字符串 service 名），禁止 import 提供方组件/实现（插件间禁止互相 import 的军规不变）。
- 提供方涉及 Detail 展示时：启动期 `Detail.add` 注册自己的条目，命令 action 内 `Detail.show(id)` 编排打开——打开轨道是提供方的事，消费方只管调命令。

### 10.3 范式：点击文件链接 → 另一插件以 Markdown 显示到 Detail

```ts
// 提供方 md-viewer：store（Model）+ Detail 条目注册 + 命令 action。
// store 要在 action 里编排壳：装配期绑定壳本体（store action 里不能 useShell——inject 硬边界）。
let shellRef: VueAppShell | null = null
export const bindMdViewerShell = (shell: VueAppShell): void => { shellRef = shell }
const theShell = (): VueAppShell => {
  if (!shellRef) throw new Error('md-viewer 壳未绑定：插件装配缺失')
  return shellRef
}

export const useMdViewerStore = defineStore('md-viewer', () => {
  const currentFile = ref<FileInfo | null>(null)          // 形态 B 的状态本体
  const open = (target: FileInfo) => {                     // 形态 A 的命令
    currentFile.value = target
    theShell().Detail.show(DETAIL_ID)                      // 编排归提供方
  }
  return { currentFile, open }
})

// 服务外壳：装配期 Pinia 尚未安装（插件函数体先于 mountVueApp 执行），
// getter 把 store 创建推迟到消费方首次解引用（须在挂载后）
export type MdViewerStore = ReturnType<typeof useMdViewerStore>
export interface MdViewerService { readonly store: MdViewerStore }
export function createMdViewerService(): MdViewerService {
  return { get store() { return useMdViewerStore() } }
}

// 插件函数体内：
//   bindMdViewerShell(ctx)
//   ctx.services.provide('md-viewer.store', createMdViewerService())
//   ctx.Detail.add({ id: DETAIL_ID, component: MdViewerPanel, ... })
```

```vue
<!-- 消费方：点击 handler 只有一行命令，不关心对方怎么渲染 -->
<a @click.prevent="useService<MdViewerService>('md-viewer.store').store.open(file)">
  {{ file.name }}
</a>
```

渲染联动自动成立：`MdViewerPanel` 内 `computed(() => store.currentFile)` 响应命令写入，零回调、零事件。

### 10.4 内置壳服务（提供方 = app 装配层，类型定稿在 ui-shell/src/desktop-bridge.ts）

| 注册名                 | 本体                    | 用途                                                                                                                                                                                                                                                                            |
| ---------------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `shell.theme`          | `ThemeController`       | 主题三档（light/dark/auto）：`mode` / `setMode` / `subscribe`；机制归壳，档位持久化也归壳（设置行零感知）                                                                                                                                                                       |
| `shell.preferences`    | `PreferencesController` | **内置设置持久化**：`get(key, fallback)` / `set(key, value)` / `subscribe(key, cb)` / `ready`；落盘 `userData/preferences.json`（主进程原子写 + schemaVersion 代际 + 损坏容错），无宿主桥时降级仅内存。键用 `<owner>.<key>` 前缀防撞（如 `demo.font-size`）；键语义归消费方解释 |
| `shell.market-runtime` | `MarketRuntime`         | 在线插件启停控制                                                                                                                                                                                                                                                                |

消费范式（插件设置行持久化）：

```ts
const preferences = useService<PreferencesController>('shell.preferences')
const value = ref(preferences.get('demo.font-size', 14))
void preferences.ready.then(() => { value.value = preferences.get('demo.font-size', value.value) })
// 变更时 preferences.set('demo.font-size', next) —— 立即通知订阅者并异步落盘
```

## 11. 版本通知机制（联动的数据源）

内核是"可变结构 + 版本订阅"，由适配器映射为渲染层响应式：

```ts
interface ChangeHub {
  readonly version: number   // 任何视图区条目 / 壳状态变更后递增
  bump(): void               // version += 1；先复制订阅者再同步投递
  subscribe(listener: () => void): Unsubscribe
}
```

- 全部八区命令 API（add / remove / show / hide / activate / open / close / setActivePage / toggleSidebar…）改完注册表或机制状态后统一 `hub.bump()`。
- **这不是事件总线**：无载荷、无主题、无多播事件语义，只表达"数据变了，重读"。消费方拉模式按需重读，不存在事件时序耦合。
- Vue 适配器薄桥（`ui-shell-vue/src/reactivity.ts`）：
  - `useShell()`：取壳上下文（不在装配组件树内则抛错）；
  - `useShellVersion()`：订阅壳版本，返回响应式版本号；
  - `useShellData(read)`：`computed` 内先 `void version.value` 建立依赖再执行 `read()`——**凡读壳数据（entries / activeId / shown / isOpen / sidebarCollapsed 等）必须经它包裹**，否则界面不随壳变更更新。

## 12. 启动期校验（违反即抛错，装配失败）

| 校验               | 规则                                                |
| ------------------ | --------------------------------------------------- |
| 条目 id 唯一       | 同区重复注册同名 id 抛错（registry 层）             |
| Sidebar 行映射     | 行 id 必须能在 Content 条目中找到                   |
| Settings 行归属    | 行的 `page` 必须指向已注册的页                      |
| Sidebar 条目形状   | 缺 `rows`、行缺 `id` / `label` 抛错                 |
| Overlay modal 互斥 | 已有 modal 再 add modal 抛错                        |
| Sheet 形状与互斥   | 缺 `title` 抛错；已有 sheet 再 add 抛错             |
| Floater 形状       | 缺 `title` 抛错；`animation` 非法值抛错；多开不互斥 |
| Toolbar slot       | 非 `leading` / `trailing` 抛错                      |
| 文件类型接管       | 同一 ext 重复注册抛错（§8.5）                       |
| DetailToolbar 归属 | addToolbar 的 owner 条目不存在抛错（先 Detail.add） |
| 服务               | 空名 / 重复 provide 抛错；use 未提供服务抛错        |

同一组硬约束经 `validateShellConstraints`（createApp 导出）在**动态插件激活后复调**（§18.1 八区契约对动态插件完全适用）：插件注册完成即校验，违规视为激活失败——执行其清理函数回滚注册后抛错，进失败 banner。禁止只查启动期（动态注册会绕过）。

## 13. 编排语义（关键决策）

1. **导航联动归框架**：Sidebar 行 → Content 条目靠 id 映射，选中态壳唯一持有，插件零连线。
2. **Overlay 收窄**：全局层只有 modal（互斥）与 banner（可叠）；popover 归插件局部渲染。全屏抽层是独立视图区 Sheet（互斥单开，头部壳固有 chrome：居中 Title + 尾端关闭钮）。
3. **Detail 双语义合一**：tab 归属（多插件共存）+ 轨道（Content 让位），`show / hide` 是壳的编排 API。
4. **壳唯一写**：`activeId` / `shown` / `isOpen` / `sidebarCollapsed` 等机制状态的写入点只有壳的编排 API，插件只读（经 `useShellData`）或经命令 API 请求变更。

## 14. 视图区几何速查

壳 = 八区，由 AppFrame 组装（Floater 浮窗、Sheet 抽层与 Overlay / Settings 浮于各区之上）：

```text
┌─ Sidebar ─┬──────── Content ────────┬─ Detail ─┐
│ 248 / 56  │  （单活动视图，撑满）    │ 320 轨道  │
│  Header   │                         │  tabs    │
│  滚动节    │                         │  轨道体   │
│  Footer   │                         │ (可收起)  │
└───────────┴─────────────────────────┴──────────┘
  Overlay（modal 居中互斥 / banner 顶部可堆叠）、Sheet（全屏抽层）
  与 Settings（大面板）浮于其上
```

- **Sidebar**：展开宽 248 / 折叠 rail 80（容纳 macOS 红绿灯排；右缘拖拽手柄调宽、双击重置，折叠后禁用）；Header/Footer 固定区不随内容滚动；行高梯度：导航行 34 / 节头 36 / 新会话按钮卡 38 / footer 行 42。
- **Content**：单活动视图撑满容器，插件禁止自加 `max-width`。
- **Detail**：右侧轨道，默认 320px、可拖拽调节（240–520，左缘手柄、双击重置，对齐 Sidebar 手柄交互）；打开时自右缘滑入动画，收起时 Content 自动回弹。
- **Settings**：大面板宽 800、高 `min(800px, vh−48)`、圆角 32；左导航 188、页项高 40；右上 28px 圆形关闭钮。
- **Toolbar**：leading/trailing 两席位通栏，全空时整条不渲染。
- **Overlay**：modal 居中互斥、banner 顶部堆叠；卡片壳样式（背景/圆角/min-width/padding/阴影）由壳容器提供，条目组件只写内部排版。

浮层层级定稿（禁改禁猜，改动须同步本表；实证：modal 曾用 50 被设置面板 60 盖住，点击穿透到面板行区）：

| 层            | z-index | 语义                                                                                                                               |
| ------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| Content       | 5       | 基准内容层                                                                                                                         |
| 拖拽手柄      | 6       | Sidebar/Detail 边缘手柄，须高于 Content dragBand（实证：z-index 1 时顶部 48px 手柄条被 dragBand 盖住无法抓取）                     |
| banner        | 40      | 顶部通知，不阻塞交互                                                                                                               |
| Settings 面板 | 60      | 全屏遮罩大面板                                                                                                                     |
| Sheet 抽层    | 62      | 全屏抽层，盖住设置面板（设置行内唤起 sheet 的常态），低于 banner（sheet 内动作触发的通知必须可见）与 modal（sheet 内确认框的常态） |
| Floater 浮窗  | 55      | 无阻塞浮窗，任何浮层族之下（设置/抽层/通知/modal/menu 都盖得住），Content 基准层之上；同层叠放次序 = DOM 顺序（点击置顶）          |
| modal         | 70      | 互斥阻塞层，必须高于设置面板与抽层（面板/抽层内确认 modal 的常态）                                                                 |
| menu          | 100     | 下拉菜单，浮于一切常规层                                                                                                           |
| toast         | 1100    | 最高瞬时反馈                                                                                                                       |

**Esc 分层**（随层级表联动）：Esc 只关最上层——modal 存在时先关 modal（OverlayPane **捕获期**监听 + `stopPropagation`，使设置面板的 Esc 冒泡监听不触发）；无 modal 时 Esc 关 sheet（SheetPane 捕获期监听，`defaultPrevented` 守卫 + modal 在场守卫双保险）；再无 sheet 时 Esc 关设置面板。禁止两层同时响应同一 Esc（实证坑：modal 在前时按 Esc 会同时关掉下层设置面板）。

## 15. 样式与文案纪律

- 主题 token 一律 `--mx-*` 命名空间；亮暗切换归壳；插件组件禁止书写字面色值（细节读本技能 references/uikit.md）。
- 零文案壳：壳骨架不含任何文案，标题、按钮文字全部由注册者提供。
- 视图区几何（宽度、让位）由壳计算，条目组件不自算布局边界。

## 16. 契约变更规则

- 新增**可选**字段不破坏契约；删除或改语义需升版本。
- 硬约束不可协商：条目 id 区内唯一、Sidebar 行 id → Content 映射校验、modal / sheet 互斥、`--mx-*` token 纪律、零文案壳。

## 17. 数据校验边界（同进程信 TS，边界才校验）

**同进程内信任 TypeScript 静态类型**：类型化调用链（八区 API、services store、组件 props）不加运行时校验、不加回退分支、不为静态接口已保证的值写防御判断——重复校验是噪声，且掩盖真实边界。

**校验只发生在系统边界**（数据从不可信一侧进入之处）：

| 边界                  | 校验什么                                                             |
| --------------------- | -------------------------------------------------------------------- |
| 配置 / 用户输入       | 形状与取值范围；加载时校验并显式报错（§12 启动期校验即此原则的实例） |
| 持久化（文件 / 存储） | 读回时校验版本与形状；未知版本拒绝，不猜测兼容                       |
| 跨进程 / 网络         | 结构与来源校验（preload 桥、任何 IPC/远程数据）                      |
| 插件 service 边界     | `use(name)` 未提供即抛错（启动期暴露）；提供方的 TS 类型就是契约     |

违者的典型症状：对同一个值在两层都写 `if (!x)`；对静态接口承诺的值再跑一遍 `typeof`。

## 18. 动态插件（在线插件 / market）

八区契约对动态插件**完全适用**：入口签名、注册 API、样式军规、id 前缀、清理函数一视同仁。在线插件（market）与预置插件（core）的唯一差异是**可删除性**——机制零分叉。order 取值一条必须遵守：在线插件注册条目 order 一律 ≥ 1001（1–1000 为壳内置与预置插件保留段，§2）。

### 18.1 入口签名与 SDK 注入

```ts
// 动态插件入口（manifest.entry 指向的 ES Module 默认导出）：
export default function (app: AppShell<unknown>, mx: { h: typeof h; MxIcon: typeof MxIcon }): void | (() => void)
```

- 第一参与静态插件完全相同（AppShell 八区 + services）。
- 第二参注入渲染工具（`h` 与 `MxIcon`）：动态插件从 `mx-plugin://` 协议加载，**无 npm import 通道**（无法 import ui-shell-vue），渲染工具由加载器注入补偿。动态插件代码禁止 import 任何工程内模块。
- 卸载语义：market 插件返回的清理函数在启停（`setEnabled`）与卸载时由壳装配层调用。

### 18.2 包格式与 manifest

zip 包 = `manifest.json` + `entry`（.mjs）+ 静态资源（.json / .css / .svg / .png，白名单后缀）。

```jsonc
{
  "id": "com.example.demo",     // 反向域名小写、至少含一个点（身份，安装后不可变）
  "name": "示例插件",            // 展示名
  "version": "1.0.0",           // semver 三段，强制
  "description": "一句话说明",   // 必填非空白
  "author": "作者名",            // 必填
  "url": "https://…",           // 可选，网站
  "repo": "https://…",          // 可选，源码库
  "license": "MIT",             // 必填（SPDX 标识）
  "entry": "entry.mjs",
  "permissions": ["…"],         // 权限告知（给用户看，不构成授权）
  "mxApiVersion": "1"           // 必须等于 ui-shell 导出的 MX_API_VERSION
}
```

- manifest 语义校验唯一家 = `ui-shell` 导出的 `manifestIssues(value): string[]`（收集全部问题一次性返回）；安装与加载前都调用它。
- 主进程只做**机械校验**（id/version 字符格式、entry 存在、zip 内路径不越界、后缀白名单）防路径穿越——§17 边界分工：渲染侧管语义，主进程管机械，两层各自完整，不互代。

### 18.3 版本代际与安装目录

- **id 是身份，version 是不可变代际，activeVersion 是激活指针**。更新 = 落新代际目录 + 指针迁移；回滚 = 切回旧指针（旧代际原样保留）；卸载才删该 id 全部版本目录。
- 安装目录 `userData/plugins/<id>/<version>/`（两层缺一不可：一层装不下多代际并存）；状态文件 `installed.json`（`schemaVersion: 1`；`plugins: [{ id, name, enabled, activeVersion, versions: [{ version, sha256, installedAt }] }]`）。
- `installed.json` 是状态文件，受持久化代际纪律约束（未知 `schemaVersion` 拒绝，不猜测兼容）；版本目录属程序资源，同版本重装允许覆盖。
- 协议 URL `mx-plugin://<id>/<version>/<file>` 锁定具体代际，指针切换后重新拉取即加载新代际。
- **多代际的安装入口**：市场卡在"该插件还有未安装版本"时保持可装（文案"安装此版本"），全部版本已装才禁用——否则多代际永远只有安装日的那一个，切版 UI 无从呈现。确认安装前必须先 `runtime.deactivate(id)`：activate 对已激活插件幂等返回，直接装新代际再 activate 会因幂等跳过而继续跑旧代际（实证坑）。

### 18.4 加载通道：mx-plugin:// 特权协议

- Chromium 禁止 http/file origin 动态 import file:// 资源（实证）→ 定稿特权协议 `mx-plugin://<id>/<version>/<file>`（host = id，路径首段 = version，与安装目录同构）；dev 与产物统一走协议，无环境分叉。
- `protocol.registerSchemesAsPrivileged` **必须在 app.ready 之前调用**（模块顶层）——app.ready 后调用静默无效。
- 协议 handler 机械校验：id/version 格式 + resolve 后仍在安装根内 + 后缀白名单，再回读版本目录文件（`net.fetch(pathToFileURL(…))`）。

### 18.5 IPC 桥与渲染侧加载器

- preload 暴露 `window.mxDesktop.plugins`：`list` / `installFromFile` / `installFromUrl` / `uninstall` / `setEnabled` / `setActiveVersion`；主进程 handler 全部校验 `BrowserWindow.fromWebContents(event.sender)`。
- app 装配期 `loadMarketPlugins(app)`（mount 前异步执行）：逐个 enabled 插件 fetch manifest → `manifestIssues` 校验 → 目录身份一致性（`manifest.id === view.id`）→ `import(/* @vite-ignore */ mx-plugin://…)` → `mod.default(app, { h, MxIcon })` → **`validateShellConstraints` 激活期复调**（违规 = 清理回滚 + 激活失败，§12）。
- **动态 import 同一 specifier 只求值一次**（ES module 语义）：停用→再启用、切版往返、卸载重装都会命中模块缓存，入口函数不再执行、注册不发生（实证坑）。每次激活必须给 import URL 加唯一 query（如 `?activate=<递增序号>`）穿透缓存；协议 handler 按 `new URL().pathname` 解析，query 不参与路径匹配，安全。
- **运行期启停与 Content 活动指针**：停用移除活动条目时 Content 回退到 order 最小条目；重新激活后条目恢复注册但活动指针**不自动跳转**（壳契约：activeId 由 Sidebar 行点击唯一写）。market 插件启停不得自动切页——用户在设置面板内操作，背景页跳转是惊扰；页面恢复由用户再点行完成。
- 失败隔离：单个插件激活失败收集为 `MarketLoadFailure`，装配完成后以 banner 呈现（Overlay.add 纯内容组件），**不挡首帧**。
- 纯 Web 环境（`window.mxDesktop` 不存在）加载器直接返回空——market 机制是 Electron 宿主能力。

### 18.6 市场源（静态托管）与信任模型

- 市场服务器形态 = **静态托管**：`index.json`（多版本数组 `versions: […]`）+ zip 包，主进程 `market:list` 拉取、`market:install` 下载 zip 走同一安装管线（sha256 + 代际落盘 + 指针迁移）。放弃了动态注册中心：无服务端状态、可 CDN 化、离线可缓存。
- 信任模型 = **显式安装即信任**（对齐 DSH"面板手势免审批"分级）：安装确认 UI 必须完整呈现 author / url / repo / license / permissions / versions，用户确认后落盘；安装时 sha256 校验完整性。放弃了 node:vm 沙箱隔离：桌面端安装即本机手势，与 core 插件同权。

### 18.7 mw CLI — Agent 安装回路（自扩展闭环）

- 动机：把插件能力**传导给 Agent**——Agent 在终端调 `mw plugin …` 安装/管理插件，app 即时感知激活，Agent「自己扩展自己」。CLI 与 UI 是同一安装管线的两个入口（electron/src/plugin-store.ts 纯 Node 工厂 + cli.ts / plugins.ts 两个薄壳）。
- 运行形态：`ELECTRON_RUN_AS_NODE=1 <Electron二进制> <dist/cli.js> …`（node-pty 同机制）；`mw install-cli` 把 wrapper 写进 **`~/.mindx/bin`**（mindx 同款路径——其安装器已在 shell rc 导入该目录，规避 /usr/local/bin 系统权限；rc 全无导入行时 CLI 补写 .zshrc 并提示 source）。存储根按平台 userData 约定推导（productName「MindX Work」，勿改），`MW_PLUGIN_ROOT` env 可覆盖（多实例/验收）。
- Agent 工作回路（写完插件包后）：

```bash
mw plugin install ./dist/my-plugin.zip   # 本地包直装；市场 URL 同样支持（SSRF 防线 + sha256）
mw plugin list --json                    # 机器可读回执：{ ok, data }；人读输出面向终端
mw plugin use <id> <version>             # 切版回滚；enable / disable / uninstall / export 同套
```

- 即时生效链：CLI 原子写 installed.json → 主进程 watch 安装根目录（防抖 300ms；**watch 目录而非文件**——原子替换走 rename，文件句柄失效）→ 广播 `plugins:changed` → market store 差分激活（新装且启用 → activate；停用 → deactivate；切版 → 先停再激活；首刷只建快照防误激活）。
- Agent 写包验收注意：manifest 全量校验仍由渲染侧 manifestIssues 执行（§18.5）——CLI 安装只过机械校验，包合法但契约违规会在激活期失败并回滚；用 `mw plugin list` 确认落盘状态 + 设置 → 插件页确认激活状态。

### 18.8 真机验收链路（market 机制专用，与普通插件 dev 5273 链路并行）

electron 走 `MX_PROD_DIST` 加载 **app 构建产物**而非 dev server——改了渲染侧源码必须重新构建再重启，否则验收的是旧产物（实踩）。四步循环：

```bash
# 1. 渲染侧代码改动后：重建 app 产物（electron 不认源码）
cd mindx-work/app && pnpm build

# 2. 终止旧实例（pkill 模式匹配不清 Helper 进程树，grep 确认后按 PID kill -9 兜底）
pkill -f "remote-debugging-port=9223"; sleep 2
ps aux | grep "[9]223" | awk '{print $2}' | xargs kill -9 2>/dev/null

# 3. 启动（市场源与产物目录均为 env 注入；默认域是占位符，本地验收必须覆盖）
cd mindx-work/electron && \
  MX_MARKET_INDEX_URL="http://127.0.0.1:8899/index.json" \
  MX_PROD_DIST="/<仓库绝对路径>/mindx-work/app/dist" \
  nohup npx electron . --remote-debugging-port=9223 > /tmp/<验收目录>/electron.log 2>&1 &

# 4. Python Playwright 连 CDP 断言（勿硬编码机器路径与端口之外的任何本机值）
python3 -c "from playwright.sync_api import sync_playwright"  # 环境自检
```

- 市场源 = 本地静态服务器：`python3 -m http.server 8899`（托管 index.json + zip；测试包生成脚本放 /tmp 验收目录，不入仓库）。
- 验收脚本统一模式：`connect_over_cdp('http://127.0.0.1:9223')` → 取非 devtools 页 → `pageerror`/`console error` 全程收集 → 末尾断言"过滤 mx-plugin:// 后错误为 0"。判据军规读本技能 SKILL.md 验证节。
