---
version: "1.5.1"
pub_date: 2026-10-25
sha: "9826c7fc3a6091d3923776ea0900e1403c462857b57bd953f6f9c59ab07ccbc0"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 Linux 打包阻塞修复：executableName 显式化（glm）

- 触发：全量构建 Linux target 报 `executableName contains characters that cannot be safely used in file paths: @mindx-workelectron`（appImageUtil.validateCriticalPathString 拒绝 @ 字符）。根因：electron/package.json name 为 pnpm workspace scope 名 `@mindx-work/electron`，electron-builder 对 Linux 可执行名缺省从 name 推导（去斜杠 → `@mindx-workelectron`）；mac 不暴露（.app 名用 productName: MindX）。用户追问「这不是开发机上的名字吗」后实证溯源确认：名字在仓库文件内，任何机器打 Linux 包都会炸，非开发机特有。
- 修正：electron-builder.yml linux 段显式 `executableName: mindx`（不动 package.json name——workspace 内部标识，改动牵连引用）；SKILL 坑 11；version 1.5.0 → 1.5.1。
- 验证：修后重跑 `pnpm run pack` 全量构建（结果见后续记录）。
