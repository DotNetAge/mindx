---
version: "0.2.0"
pub_date: 2026-10-20
sha: "11a5637fc7c4286705c29427d3e7df66cc228e84b755db2b9bf73c3a07afea87"
modifier: "Ray"
modifier_id: "1"
---

# Overlay 弹窗分区与可调卡片（glm）

## 2026-10-20 Overlay 弹窗卡片可调但 references 未同步（glm）

- 问题：规则缺失——Overlay modal 卡片壳两项升级（卡片可拖拽调宽高 + 原语类 `mx-modal-body`/`mx-modal-footer` 钉底分区）未同步进技能 references；插件开发者继续自造 `.actions` 按钮行，卡片拉矮/拉大后按钮随内容滚走或被裁剪。
- 否决：只在 SKILL.md 正文写弹窗纪律——SKILL.md 是决策路由，样式与契约细节归 references；原语清单留在 uikit.md 单一事实源。
- 修正：①contract.md §6 补「modal 卡片壳是用户可调的」契约条目（右下角手柄、宽上限 960、卡片自身不滚动、内容弹窗必须 body/footer 拆区、确认小弹窗可豁免）；②uikit.md 补弹窗分区原语；③uikit.md 高频错误补「自造 .actions 按钮行会随内容滚走」反模式。壳侧事实源：`ui-shell-vue/src/components/OverlayPane.vue` 与 `styles/controls.css`。
- 验证：实机范式见 `plugins/src/market/overlays/PublishModal.vue` / `InboxModal.vue` / `SubmitModal.vue`（均已按 body/footer 拆区接入）。
