# 发行搭载与市场分流的 distribution 定案（glm）

## 2026-10-22 官方插件发行搭载与市场分流无标记字段（glm）

- 问题：规则缺失——官方插件上架资源社区（市场）与随发行版搭载两条分发通道并存，plugin.json 无分流标记，构建管线无法分流输出。
- 修正：plugin.json 加 `distribution` 字段（缺省 = 搭载发行 dist-optional/；`"market"` = 市场分发 dist-market/ → 展开落 mindx-market/contents/plugins/<id>/ → build-market.mjs 上架）；build.mjs 分流输出 + 双目录先清旧包；gitgraph / svgboard / video-editor / mailbox 四插件全部 `market`（dist-optional 现为空）。
- 验证：`node scripts/optional-plugins/build.mjs` 计数含四插件于 dist-market；`node scripts/build-market.mjs` 上架链验证；已装实例不受影响（installed.json 与搭载无关，货架安装走 /api/v1/packages）。

下次我看到「官方插件要上架资源社区」时，我将：plugin.json 加 `"distribution": "market"` → build.mjs 出包 → 展开落 contents → build-market.mjs 验证 → 同步两份技能副本与规范 §4/§11.1。
