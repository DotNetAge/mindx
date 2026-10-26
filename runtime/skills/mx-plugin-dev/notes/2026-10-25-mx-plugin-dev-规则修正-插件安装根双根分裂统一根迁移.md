---
version: "0.2.0"
pub_date: 2026-10-25
sha: "e98f8f19a69e6daae58f5f633792f5f28cba0c505540417a5502e650b0dfd484"
modifier: "Ray"
modifier_id: "1"
---

# 插件安装根双根分裂统一根迁移（glm）

## 2026-10-25 插件安装根双根分裂致 CLI 装包实例不可见（glm）

- 问题：指令冲突（定案变更未全链扫尾）——用户实测「mw CLI 装插件，设置页不显示」实锤双根 → 用户定案铁律：**已安装可执行插件构建物必须落 `~/.mindx/plugins`（机器级唯一根），装进 userData = 工程错误**；插件源码归项目 `.agents/plugins/<id>/`（源码/构建物双根分层）。
- 否决：MW_PLUGIN_ROOT env 对齐补丁——治标不治本，否决。
- 修正：electron/src/plugins.ts + cli.ts 双侧改根 `homedir()/.mindx/plugins`（删 defaultUserDataRoot）；实机迁移两插件+installed.json、清 userData 旧账；扩散修正 SKILL「安装根军规」、contract §18.3/§18.7、scaffold 注释、规范 §6、AGENTS「插件根铁律」节。严格复扫轮又挖 3 处（plugin-store 头注释、AGENTS 产品名条目尾句、archify 架构图）。
- 验证：CLI 双通道 `plugin list` 过；编译产物 dist/cli.js 与 plugins.js 各 1 处新根、0 处旧根；mindx Go daemon / mindx-market 服务端 / .agents / memory 全零残留。
