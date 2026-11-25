---
version: "1.1.0"
pub_date: 2026-11-26
sha: "cbb0b3567d400bacbcb4d00c3d58e380dce5b09b38a7d53efbab5e4b66b5dc0e"
modifier: ""
modifier_id: ""
---

## 2026-11-26 gltf-clip 装载器落地时暴露四处文档缺口（mainagent）

- 问题：规则缺失——`references/renderers.md` 的 gltf-clip 一节停留在设计稿，与实际装载器行为不符。同节里「模型正面朝 +X 时 yaw 取 0、壳默认给 -90° 到 3/4 侧视」是错的：yaw 默认 0，且每只模型朝向都不同，同批四只宠的正脸分别落在 0.7 / 4.65 / 4.7，靠猜必错；过渡写的 120 ms 实际是 220 ms；归一化写的「缩放到 ctx.size」实际是 heightTo / groundY 两个显式参数；三角面上限写的 60k 与 SKILL.md 准入线的 350k 冲突。最要紧的一条完全没写：装载器会剥掉 clip 里所有 `.position` 轨道——不剥的话 run 一播，整只模型顶到镜头前，画面里只剩一张脸。
- 否决：不做根位移剥离、改由作者保证 in-place 动作。否决原因：GLB 内部看不出位移烘在哪些轨道上，作者本人也不知道自己交的动作带根位移，无法靠交付规范兜住。
- 修正：重写 `references/renderers.md` 的 gltf-clip 一节（必填项与缺 idle 即禁用、clips 语义键可缺的后果、yaw 的实测方法、heightTo/groundY、position 剥离及其失真代价、220 ms 过渡、durations 截断、head 骨骼视线跟随、命中阈值、准入线）；`references/manifest.md` 字段表补 `heightTo` / `groundY` 并订正 `durations` 与 `clips` 说明；`SKILL.md` 清单字段表补三个 gltf-clip 专用字段，version 1.0.0 → 1.1.0。
- 验证：`mindx skill validate --path` 0 issue；忍者包 `verify_buddy.py` 退出码 0（仅 viewport 未声明一条警告）；四态取帧见 /tmp/nj4-sheet.png，其中 idle=dance_05、busy=dance_03、play/jump=cheer，question 因 bow 会趴地而未映射。
