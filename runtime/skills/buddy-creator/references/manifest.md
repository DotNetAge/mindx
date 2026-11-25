---
updated: 2026-11-25
scope: buddy.json v1
---

# 清单字段与校验

## 字段全集

| 字段          | 类型                                          | 必填 | 校验规则                                                  |
| ------------- | --------------------------------------------- | ---- | --------------------------------------------------------- |
| `id`          | string                                        | 是   | `^[a-z0-9][a-z0-9-]{1,31}$`，且与目录名一致                |
| `name`        | string                                        | 是   | 1–24 字符                                                  |
| `version`     | string                                        | 是   | `MAJOR.MINOR.PATCH`                                        |
| `renderer`    | `gltf-clip` \| `sprite` \| `custom`           | 是   | 枚举值                                                     |
| `model`       | string                                        | 是   | 相对路径，存在且不越出组件目录；`gltf-clip` 须 `.glb`      |
| `author`      | string                                        | 否   | ≤40 字符                                                   |
| `description` | string                                        | 否   | ≤80 字符，壳在设置页展示                                   |
| `clips`       | object                                        | 否   | 键限 `idle/busy/question/complete/failed/notice/update/play/jump/drag`，值为 GLB 内 clip 名 |
| `frames`      | object                                        | sprite 必填 | `count` ≥1，`cols` ≥1，`count ≤ cols × rows`，`fps` 1–30 |
| `viewport`    | object                                        | 否   | `width/height` 16–512 整数，`anchor` ∈ `bottom-center/bottom-left/bottom-right/center` |
| `sound`       | string                                        | 否   | `.wav`/`.mp3`，≤512 KB                                     |
| `durations`   | object                                        | 否   | 键同 `clips`，值 200–3000 毫秒，供壳判定瞬发动作结束       |
| `yaw`         | number                                        | 否   | `-3.14..3.14`，模型初始朝向偏角                            |
| `scale`       | number                                        | 否   | `0.2..3.0`，在 `ctx.size` 内的等比缩放                     |
| `rig`         | string                                        | procedural 必填 | 同目录 `rig.json` 的文件名；内容为部件判据与动作参数，字段见 `branches.md` 分支五 |

## 校验分级

- 硬错误（退出码 1）：必填缺失、id 不合规、文件不存在、体积超准入线、`renderer` 非法、`sprite` 缺 `frames`、GLB 内找不到 `clips.idle` 指向的动画。
- 警告（退出码 2）：`clips` 覆盖不全（只有 `idle`）、缺 `viewport`、缺 `description`、`sound` 过大、贴图边长超 1024。
- 通过（退出码 0）：无硬错误且无警告。

## 示例一：sprite（2D 序列帧）

```json
{
  "id": "banana",
  "name": "会扭的香蕉",
  "version": "0.1.0",
  "renderer": "sprite",
  "model": "banana.webp",
  "frames": { "count": 8, "cols": 4, "fps": 10 },
  "viewport": { "width": 96, "height": 128, "anchor": "bottom-center" },
  "sound": "squeak.wav",
  "description": "一根会扭的香蕉"
}
```

图集按 `cols` 等分，行数 = `ceil(count / cols)`，第一帧在左上，向右向下读取，透明底。

## 示例二：gltf-clip（3D 骨骼）

```json
{
  "id": "cow",
  "name": "小牛",
  "version": "1.0.0",
  "renderer": "gltf-clip",
  "model": "cow.glb",
  "clips": {
    "idle": "Idle",
    "busy": "Typing",
    "question": "Confused",
    "complete": "Happy",
    "play": "Petted"
  },
  "durations": { "complete": 1400, "question": 1200, "play": 900 },
  "viewport": { "width": 107, "height": 107, "anchor": "bottom-center" },
  "yaw": 0.58,
  "description": "经典桌面小牛"
}
```

`clips` 的键是语义名，值是 GLB 里 `AnimationClip.name`，必须逐字一致（大小写敏感），拼错时壳只能静默不播，因此必须过 `verify_buddy.py`。

## 最小可运行清单

只有三个字段也能跑（壳取默认）：`id`、`name`、`version`，加 `renderer` 与 `model`。其余全部有缺省。先用最小清单跑通挂载，再逐项补动作映射。
