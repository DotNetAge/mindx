---
version: "1.5.0"
pub_date: 2026-10-25
sha: "5b089f177e86284533abc70458f6b6a83b90e6d0d806eb36b15fe238c13d54d4"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 版本策略定案：daemon 从「按 tag 精确下载」改「拉最新发布版」（glm）

- 触发：用户追问「版本锚取舍」表述看不懂（用人话重讲 mac daemon=按版本号精确下载 vs win rootfs=docker latest 浮动的差异后），AskUser 三选项定案「也改拉最新」——推翻「禁止 latest」旧定案（该定案防的是 2.5.6/2.5.3 错配，错配根源是本机构建与 CI 产物不同步；现构建物一律取自远端发布，浮动 latest 的错配风险已消失，定案更换成立）。
- 修正：resolve-mindx-version.mjs 权威来源改为 GitHub API `releases/latest`（天然排除 draft/prerelease），本地 mindx 仓库 git tag 降为离线回退、daemon-installer.ts 兜底字面量殿后；build-mindx-bundle.mjs 注释同步。SKILL「构建物来源区分」节改写 + 坑 10（包内 mindx 版本随打包时间浮动 + 追溯手段 + 限流回退核对项）。version 1.4.0 → 1.5.0。
- 实证：`node scripts/resolve-mindx-version.mjs` 直跑输出 2.6.1（GitHub API 路径真实工作）。
- 教训：向用户汇报「取舍待确认」时禁用「版本锚 / floating latest」类黑话——先给两方案的大白话对照表（怎么拿 / 结果是什么），再用 AskUser 收口；本轮先术语后提问被用户打回重讲。
