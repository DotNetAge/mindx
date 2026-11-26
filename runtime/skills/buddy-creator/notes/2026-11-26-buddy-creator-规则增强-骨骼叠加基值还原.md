---
version: "1.1.0"
pub_date: 2026-11-26
sha: "6cce9dfc8bc184927cfb4209c518146403e5e5a4e83884bc4fe9971da0baf1ed"
modifier: ""
modifier_id: ""
---

## 2026-11-26 冻结 idle 时头部每帧累加自转（mainagent）

- 问题：实现缺陷——绝对叠加没有先还原骨骼基值。`idleSpeed: 0` 冻结 idle 后 mixer 不再重写 head 轨道，`premultiply` 的输入就成了上一帧叠加后的结果，角度逐帧累乘，头会一路转。单帧截图里看起来像「头偏过去了」，实测连拍才发现视线不变时头四元数每帧都不同（连续两次读数 `[-0.02,0.01,-0.03,0.999]` 固定一致才是对的，修前是 `[0.18,0.15,...]` → `[-0.68,-0.71,...]`）。
- 否决：只把 `idleSpeed` 保持 ≥0.05 绕开冻结。否决原因：冻结是合法需求（素材只有舞蹈时用来静立），不能为了绕 bug 禁掉一项能力。
- 修正：update 开头若已有基值，先把 head 骨骼四元数 copy 回基值，再 `mixer.update`，然后把更新后的值存为基值。写进 `references/renderers.md` 的 gltf-clip 节。
- 验证：冻结 idle 下连续两次 `readGaze()` 的 head 四元数完全一致；三向强制视线（-0.9 / +0.9 / 上）身体像素完全不动、只有头转动，见 /tmp/nj2-follow-fixed.png；四态 idle/look_around/agree/dance_05 抓帧正常，见 /tmp/nj2-states.png。
