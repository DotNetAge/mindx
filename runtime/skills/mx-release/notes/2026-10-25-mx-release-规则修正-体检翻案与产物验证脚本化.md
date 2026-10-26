---
version: "1.3.0"
pub_date: 2026-10-25
sha: "3054a3abade33a46f9f7a26e3e6c89ec103a2c0cb224fdc005db25fc0cb8a810"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 体检复检翻案：脚本化维度漏检 + 正文日志噪声 + spctl 判定标准实证校准（glm）

- 触发：用户否定首轮体检结论「健康」——指出两项漏检：①技能该脚本化的确定性操作没落技能 scripts/（产物验证三连当时是正文手敲命令清单）；②正文残留日志式噪声。用户纠正=最强信号，教训已固化进 mindx-work/AGENTS.md「技能体检纪律」节（validate 全绿只是底线，必查脚本化与日志噪声两维度）。
- 修正：新建 `scripts/verify_artifacts.mjs`（自包含零依赖，三连判定 + JSON stdout + 退出码 1/2/3/4 分型 + --help）；正文验证节改脚本路由 + 手敲仅作兜底；删日期戳「（2026-10-25 用户定案）」「（全部实证）」与重复句「这是唯一发版动作，无任何 CI 备选」。pack/release 本体留仓库 scripts/ 并说明依据（须在 pnpm workspace + git tag 上下文执行，复制即双源漂移）。version 1.2.1 → 1.3.0。
- 实证校准（复检副产物，两条标准原文有错）：①spctl 实机输出为 `source=Developer ID`+`origin=Developer ID Application: …`，并非旧文写的 `source=Notarized Developer ID`（随 macOS 版本漂移）——gatekeeper 判定改为 accepted + source=(Notarized )?Developer ID + origin=Developer ID Application；②`stapler validate` 报 dist/ 现存 2.6.0 产物 does not have a ticket stapled to it（exit 65）——**脚本真实拦下不合格产物**，非误判：这批产物无公证票据，不可用于发布。
- 验证：脚本实跑 dist/mac-arm64/MindX.app → gatekeeper=true / notarization=false / signature=true，退出码 2 精确命中公证项；注意管道后 `$?` 取的是管道末命令退出码，验退出码须单独跑或用 PIPESTATUS。
