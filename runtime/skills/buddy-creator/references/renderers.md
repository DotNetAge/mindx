---
updated: 2026-11-26
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

- 资源：单个 `.glb`，贴图内嵌。外部 `.bin` / 贴图文件允许但不推荐（增加丢文件风险）。
- 必填：`model` 与 `clips.idle`。缺 `clips.idle` 时壳直接禁用该包，不猜「第一个 clip」。
- 语义映射：`clips` 的键是语义名，值是 GLB 内 `AnimationClip.name`。装载器做大小写与 `_`、`-`、空格归一化匹配，但仍应逐字一致，拼错只能靠 `verify_buddy.py` 发现。除 `idle` 外每个键都可缺，缺哪个就少哪种反应——壳不兜底。
- 朝向：`yaw` 是模型自身的偏航修正，默认 0。**每个模型的初始朝向都不一样，必须实测**：同一批 Tripo 导出的四只宠，正面对镜头分别落在 `yaw` 0.7 / 4.65 / 4.7 量级，猜不出来。做法是扫 0 / π/2 / π / 3π/2 四个角度各取一帧，挑正脸那个再微调。
- 尺寸：`heightTo`（默认 1）是等比缩放后的总高，`groundY`（默认 0.5）是模型底边落点。按包围盒缩放并水平居中，模型不必自带统一单位。
- 根位移：装载时剥掉 clip 里**所有** `.position` 轨道，只保留旋转与缩放。walk / run / cheer 普遍把位移烘在关节 `position` 上，固定机位下会整只顶到镜头前或跑出画面；代价是依赖关节位移的动作（蹲伏、跳跃升程）会失真，作者侧应优先选原地动作。
- 播放：`idle` 常驻循环；`busy(on)` 切 `busy` 循环、关闭回 `idle`；`question` / `play` / `jump` 用 `LoopOnce` 播一次后回基础动作。
- 过渡：220 ms 交叉淡入淡出。
- 一次性动作截断：按 `durations`（毫秒）截断，缺省 play 3 s / question 2.5 s / jump 2 s。作者常交出 12 秒的 cheer / dance，不截断会在 107 px 窗口里播半分钟。
- idle 速率：`idleSpeed` 默认 1；设 0 冻结在 clip 首帧（完全静止），0.1–0.3 为慢放。素材里只有舞蹈 / 跑步、没有真正的 idle 时，把它塞进 `clips.idle` 全速播会让宠物一启动就在跳舞——用慢放或冻结，别硬凑。
- 视线跟随：模型里存在名字含 `head` 的骨骼时，`lookat` 叠在该骨骼上（每帧 mixer 更新后追加一次，不累积）；没有 head 骨骼则不做跟随。
- 命中：整身 raycast，命中点高于包围盒七成高度算摸头。
- 性能：单 scene、单 skinned mesh 优先，贴图 ≤1024；三角面与体积走 SKILL.md 的准入线（350k / 10 MB）。60k 只是 107 px 下的经验值，不是硬线，不要当校验条件。
- 光照：壳提供固定三点光与地面接影，组件不要自带光源（会叠亮）。

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
