# Sheet 互斥单开改栈式多开（glm）

## 2026-10-20 Sheet 互斥单开不满足逐层返回（glm）

- 问题：规则缺失——Sheet 原为互斥单开（add 时已有条目抛错），用户定案改「无限拉起新 Sheet 覆盖旧的」且**栈式逐层返回**。
- 否决：替换式（拉新自动关旧）——用户明确要逐层返回；KeepAlive 保真下层状态——复杂度高首版不做，契约声明「状态不保真、跨层状态自带外部 store」兜底。
- 修正：①`ui-shell/src/views.ts` createSheetView 删互斥抛错，浅拷贝条目并抹掉 `order`（registry 按 order 排序会打乱栈序，Sheet 的 order 不消费）；②SheetPane.vue 渲染目标 `entries[0]` → `entries.at(-1)`，Transition 加 `:key="data.id"` 使层切换重挂载播入场动画；③消费点 `plugins/src/market/overlays/accountSheets.ts` clearSheet 取 `at(-1)`；④contract.md §6.1 改栈式契约。typecheck 通过。
- 验证：连续拉起两层 Sheet 后 Esc 逐层退出、被盖层返回时重挂载播入场动画、重复同 id add 仍抛错。
