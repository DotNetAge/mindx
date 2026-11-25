---
updated: 2026-11-25
scope: sprite / gltf-clip / custom 三种渲染器
---

# 渲染器规格

三种渲染器对应 `buddy.json` 的 `renderer`。壳按此装载，组件不需自己实现渲染循环（`custom` 例外）。

## sprite（2D 序列帧）

- 资源：单张图集，PNG 或 WebP，透明底，正方形或矩形均可。
- 切格：`frames.cols` 列等分；行数 = `ceil(count / cols)`；帧顺序左上→右下。
- 帧内容向左下对齐（宠物脚底贴图集格底边），壳按 `viewport.anchor` 对齐窗口。
- 默认流程：`idle` 帧组占 `frames.count` 的前若干帧时，用 `clips` 的 `idle: "0-3"` 形式声明帧区间（字符串，闭区间，从 0 编号）；未声明时全部帧循环。
- 动作到帧区间的映射同样写在 `clips`：

```json
"clips": { "idle": "0-3", "play": "4-5", "question": "6", "complete": "7" }
```

- 播放：`idle` 常驻循环；瞬发动作播完区间返回 `idle`；`busy(on)` 切到 `busy` 区间循环（未声明则留在 `idle` 并加速 1.5 倍）。
- 禁止在图集里放文字、UI、水印；气泡由壳画。

## gltf-clip（3D 骨骼）

- 资源：单个 `.glb`，内嵌贴图；外部 `.bin` / 贴图文件允许但不推荐（增加丢文件风险）。
- 朝向：模型正面对 `+X` 时 `yaw` 取 0；壳默认给 -90° 到 3/4 侧视，可由 `yaw` 覆盖。
- 动画：必须以 `AnimationClip` 存在于 GLB 内，`clips` 映射语义名到 clip 名。
- 过渡：语义切换用 120 ms 交叉淡入淡出；`idle` 循环，瞬发 clip 播一次（`LoopOnce`）后回 `idle`。
- 归一化：壳按模型包围盒缩放到 `ctx.size`，并按脚底（包围盒 Y 最小处）对齐锚点；模型不必自带统一单位。
- 性能：保持单 scene、单 skinned mesh 优先；贴图 ≤1024、三角面 ≤60k，超限直接拒收。
- 光照：壳提供固定三点光与环境贴图，组件不要自带光源（会叠亮）。

## custom（自定义，含代码）

- 允许自带 `runtime.js`，但必须跑在独立沙箱页面：`nodeIntegration: false`、`contextIsolation: true`、无其它 IPC，仅暴露一条只读事件通道与一个 `reportViewport()`。
- 入口导出：`export function createBuddy(host) { return { mount, notify, ... } }`，`host` 提供 `{ container, assets, size, theme, onEvent }`。
- 沙箱页每秒最多上报 1 次视口变化；超出丢弃。
- 禁止：网络请求、文件系统访问、动态 `eval`、加载外部脚本、长时间占用主线程（单帧预算 8 ms）。

## 渲染库不锁定

壳内用哪个库画，由装载器决定，与组件契约无关：

- `gltf-clip` 当前装载器用 Three.js（`AnimationMixer` 播 clip + 三点光）。
- `sprite` 装载器用 2D Canvas 按帧率换帧。
- 换库（Babylon、自研 WebGL）只替换装载器，组件交付物与清单字段不变。

## 扩展：新增一种渲染器

新增 lottie / live2d / video / 逐帧序列这类形态时，按五步走，禁止改动七个动作方法与清单必填字段：

1. 定资源形态与清单字段（如 `renderer: "lottie"`、`model: "x.json"`），写进 `references/manifest.md` 字段表。
2. 壳侧写装载器：`mount` 建容器、载资源、接 `lookat` 的插值，`dispose` 释放。
3. 装载器必须复用同一套兜底反应、3 s 超时与错误隔离（见 `contract.md`），不得另立规则。
4. `scripts/verify_buddy.py` 加该 renderer 的分支：必填字段、资源存在、体积上限。
5. 清单枚举与设置页展示一并加值；`clips` 语义键不新增。

## 三种渲染器的选择判据

| 需求                       | 选择             |
| -------------------------- | ---------------- |
| 只要形象变化、动作靠帧     | sprite           |
| 有骨骼动画、要 3D 体积感   | gltf-clip        |
| 需物理/着色器/程序化表情   | custom（沙箱）   |
| 只是换颜色/贴图            | 改 sprite 图集即可，不要自建渲染器 |
