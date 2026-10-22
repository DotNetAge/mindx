# 发行搭载与市场分流的 distribution 定案（2026-10-22）

用户定案：官方插件不再随发行版搭载，统一走资源社区按需安装。gitgraph / svgboard / video-editor / mailbox 四个插件全部 `"distribution": "market"`，dist-optional 现为空（0 个搭载）。

## 机制

- plugin.json 加 `distribution` 字段：缺省 = 搭载发行（dist-optional/ → electron-builder extraResources）；`"market"` = 市场分发（dist-market/ → 展开落 mindx-market/contents/plugins/<id>/ → build-market.mjs 上架）。
- build.mjs 构建产物分流输出，两个输出目录都先清空旧 .mindpkg。
- 展开落 contents = unzipSync 全文件写盘（entry.mjs / manifest.json / assets/icon.svg），build-market.mjs 从该目录重新打包（walkFiles 全收）。
- 已装实例不受影响：installed.json 与发行搭载无关；货架安装走 /api/v1/packages。

## 下次我看到「官方插件要上架资源社区」时，我将

1. plugin.json 加 `"distribution": "market"`
2. `node scripts/optional-plugins/build.mjs`（dist-market 出包）
3. unzipSync 展开到 mindx-market/contents/plugins/<id>/
4. `node scripts/build-market.mjs` 验证（计数含新插件包）
5. 同步两份技能副本（SKILL.md / contract.md）与规范 §4/§11.1

## 关键位置

- mindx-work/scripts/optional-plugins/build.mjs：distribution 分流（L70 marketOutDir、L213-215 输出分流、主流程双目录清空）
- mindx-work/.agents/plugins/<id>/plugin.json：四插件 distribution=market
- mindx-work/docs/插件生命周期规范.md §4（distribution 字段）/ §11.1（分流）
