---
name: buddy-creator
description: 开发 MindX 桌面宠物（Buddy）组件。当用户要「做一个宠物、换个宠物形象、做 2D 宠物、开发 Buddy 组件、校验宠物包、把宠物放上架」时使用；提到 Buddy 接口、宠物组件 时主动使用。
metadata:
  name_zh: 宠物组件开发
  version: 1.0.0
  author: Ray
---

# 宠物组件开发（Buddy）

把桌面宠物做成可替换组件：一个文件夹 + 一份清单 + 七个动作方法。壳（MindX）只负责挂载、转发事件、画气泡与窗口，组件只负责形象与动作。

## 何时使用

- 用户要新做一只宠物、换掉现在的牛、做 2D / 序列帧 / Lottie 形态
- 用户要校验一个宠物目录能不能被壳正确加载
- 用户要把宠物打包分发或上架
- 用户提到 Buddy 接口、`buddy.json`、`~/.mindx/buddies`

## 接口（契约全文，无映射表）

组件实现一个对象，挂载到壳提供的容器。七个动作方法全部可选实现，缺省由壳给通用兜底；`mount` 与 `dispose` 必做。

```ts
type NotifyKind = 'complete' | 'failed' | 'notice' | 'update'

interface BuddyContext {
  assets: string // 组件资源目录 URL 基址，拼接资源一律相对此值
  theme: 'light' | 'dark'
  size: number // 宠物区边长（逻辑 px），按此布局，禁止写死其它尺寸
}

interface Buddy {
  mount(container: HTMLElement, ctx: BuddyContext): Promise<void>
  notify(kind: NotifyKind): void // 收到通知：完成 / 失败 / 一般通知 / 更新就绪
  question(): void // 表示疑惑
  play(): void // 被摸头
  jump(): void // 高兴，跳一次
  busy(on: boolean): void // 忙碌开始 / 结束
  drag(on: boolean): void // 被拖起 / 放下
  lookat(x: number, y: number): void // 视线跟随，x/y 归一化 -1..1（-1=左/上）
  dispose(): void
}
```

性质决定调度方式，写成方法时按此实现：

| 方法                    | 时间性质 | 触发时机                       | 缺省兜底（组件不写时壳做）    |
| ----------------------- | -------- | ------------------------------ | ----------------------------- |
| `notify(kind)`          | 瞬发     | 工作台事件到达，演一次         | 整体上下点头 8px              |
| `question()`            | 瞬发     | ask_user / 授权阻塞            | 整体轻微歪头 6°               |
| `play()`                | 瞬发     | 点中宠物头 / 肚皮              | 整体压扁回弹一次              |
| `jump()`                | 瞬发     | 任务完成（高兴）               | 整体上跳 12px 回落            |
| `busy(on)`              | 持续     | 会话执行开始/结束              | 整体摆幅减半 + 下降 2px       |
| `drag(on)`              | 持续     | 拖拽开始/结束                  | 跟手偏移，松手回弹            |
| `lookat(x, y)`          | 每帧输入 | 光标移动（壳约 50ms 送一次）   | 整体视差位移 ±6px，纯平移     |

瞬发方法可被连续调用：壳保证同一时刻只播一个瞬发动作，新动作排队，队列上限 3，超出丢最旧。组件内部不必自己去重。

## 清单（buddy.json）

放在组件根目录，与资源同层。五个必填。

```json
{
  "id": "cow",
  "name": "小牛",
  "version": "1.0.0",
  "renderer": "gltf-clip",
  "model": "cow.glb",
  "clips": { "idle": "Idle", "busy": "Typing", "question": "Confused" },
  "viewport": { "width": 107, "height": 107, "anchor": "bottom-center" },
  "sound": "squeak.wav"
}
```

| 字段       | 必填 | 说明                                                              |
| ---------- | ---- | ----------------------------------------------------------------- |
| `id`       | 是   | kebab-case，与目录名一致                                          |
| `name`     | 是   | 展示名，中文可                                             |
| `version`  | 是   | 语义化版本                                                        |
| `renderer` | 是   | `gltf-clip` / `sprite` / `lottie` / `video` / `procedural` / `custom` |
| `model`    | 是   | 入口资源文件名（gltf-clip 为 .glb，sprite 为图集 .png/.webp）     |
| `clips`    | 否   | gltf-clip 专用：语义名→clip 名映射，缺 `idle` 视为不完整组件      |
| `viewport` | 否   | 逻辑尺寸与锚点；缺省由壳取资源边界推                              |
| `sound`    | 否   | 气泡音效，缺省用内置音                                            |
| `frames`   | 否   | sprite 专用：`{ "count": 8, "cols": 4, "fps": 10 }`               |
| `rig`      | 否   | procedural 必填：同目录 `rig.json` 文件名                          |

`rig.json` 里 `parts[]` 默认**逐网格建 pivot**（左右耳、双眼、双手各自独立侧向与相位）；多段组成的单体部件（如分节尾巴）加 `"per": "group"` 让整组共用 pivot。`jump` 段可选，缺省则该宠物不做弹跳。

组件目录内的资源用相对路径引用，壳把目录挂到 `mx-buddy://<id>/`，组件只认 `ctx.assets`。

## 部署与加载（与插件同构）

磁盘布局、版本清单、协议、变更通知全部照 `mx-plugin` 的做法，只换根与协议名：

| 项       | 插件                                  | Buddy                                  |
| -------- | ------------------------------------- | -------------------------------------- |
| 根       | `~/.mindx/plugins`                    | 用户根 `~/.mindx/buddies`（可写）+ 随包根（只读） |
| 布局     | `<id>/<version>/…`                    | `<id>/<version>/…`                     |
| 清单     | `installed.json`                      | `installed.json`（同结构，数组键 `buddies`） |
| 包       | `.mindpkg`（zip，内含 `manifest.json`） | `.mindpkg`（zip，内含 `buddy.json`）     |
| 协议     | `mx-plugin://<id>/<version>/<file>`    | `mx-buddy://<id>/<version>/<file>`      |
| 变更     | 原子替换清单 + 目录 watch 300 ms 广播  | 同                                      |

`installed.json` 条目字段：`id / name / enabled / activeVersion / versions[{version, sha256, installedAt}] / origin`。作者不写这个文件，由安装器生成。

**双根与默认包**：壳扫两份清单——随包只读根（随安装包发布的 `Resources/buddies/`）与用户根 `~/.mindx/buddies`。同 id 同版本以用户根为准；默认 `activeId` 由随包清单给出。壳代码里**只能出现 `activeId` 这个值，不得出现任何具体宠物的 id 判断**，装载一律按 `renderer` 分派。默认包若被用户卸载，回退到随包根里的同名条目。

打包（作者侧）：目录内直接是 `buddy.json` 与资源，禁止套一层同名文件夹；打成 zip 后改后缀 `.mindpkg`：

```bash
cd my-pet && zip -r ../my-pet.mindpkg buddy.json atlas.webp sound.wav
```

安装（壳侧）：解包到 `<id>/<version>/`，校验 `buddy.json` 的 id/version 与路径，算 sha256 写清单，原子替换并广播。

加载（壳侧）：取 `enabled && activeVersion` 的条目，经 `mx-buddy://<id>/<version>/buddy.json` 读清单，按 `renderer` 装载；失败或 3 s 超时回退到清单里的默认 `activeId`，不改写用户选择。

本地试装不必打包，把目录放进 `~/.mindx/buddies/<id>/<version>/` 并手写一条 `installed.json` 即可，壳的 watch 会接住。

## 端到端：一个模型文件 → 可安装包

作者只提供模型，Agent 按四步产出，中间不需要人：

1. **判分支出骨架**：

```bash
python3 scripts/gen_buddy.py --model pet.glb --id pet --name 名字 --out ./pet
```

有 clip 自动走 `gltf-clip` 并给出 `clips` 映射草稿；无 clip 走 `procedural` 并生成 **完整 `mindx.buddy.rig/1` 骨架**（动作参数取 `scripts/rig-template.json`，部件判据按本模型包围盒换算成模型局部坐标）。生成的判据是初稿：模型朝向（`orientation.facingYaw` / `viewYaw`）、头补片（`skullCap`）、心心特效（`fx`）默认关闭/按参考实现，必须按报告与实况复核后再交付。

2. **复核分件**：读报告里的 `meshes` 与 `groups`，逐组确认头/耳/眼/前肢/尾。置信度 `low` 的组必须人工核对，不能直接交付。
3. **校验**：`python3 scripts/verify_buddy.py ./pet`。退出码 1 的硬错误必须清零，退出码 2 的警告逐条判断。
4. **打包**：`python3 scripts/pack_buddy.py ./pet --out dist/pet-0.1.0.mindpkg`，产出 sha256 交给安装器。

装到本机见路由 5，分发见路由 6。

## 决策路由

### 路由 1：先定实现分支，再动手

按"作者手上有什么、会不会写代码"四问定分支，细则见 `references/branches.md`：

| 作者情况 | 分支 | 判定 |
| -------- | ---- | ---- |
| 有带动画的 GLB | `gltf-clip` | 零代码，普通用户可独立完成 |
| 会画画或要夸张形象 | `sprite` | 零代码，成本最低，最小验证路径 |
| 有 AE 动效 | `lottie` | 零代码，需打 marker |
| 只有现成视频 | `video` | 零代码，互动性弱 |
| 只有静态模型，要求"像牛一样" | `procedural` | 交模型 + `rig.json`；Agent 用 `verify_buddy.py --inspect` 读网格分布生成参数 |
| 要用 PIXI / 自研 WebGL / 物理 | `custom` | 作者写代码，进沙箱 |

- 拿不准 → 一律 `sprite`。
- `renderer` 是可扩展枚举，不是封闭集合。新增一类只需加壳装载器与清单字段，七个动作方法一律不变；步骤见 `references/renderers.md` 的「扩展」一节。

### 路由 2：建目录

```
<id>/
├── buddy.json
├── <model>          # .glb 或图集
├── sound.wav        # 可选
└── runtime.js       # 仅 custom 需要
```

用脚手架生成，别手抄：

```bash
python3 scripts/scaffold.py --id my-pet --name 我的宠物 --renderer sprite --out ./my-pet
```

### 路由 3：实现七个方法

按"性质"写，不按"事件名"写：瞬发方法演完即止，`busy` / `drag` 记住当前状态在 `update` 里读，`lookat` 只存目标值并做插值，不在方法体内直接改 DOM。

### 路由 4：本地校验（必做，禁止肉眼代替）

```bash
python3 scripts/verify_buddy.py ./my-pet
```

退出码：`0` 通过，`1` 有硬错误（缺必填 / 文件缺失 / 超出准入线），`2` 只有警告。硬错误必须清零后才交付。

### 路由 5：装进本地看效果

作者目录在磁盘上落到 `<id>/<version>/`，并让 `installed.json` 有一条启用记录（装包会做，手动试装自己写一条）：

```bash
mkdir -p ~/.mindx/buddies/my-pet/0.1.0 && cp -R ./my-pet/. ~/.mindx/buddies/my-pet/0.1.0/
```

```json
{ "schemaVersion": 1,
  "buddies": [{ "id": "my-pet", "name": "我的宠物", "enabled": true,
    "activeVersion": "0.1.0",
    "versions": [{ "version": "0.1.0", "sha256": "", "installedAt": 0 }],
    "origin": "local" }] }
```

壳 watch 到清单变更即重扫，设置页「宠物」一栏出现条目。加载失败不会崩，回退默认 `activeId` 并保留原选择。

### 路由 6：分发与上架

- 当前 Market 服务端 kind 白名单是 `agent / skill / team / plugin`，**没有 buddy 品类**。上架前先确认服务端已加白名单；未加时只做本地目录分发（`~/.mindx/buddies/`），不要改 kind 硬闯发布接口。
- 服务端支持后，按 `market-publish` 技能打包成 `.mindpkg`，清单与 `buddy.json` 同源。

## 准入线（脚本按此校验，超出即硬错误）

数字取自随包默认包的实测上限（8.38 MB、322,898 三角面、14 张图），即壳已实证能稳定渲染的量级，不另拍更严的线。

| 项           | 上限                                     |
| ------------ | ---------------------------------------- |
| GLB 体积     | 10 MB                                    |
| 图集体积     | 2 MB                                     |
| 贴图边长     | 1024 px                                  |
| 三角面       | 350k                                     |
| 单目录总资源 | 14 MB                                    |
| 加载超时     | 3 s（超时由壳回退默认，组件不自行重试）  |

随包附带的默认包与用户包同规，**无任何准入豁免**：它同样走 `mx-buddy://`、同样按 `renderer` 装载、同样受上表约束，差别只在于它在只读根里、由随包清单声明为默认 `activeId`。它的 `rig.json` 可作为 authored 参数示例（见 `references/reference-implementations.md`），但那是作者侧参考，不是壳内实现。

## 纪律边界

- **气泡文案与图标由壳画**。组件内禁止出现任何中文文案、禁止自绘气泡，否则多语言与主题切换会分裂。
- **禁止写死尺寸**。布局只认 `ctx.size` 与 `viewport`，壳在 S/M/L 三档与任意窗口缩放下复用同一份组件。
- **禁止在组件内做命中判定**。命中由壳对画布做低频 alpha 采样，组件不要另行拦截鼠标事件、不要 `pointer-events: all` 铺满容器。
- **禁止依赖网络**。组件资源全部本地，禁 `fetch` 外链、禁 CDN 字体。
- **`custom` 是代码，必须走独立沙箱页面**（无 node、无 IPC 除一条只读事件通道）；`sprite` / `gltf-clip` 不含可执行代码，禁止把逻辑塞进清单字段绕过这道线。

## 失败模式（现象→原因→动作）

- **现象：宠物不动，只有气泡。** 原因：`clips.idle` 缺失或 GLB 里 clip 名与清单不一致。动作：跑 `verify_buddy.py` 读 clip 清单，按报告改名或补映射。
- **现象：宠物点不到、拖不动。** 原因：`viewport` 比实际画布小，或 `custom` 组件铺满容器拦截事件。动作：改 `viewport` 与实际边界一致；去掉 `pointer-events`。
- **现象：宠物挡住桌面点击。** 原因：锚点或尺寸声明过大，透明区域被当成命中区。动作：缩小 `viewport`，命中以 alpha 为准而非矩形。
- **现象：换包后气泡压在宠物头顶。** 原因：`viewport` 高度与实际绘制高度不一致（上下留白算反）。动作：`viewport` 只写宠物本体边界，气泡条由壳按锚点另算。
- **现象：切换组件后启动变慢。** 原因：资源超准入线或清单声明了外链。动作：跑 `verify_buddy.py` 看体积项，压缩贴图与减面。

## 参考文档索引

| 文档                          | 何时加载                                       |
| ----------------------------- | ---------------------------------------------- |
| `references/contract.md`      | 需要方法时序、坐标系、兜底细节、生命周期边界时 |
| `references/manifest.md`      | 需要字段全集、校验规则、两份完整示例时         |
| `references/branches.md`      | 定实现分支、走程序化 rig（模型→rig.json）时   |
| `references/reference-implementations.md` | 需要一份跑通过的实现作参照（程序化 rig / sprite / gltf）时 |
| `references/renderers.md`     | 实现 sprite 图集规格、gltf clip 过渡、custom 沙箱约束时 |
| `references/host-checklist.md`| 壳侧装载器实现/验收时对照                      |
