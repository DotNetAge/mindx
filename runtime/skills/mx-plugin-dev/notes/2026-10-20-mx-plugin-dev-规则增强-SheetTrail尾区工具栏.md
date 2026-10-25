# Sheet 抽层尾区工具栏（glm）

## 2026-10-20 Sheet 抽层无尾区动作条（glm）

- 问题：规则缺失——Sheet 只有头部 chrome + 本体，注册者无法定义底部动作条（用户提出「可以定义 Sheet Trail 的工具栏」）；文档/编辑类抽层只能把动作按钮塞进本体滚动流，滚动后动作不可达。
- 修正：①`ui-shell/src/views.ts` SheetEntry 加可选 `trail?: C`；②SheetPane.vue 本体之下渲染 `.sheetTrail` 钉底尾区（flex none、顶部 0.5px 分隔线、本体超高滚动不挤压尾区）；③contract.md §6.1 同步契约（trail 参数 + 纪律：只放动作控件，禁止塞正文内容）。typecheck 通过。
- 验证：`cd mindx-work && pnpm typecheck`；实机验证点 = 首个消费插件（markdown / docpreview 类抽层）接入后回归钉底行为。
