---
version: "1.3.1"
pub_date: 2026-10-25
sha: "f7d3d007995eca19838fee9e075e4201848ec06a80f287fc1ff4b199aedc1732"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 pnpm pack 内置命令劫持：SKILL 命令静默失败（glm）

- 触发：用户要求全量构建产物，按 SKILL 正文执行裸 `pnpm pack`——pnpm 内置 tarball 命令优先于 package.json 同名 script，静默产出 mindx-work-0.1.0.tgz（17.8MB 仓库 tarball）且 exit 0，真打包从未启动。output 出现「Tarball Details」即露馅。
- 修正：SKILL 本地打包节四条命令统一改 `pnpm run pack` 形式（`pnpm run` 显式执行 script 不受劫持），新增坑 9；version 1.3.0 → 1.3.1。误产 tarball 已删。release.mjs 用 `npm run pack:<platform>` 不受影响（实证 grep）。
- 教训：pnpm/npm 对与内置命令同名的 script（pack/test/start…），裸调用走内置——技能里写 package.json script 命令必须带 `run` 或先实证裸名可用。
