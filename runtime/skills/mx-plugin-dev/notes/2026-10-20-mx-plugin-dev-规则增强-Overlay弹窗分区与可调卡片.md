# 2026-10-20 Overlay 弹窗分区与可调卡片（glm）

- 问题：规则缺失——壳（ui-shell-vue）的 Overlay modal 卡片壳完成两项升级（卡片可拖拽调宽高 + 原语类 `mx-modal-body`/`mx-modal-footer` 钉底分区），但技能 references（contract.md §6、uikit.md 控件原语清单）未同步。后果：插件开发者继续自造 `.actions` 按钮行，卡片拉矮/拉大后按钮随内容滚走或被裁剪；也不知道卡片已被用户调宽高，内容不拆分区就无法适配。
- 否决：只在 SKILL.md 正文写弹窗纪律——SKILL.md 是决策路由，样式与契约细节按三层结构归 references；正文的「资料全部收纳」原则要求原语清单留在 uikit.md 单一事实源。
- 修正：三处同步——①contract.md §6 补「modal 卡片壳是用户可调的」契约条目（右下角手柄、宽上限 960、卡片自身不滚动、内容弹窗必须 body/footer 拆区、确认小弹窗可豁免）；②uikit.md 控件原语清单补「弹窗分区 `.mx-modal-body`+`.mx-modal-footer`」；③uikit.md 高频错误补「自造 .actions 按钮行会随内容滚走」反模式。壳侧事实源：`ui-shell-vue/src/components/OverlayPane.vue`（.modalCard flex column + resize both）与 `ui-shell-vue/src/styles/controls.css`（弹窗分区节）。
- 验证：读 mindx-work/ui-shell-vue/src/components/OverlayPane.vue 与 controls.css 对照契约描述；实机验证范式见 `plugins/src/market/overlays/PublishModal.vue` / `InboxModal.vue` / `SubmitModal.vue`（均已按 body/footer 拆区接入）。
