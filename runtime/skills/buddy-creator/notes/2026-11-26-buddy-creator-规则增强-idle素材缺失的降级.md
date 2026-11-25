---
version: "1.1.0"
pub_date: 2026-11-26
sha: "152dd3f75c8bc901e6fea66855de81fa1e3aaa9d903a22787328f7c12e3edcfa"
modifier: ""
modifier_id: ""
---

## 2026-11-26 素材只有舞蹈时宠物一启动就在跳舞（mainagent）

- 问题：规则缺失——gltf-clip 的必填项只要求 `clips.idle` 存在，没规定「素材里没有真正 idle 时怎么办」。忍者只有 run / cheer / bow / dance_03 / dance_05 五段，只能把 `dance_05` 塞进 idle，结果宠物一启用就在全速跳 2.88 秒一轮的舞，用户第一反应是「它停不下来」。这是映射硬凑造成的，不是装载器缺陷。
- 否决：把 `clips.idle` 设为空让模型站 T-pose。否决原因：绑定姿势通常是 T-pose 或 A-pose，比舞蹈更难看成「宠物」，而且壳规定缺 idle 即禁用整包，语义冲突。
- 修正：新增 `idleSpeed`（0..3，默认 1；0 冻结在 clip 首帧，0.1–0.3 慢放）。写进 `references/renderers.md` 的 gltf-clip 节、`references/manifest.md` 字段表、`SKILL.md` 清单字段表。忍者包设 0.15，一轮舞蹈拉到 19 秒，观感变成缓慢晃动。
- 验证：同一模型取帧对比 idleSpeed=0（定格）与 0.15（t0 / t+3s 两帧），见 /tmp/nj-idle-compare.png；两帧头部差异来自 gaze 跟随，与 idle 无关。
