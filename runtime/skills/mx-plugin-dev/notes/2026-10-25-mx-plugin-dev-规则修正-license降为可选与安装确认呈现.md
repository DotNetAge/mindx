# license 降为可选与安装确认呈现（glm）

## 2026-10-25 market-hello 加载被拒「license 不能为空」（glm）

- 问题：规则缺失（字段必填性定案过严）——用户定案：license 非必填，缺失不显示；scaffold 默认 MIT，改其它协议须用户明示。
- 修正：五面同步——①校验唯一家 ui-shell/src/plugin-manifest.ts（`license?: string` +「若提供必须非空白」）+ node 镜像 plugin-store.ts + market 仓 build-market.mjs 三处同文案；②渲染 InstallConfirmModal 许可证行 v-if、MarketManageRow 授权标签+值整行 v-if（标签/值平铺结构必须连标签一起包）、MarketPaneRow 本就条件渲染；③文档规范 §5/§9、contract §18.3/§18.6、两处组件头注释；④scaffold 默认 MIT 已有。官方可选件构建器 build.mjs 保持必填（自家产品协议必须显式，不引入静默默认）。
- 验证：ui-shell + electron typecheck 双绿；market-hello 重启 dev 后加载通过、设置页正常显示。

下次遇到「必填字段」定案变更，我将：按「校验唯一家 + node 镜像 + 各渲染点 + 文档契约 + scaffold 默认值」五面查改；标签/值平铺结构的 v-if 必须连标签一起包。
