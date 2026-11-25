---
updated: 2026-11-25
scope: Buddy 实现分支（可选技术栈）
---

# 实现分支

六个分支，按"作者要交什么"区分。前四个零代码，普通用户能独立完成；后两个需要写代码或有官方参与。

| 分支 | `renderer` | 作者交付 | 要写代码 | 壳侧要求 |
| ---- | -------- | -------- | -------- | -------- |
| 3D 骨骼 | `gltf-clip` | `.glb`（内含动画 clip） | 否 | gltf 装载器（现 Three.js 可复用） |
| 2D 序列帧 | `sprite` | 一张图集 | 否 | canvas 装载器 |
| Lottie | `lottie` | AE 导出的 `.json` + images | 否 | lottie-web 装载器 |
| 视频循环 | `video` | `.webm`/`.mp4`（透明底） | 否 | `<video>` 装载器 |
| 程序化 rig | `procedural` | 模型 + `rig.json` | 是（Agent 生成参数） | 通用驱动器 + `rig.json` |
| 自定义 | `custom` | `runtime.js` + 资源 | 是（作者自写） | 沙箱页 |

## 分支一：gltf-clip（3D 骨骼，零代码）

普通用户走这条的前置条件是"手上有一个带骨骼动画的 GLB"。做法：

1. 准备 GLB：Blender 导出 glTF 2.0，勾选 Animation，贴图内嵌（Embedded）。
2. 至少做一条 `Idle` 循环动画；想要反应更丰富，再补 `Typing`（忙碌）、`Confused`（疑惑）、`Happy`（完成）。
3. 在 `buddy.json` 写 `"renderer": "gltf-clip"` 与 `clips` 映射，clip 名与 Blender 里的 Action 名逐字一致。
4. 跑 `verify_buddy.py`，它会解析 GLB 报告真实 clip 名与三角面数。

不需要写任何代码，也不需要懂 Three.js。导出时的常见错：只勾了 Pose 没勾 Action（clip 变成空）、贴图留在外部（换机丢图）、模型朝向不是 `+X`（用 `yaw` 修）。

## 分支二：sprite（2D 序列帧，零代码）

最省成本，适合手绘、像素风、夸张表情。

1. 画一张图集：横向排帧或网格排帧，透明底，每帧同尺寸。
2. `frames` 声明 `count/cols/fps`，`clips` 用帧区间字符串（`"idle": "0-3"`）。
3. 帧率建议 8–12；低于 6 会卡顿，高于 20 对宠物体量没必要。

## 分支三：lottie（零代码）

适合矢量动效、UI 风格角色、表情包式角色。

1. AE 用 Bodymovin 导出 JSON（勾 Include images 时导成一个文件夹，打包时一起带上）。
2. `buddy.json`：`"renderer": "lottie"`、`"model": "pet.json"`；`clips` 映射到 lottie 里的 marker 名。
3. 交互响应靠标记段：给每个动作单独打 marker，壳按 marker 播放；没有 marker 的 lottie 只能循环整段。

## 分支四：video（零代码，最省事）

适合已有形象的快速上线，代价是互动性弱、体积大。

1. 导一段 2–4 秒循环，透明底 WebM（VP9 + alpha）优先，MP4 无 alpha 会带黑底。
2. 一个视频对应一个动作；想让 `busy` 与 `idle` 不同，就准备两段并在 `clips` 指向各自文件。
3. 体积压到 3 MB 内，超过会在宠物窗首帧卡顿。

## 分支五：程序化 rig（静态模型，Agent 可产出）

适用：只有静态模型、无骨骼动画，但要"会呼吸、会转头、会摸头抖耳"这类反应。

唯一做法：`renderer: "procedural"` + 包内 `rig.json`，壳内是通用驱动器（按 `rig.json` 读判据与动作参数）。默认包与用户包同规，壳代码不含任何宠物 id 判断。

禁止：把某只宠物的绑定写成壳内专属模块，或声明 `procedural:<kind>` 之类按宠物分家的写法。壳只认 `renderer` 类型、不认具体宠物；一旦某只宠物需要专属代码，它就不可替换，这条路线当场作废，改走 `custom`。

Agent 工作流：

1. 读网格分布：`python3 verify_buddy.py --inspect model.glb`，拿到每个 mesh 的 `bbox`、`center`、`verts`。分件只依据包围盒，不凭 mesh 名字（导出后常是 `meshes[i]`）。
2. 生成初稿：`python3 gen_buddy.py --model model.glb --id xx --name 名字 --out ./xx`，自动判分支并写 `buddy.json`（无 clip 时附 `rig.json`）。
3. 复核判据：把包围盒归一化到 0..1，按高度、左右外扩、前后位置聚成头/耳/眼/前肢/尾五组；`rig.json` 的 `parts` 只写判据区间，不写 mesh 索引，换模型才可复用。
4. 本地试跑：静止态逐 mesh 世界坐标必须与原始模型一致；旋转态接缝不裂、不露腔。
5. 验收：见下方「可复核判据」与 `references/reference-implementations.md` 参考实现 A。

分件口径、pivot 补偿公式、动作参数与已踩过的坑不在正文，见 `references/reference-implementations.md`。

### 可复核判据

- 静止帧与原始模型逐 mesh 世界坐标一致（零位移），否则 pivot 补偿算错。
- 头部转到声明上限时，头身接缝无裂缝。
- 摸头耳抖与尾摆不穿模；`busy` 时视线落在前肢操作区。
- 同构模型（同部件数、同相对位置）复用同一份 `rig.json` 仍能跑。

## 分支六：custom（自带代码）

适合物理摆动、着色器特效、Canvas 手绘程序化角色。

1. 写 `runtime.js`，导出 `createBuddy(host)`，返回七个动作方法（未实现的省略即可）。
2. 可用 PIXI、Babylon、原生 Canvas；禁用网络请求、禁 node API、单帧预算 8 ms。
3. 壳把它放进沙箱页面（`nodeIntegration:false`、`contextIsolation:true`），只给 `container`、`assets`、`size`、`theme`、`onEvent`。

## 分支选择的三句话

- 手上有带动画的 3D 模型 → 分支一。
- 想自己画、想便宜、想夸张 → 分支二。
- 只有静态模型又想要牛的质感 → 分支五，别承诺"用户自己能做"。
