# 2026-10-20 Sheet 互斥改栈式多开（glm）

- 问题：规则缺失——Sheet 原为互斥单开（add 时已有条目抛错），2026-10-20 用户定案改「无限拉起新 Sheet 覆盖旧的」且选择**栈式逐层返回**（AskUserQuestion 二选一：替换式被否）。
- 否决：替换式（拉新自动关旧）——用户明确要逐层返回；KeepAlive 保真下层组件状态——复杂度高，首版不做，契约里声明「状态不保真、跨层状态自带外部 store」兜底。
- 修正：①`ui-shell/src/views.ts` createSheetView 删互斥抛错，浅拷贝条目并抹掉 `order`（registry 按 order 稳定排序会打乱「追加即栈顶」的栈序，Sheet 的 order 不消费）；②`ui-shell-vue/src/components/SheetPane.vue` 渲染目标 `entries[0]` → `entries.at(-1)`（栈顶），Transition 加 `:key="data.id"` 使层切换必重挂载播入场动画；③消费点同步 `plugins/src/market/overlays/accountSheets.ts` 的 clearSheet 取 `at(-1)`（原 entries[0] 在栈式下取到栈底，替换语义失效）；④技能 contract.md §6.1 改栈式契约（多开、同 id 仍抛错、order 不消费、状态不保真声明）。typecheck 通过，无既有测试依赖互斥。
- 验证：`cd mindx-work && pnpm typecheck`；实机验证点——连续拉起两层 Sheet 后 Esc 逐层退出、被盖层返回时重挂载播入场动画、重复同 id add 仍抛错。
