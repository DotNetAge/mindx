# 2026-10-20 Sheet Trail 尾区工具栏（glm）

- 问题：规则缺失——Sheet 抽层只有头部 chrome（Title + 关闭钮）+ 本体，注册者无法定义 Sheet 底部的动作条（2026-10-20 用户提出「可以定义 Sheet Trail 的工具栏」）；文档/编辑类抽层只能把动作按钮塞进本体滚动流里，滚动后动作不可达。
- 修正：壳三层同步——①`ui-shell/src/views.ts` SheetEntry 加可选 `trail?: C`（尾区工具栏组件，注释写明语义与适用场景）；②`ui-shell-vue/src/components/SheetPane.vue` 本体之下渲染 `.sheetTrail` 钉底尾区（flex none、顶部 0.5px 分隔线对齐 modal footer、padding 走 token、本体超高滚动不挤压尾区）；③技能 contract.md §6.1 同步契约（API 签名加 trail 参数 + 使用纪律：trail 只放动作控件，禁止塞正文内容）。typecheck 通过。
- 验证：`cd mindx-work && pnpm typecheck`；实机验证范式待首个消费插件接入（markdown / docpreview 类抽层）后回归确认钉底行为。
