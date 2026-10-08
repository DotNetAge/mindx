# 插件场景用法（scaffold 用法说明与引用）

Agent 生成插件的高频路径：**选场景 → scaffold 生成 → 按需修改**，不手写样板代码。本文是各场景生成器的用法说明；每个场景的代码范式都来自 demo 预置插件与壳实现的真实代码（证据位置随文标注），API 完整签名读 [contract.md](./contract.md)。

## 生成器用法

```bash
# 在 mindx-work 仓库根目录执行
python3 .agents/skills/mx-plugin-dev/scripts/scaffold_plugin.py my-plugin --with detail settings
```

- 场景可任意叠加（空格分隔）；不带 `--with` 即 basic。
- 每次生成一个独立插件模块（`plugins/src/<name>/`），并自动完成三处注册：`plugins/package.json` exports 子路径、`plugins/src/index.ts` re-export、`app/src/main.ts` 装配清单。锚点不匹配时跳过并提示手工步骤，不破坏既有文件；目标目录已存在则拒绝覆盖。
- 生成后必做：`pnpm typecheck` + 起 dev 断言渲染（见 SKILL.md 验证节）。

场景总览：

| 场景            | 生成内容                                     | 典型问题                   |
| --------------- | -------------------------------------------- | -------------------------- |
| `basic`（缺省） | 页面 + 侧栏节 + Pinia store                  | 给界面加个新页面           |
| `detail`        | + Detail tab，页面挂载时自动打开             | 点侧栏菜单，内容进详情轨道 |
| `settings`      | + 配置项（store）+ 通用配置行 + 自定义配置页 | 插件要有设置项、可读可写   |
| `overlay`       | + modal / banner 浮层，页面按钮触发          | 弹全局对话框、顶部通知     |
| `services`      | + store 以服务形式 provide                   | 跨插件共享数据             |

## basic——页面 + 侧栏 + 状态

生成 `index.ts`（Content.add + Sidebar.add）、`store.ts`（Pinia setup store）、`pages/HomePage.vue`。

关键契约（证据：`plugins/src/demo/index.ts`、`ui-shell/src/createApp.ts` 的 `validateOrThrow`）：

- **行 id 与 Content 条目 id 一一对应**，对不上装配直接抛错。
- 条目 id（`<name>-home` 等）带插件前缀——条目 id 区内唯一，两插件撞 id 启动期抛错。
- Sidebar 行是数据模型由壳渲染，行点击切页归壳（`Content.activate` 壳唯一写），插件零连线。

## detail——点侧栏菜单，内容显示到 Detail 并自动打开

在 basic 上追加：`Detail.add` 注册 tab、`DetailPanel.vue`、页面 `onMounted` 调 `shell.Detail.show(id)`。

机制（有渲染层代码背书）：Content 条目是 `v-if` 条件渲染——`ContentPane.vue` 里 `<component :is="active.component" v-if="active" :active="true" />`，即**点击 Sidebar 行 = 挂载目标页**；页面挂载时机调 `show` 即"自动打开"（`show` 调用实证：`OverviewPage.vue`；tab 渲染与 `shown=false` 收起实证：`DetailPane.vue`）。

定制点：改 `Detail.show` 的调用时机（如改成按钮触发）与 `DetailPanel` 内容；`show(id?)` 缺省激活首个 tab，id 不存在抛错。

## settings——配置项：通用行 + 自定义配置页 + 读写

在 basic 上追加：store 增加可写配置项（`limit` / `volume`）、`prefs/GeneralRow.vue`、`prefs/PageRow.vue`、两种注册。

- **通用配置行**：`ctx.Settings.row({ id, component })` 不传 `page`，归入壳自带"通用"页（views.ts：缺省归 `GENERAL_PAGE_ID`，该页不可移除）。
- **自定义配置页**：`ctx.Settings.page({ id, title, icon, component })` + `ctx.Settings.row({ id, page: '<name>-prefs', component })`；`page` 指向未注册的页启动期抛错。
- **读取配置**：配置本体就是 store 的可写状态，行组件 `v-model.number="store.limit"` 直读直写（Pinia）。壳不提供配置存储 API（契约：框架不做状态管理；实证：theme 控制器为内存态、无持久化）。其他组件读同一份 store 即响应生效；**跨插件读走 services**（见下）。
- 设置行排版用 `.mx-pref-row` 等原语（读 [uikit.md](./uikit.md)）。

## overlay——浮层：modal 互斥 + banner 堆叠

在 basic 上追加：`overlays/ConfirmModal.vue`、`overlays/NoticeBanner.vue`、页面按钮与触发函数。

实证范式（`OverviewPage.vue`、`DemoModal.vue`、views.ts 校验）：

- **modal 同一时刻至多一个**：重复 add 抛错；编排惯例同 id 先 `has`→`remove` 再 `add`。modal 组件自己关闭（`has` 守卫后 `remove`，关闭 = 移除条目）。
- **banner 可堆叠**：自增序列号（或时间戳）区分 id。
- **条目组件不收 props、banner 关闭钮由壳容器提供**（`OverlayPane.vue` 渲染 `<component :is="banner.component" />` 不传 props，关闭钮在壳的 bannerCard 里调 `remove(banner.id)`）——条目组件是纯内容组件，**不要自绘关闭钮或卡片壳样式**（modal 卡片壳样式同样由壳的 modalCard 提供）。
- 边界：`kind` 只能 `'modal' | 'banner'`；popover / sheet 贴触发点的局部浮层由插件组件自己渲染，不进 Overlay。

## services——跨插件共享与响应

在 basic 上追加：`bind<Name>Shell(ctx)` + `ctx.services.provide('<name>.store', create<Name>Service())`（延迟外壳三件套由 scaffold 生成，通道名用 `<name>.store`）。

- **装配时序坑（实证）**：插件函数体在 `createApp([...])` 时执行，`createPinia()` 在 `mountVueApp` 内才安装——插件函数体内 `use<Name>Store()` 必炸（无 activePinia）。因此装配期 provide 一律走**延迟外壳**：`create<Name>Service()` 返回 `{ get store() { return use<Name>Store() } }`，store 创建推迟到消费方首次解引用（须在挂载后）。禁止在插件函数体、provide 调用里直接创建 store。
- **service store 内取壳**：以服务提供的 store 首次创建可能发生在非 setup 上下文（消费方 store 的 action 经 service 解引用触发），此时 store 函数体内 `useService` / `useShell` 会抛"壳上下文缺失"——走 bind/theShell：插件函数体 `bind<Name>Shell(ctx)` 捕获壳本体，store 内 `theShell()` 取用；`shell.services.use()` 是内核方法、无 inject 依赖，拿到壳后随处可调（实证：markdown / explorer / image-viewer / web-viewer 四插件）。
- **添加**两个可行时机：插件函数体内（`ctx` 就是 AppShell）或装配方 `app/src/main.ts`（实证：`shell.services.provide('shell.theme', createThemeController())`，在 `mountVueApp` 之前）。重复 provide 抛错。
- **消费**：组件内 `useService<T>('<name>.store').store`（实证：`ThemeRow.vue` 消费 `shell.theme`）。通道里流动的是 **Pinia store 响应式本体**，不是快照副本——消费端读到的就是提供方那份数据，改它全局生效。
- 语义（`services.ts` 实证）：`use` 未提供时抛错——依赖缺失在启动期暴露；时序上 provide 都发生在组件渲染之前。
- **响应（联动）**：在共享之上加跨插件动作时只有两种形态——命令调用（调对方 store 的 action）与状态订阅（各方 watch 同一份状态）；瞬时事件物化为状态迁移。选型规则、职责边界与完整范式（"点击文件链接 → 另一插件 Markdown 显示到 Detail"）读 [contract.md](./contract.md) §10.1–10.3，此处不重复。

## market——在线插件管理（手写范式，非 scaffold 场景）

market 是"动态插件机制"的消费者插件：管理已装清单（启停/切版/卸载）与市场浏览安装，机制本体读 [contract.md](./contract.md) §18。它的实现是三条实证范式的活样本（证据：`plugins/src/market/`）：

- **store 初始化捕获 services**：`useService(MARKET_RUNTIME_SERVICE)` 写在 store 函数体顶部（首次 `useMarketStore` 由组件 setup 触发，inject 有效）；action 与异步任务里直接 useService 抛"壳上下文缺失"（§10 时机硬边界的实踩修复）。
- **modal 互斥编排**：打开确认框先 `shell.Overlay.has(id)` 守卫再 `remove`，裸 remove 首开即炸（§6 惯例）。
- **宿主桥缺失显式报错**：`window.mxDesktop?.plugins` 不存在时动作抛中文错误提示，不做静默降级（§17 环境边界）。

## 手写插件依赖事项（不走 scaffold 时必查）

scaffold 自动完成三处注册，手写插件模块需自行核对，两处实踩：

- **`plugins/package.json`**：`exports` 加子路径 + `dependencies` 加 `"@mindx-work/ui-shell": "workspace:*"`（缺依赖 = TS2307 找不到模块；改动后 `pnpm install --no-frozen-lockfile` 刷新 lockfile）。
- **ui-shell-vue 全部具名导出**：`import { MxIcon, useShell } from '@mindx-work/ui-shell-vue'`——MxIcon 无 default 导出（TS2613 实踩）。

## 生成之后

骨架只含范式代码，业务逻辑按需填充；每处改动仍受 SKILL.md"插件义务与禁止"约束（行 id 对应、token 样式、CSS Modules、中文注释、清理函数、禁事件总线）。分场景之外的写法问题先查 [contract.md](./contract.md) 的对应契约，禁止凭印象造 API。
