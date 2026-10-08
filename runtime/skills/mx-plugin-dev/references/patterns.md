# 插件实战模式（跨插件收敛的结构经验）

收录纪律：只收**结构性 / 架构级**经验——凡 contract.md（契约）、examples.md（骨架）、daemon.md（数据通道）、uikit.md（样式）、SKILL.md（流程军规）已覆盖的一律不重复；每条必须有真实插件证据（≥2 个插件独立收敛出同构做法，或一次实踩修复后固化）。写「决策规则 + 为什么」，不写流水账。新增经验先对照既有条目，能并入就不加条。

## 1. engine/ 纯逻辑分层（插件长大后）

**规则**：插件超过"单组件 + store"规模时，把 UI 无关的纯逻辑抽进 `engine/` 子目录——数据源、几何/布局计算、编码导出等，只依赖类型不依赖 Vue；组件层与 store 薄，只做装配与绑定。

**为什么**：纯逻辑可独立验证（不挂 UI 就能跑断言）；渲染与数据解耦后，改界面不碰算法、改算法不碰界面。

**证据**：video-editor（`engine/probe|render|mediaPool|exporter`）与 gitgraph（`engine/gitSource|graphLayout|fileTree`）互不参考，独立收敛出同一结构。

## 2. 预览与导出共用同一渲染函数

**规则**：凡是"能编辑、能输出"的插件（画布、视频、文档），预览画面与导出产物必须出自**同一个渲染函数**（输入 = 上下文 + 数据 + 时刻），禁止预览一套画法、导出另一套。

**为什么**：双实现必然漂移——预览对、导出错（或反之），且修一处漏一处；单一事实源让"所见即所得"由结构保证而非测试保证。

**证据**：video-editor `renderFrame` 同时驱动 PreviewPlayer 的 rAF 循环与 exporter 的离屏录制。

## 3. 长任务三件套：快照 + 互斥守卫 + 进度取消

**规则**：任何秒级以上的任务（导出、批量处理、批量安装）落地时同时做三件事——①任务启动时对输入数据取**快照**（渲染用快照不用活状态，编辑不干扰进行中的任务）；②一个布尔守卫（如 `exporting`）**覆盖全部修改性入口**（store action 早退 + 相关 UI 禁用），禁止"导出中还能编辑"；③暴露进度与取消通道，取消要能清干净（录音机、定时器、临时流）。

**为什么**：缺①任务结果漂移，缺②用户必踩，缺③表现为假死。

**证据**：video-editor 导出链（快照渲染 + exporting 守卫 + 进度回调/取消信号，实踩补齐）。

## 4. 状态上移判定：跨挂载存活即进 store

**规则**：界面状态放组件本地还是 store，判定标准只有一条——**该状态是否需要跨挂载存活**（切走 tab 再切回、modal 关了再开要还在）：要 → store；不要（输入框草稿、hover 态）→ 组件本地。拿不准时上移。

**为什么**：Detail/Content 条目是 v-if 条件渲染，组件卸载即失忆；画布、草稿、撤销栈丢一次就是用户数据丢失。

**证据**：svgboard 全量状态上移重构（切 tab 丢整块画布实踩修复）；demo 骨架的表单草稿留在组件本地（正确）。

## 5. store 装配纪律：markRaw 与深拷贝

**规则**：①class 实例（引擎、播放器、终端会话）进 store 必须 `markRaw` 包裹——响应式包装会破坏实例内部契约且类型报错；②对 store 里的 reactive 片段做深拷贝用 `JSON.parse(JSON.stringify(x))`，**禁用 structuredClone**（遇 proxy 必抛 DataCloneError，异常被事件层吞掉表现为"按钮点了没反应"）。

**为什么**：两条都是静默失败形态，typecheck 全绿，排查成本极高。

**证据**：video-editor 实踩修复（MediaPool markRaw；splitAtPlayhead 克隆崩溃）。

## 6. 查看器 / 解释器插件的完整装配套路

**规则**：做"打开某类文件"的插件（markdown / image-viewer / video-editor / svgboard 同族）按固定五步装配——①`services.provide('<name>.store', createService())` 延迟外壳；②store 内 `open(path)` 命令 action（写状态 + `Detail.show` 编排）；③`ctx.fileTypes.register(exts, '<name>.store')` 接管扩展名——**exts 是数组，一次注册一批**：图片查看器就该把全部图像后缀一把注册，不必逐个调用（重复注册抛错；返回的清理函数在停用时摘除）；④承载席位：**Detail 是主工作区**（工作面类加 `openMax` 满宽 / `preferredWidth` 理想宽，沉浸类用 Sheet，纯浏览才用 Content 页），**Sidebar 无链接**——不做导航行，入口就是文件类型路由；创建能力分两种挂法：无上下文的全局新建挂 Toolbar `trailing`（本类插件无 Content 条目，trailing 即全局层恒显，如常驻"新建文档"），属于已打开文档上下文的操作挂 DetailToolbar（`owner` 归属 Detail 条目，随条目激活才出现）；⑤`openMax`/`preferredWidth` 是声明式的，宽度仲裁归壳，禁止组件自算布局。

**为什么**：六插件（markdown/svgboard/image-viewer/video-viewer/docpreview/video-editor）收敛出的同构装配；照抄即可，不必再发明。

**证据**：上述插件真码 + contract.md §8.5 / §5；`fileTypes.ts` register 签名 `register(exts: readonly string[], serviceId)`（数组入参 + 重复注册抛错，壳实证）；DetailToolbar owner 见 notes「kanban-agent-driven-ui」（五查看器 addToolbar 带 owner 先例）。

## 7. 大列表：分页 + 按需加载 + 按键缓存

**规则**：潜在大集合（提交历史、目录树、文件清单）默认三件套——分页/触底加载（设总量上限）、子资源按需拉取（展开才取）、按主键缓存已取结果（二次展开零开销）。

**为什么**：一次性全量拉取在真实仓库 / 目录上必然卡死交互；缓存缺失则每次展开都转圈。

**证据**：gitgraph（200/页分页 + 提交文件树懒加载缓存）与 explorer（逐目录展开）。

## 8. 坐席选型决策：先问交互形态，再落区

**规则**：动工前先回答"这个插件的交互形态是什么"，按形态选坐席，禁止边做边挪——

- **文件打开类**（编辑器 / 查看器 / 解释器）：Detail 是主工作区，Sidebar 无链接；先注册文件后缀名再写界面（exts 数组一次注册一批，如图片查看器注册全部图像后缀）；带创建能力把"新建"挂 Toolbar trailing。装配套路见第 6 条。
- **小工具类**（小游戏、时钟、地图、速算）：优选 Floater 浮窗——多条目并存、无遮罩不阻塞、不占任何轨道；浮窗支持头部拖动 + 右下角手柄调节尺寸（min 240×240），注册声明的 `width`/`height` 只是初始值。**禁止用 Content 承载小工具**。
- **业务系统类**（订单、采购单等重流程处理）：唯一考虑占用 Content 的形态，前提是**完全不需要 Agent 交互**；且 Content 必须配 Sidebar 入口——壳的 Content 激活通道唯一（Sidebar 行点击，`activate` 为壳保留），无入口的 Content 页不可达；Detail 与 Toolbar 按需另配。
- **Sheet**：仅当插件要"脱离当前全部使用场景、相当于进入另一个系统"时才用——Sheet 全屏互斥单开、覆盖一切（含 modal 之下的全部工作区），门槛最高，普通查看/编辑够不着。

**辅助区按需组合，主工作区只选一个**：

- **弹层分界 = 是否中断当前工作流**：必须"回答才能继续"（删除确认、危险操作）→ modal（互斥、遮罩、阻塞）；只需"看到、可不理会"（完成/出错提醒）→ banner（可堆叠、不阻塞）；要持续操作且不中断工作流 → Floater。
- **配置去向**："设置一次就不想再看到"的偏好（阈值、开关、默认行为）→ Settings 配置行（壳自带通用页）或自定义配置页；工作流内的临时选项 → 留在插件页面内。
- **Content + Detail 并存时 `DetailEntry.owner` 必须指向 Content 条目 id**——否则 Detail 不随 Sidebar 行点击联动，成了孤立唤起层（全字段见 contract §5）。

**为什么**：坐席错位是插件观感差的最大来源——小工具占 Content 会把主工作区挤掉，重表单塞浮窗施展不开；先定形态再选区，一次到位。

**证据**：主清单为用户定稿的选型决策（2026-10-03），非跨插件归纳；其中两条硬约束来自壳实证——Content 激活唯一通道 = Sidebar 行（views.ts `activate` 壳唯一写注释）、Sheet 全屏互斥单开（views.ts SheetViewApi 定义）；Floater 调尺寸为配套壳能力（FloaterPane 右下角手柄）；弹层分界佐证 notes「shell-floater-window」（z-index 层级表 banner 65 < modal 70）；Content+Detail 联动为实装收敛（contract §5 owner 字段）。
