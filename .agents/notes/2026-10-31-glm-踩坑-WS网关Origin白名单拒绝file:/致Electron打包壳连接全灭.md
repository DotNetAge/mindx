# 踩坑：WS 网关 Origin 白名单拒绝 file:// 致 Electron 打包壳连接全灭

- 记录时间: 2026-10-31 17:35
- agent: glm
- 分类: 踩坑
- 仓库: mindx（修复 e403b16，tag v2.6.8）

## 现象

mindx-work 打包版（Electron）连不上 daemon：首连/重连全灭，daemon 日志
`rejected WebSocket connection origin=file://`，UI 停在"会话加载失败"。
CLI 不受影响（Go 进程不发 Origin 头，gort 对空 Origin 直接放行）。

## 根因

gort v0.1.9 gateway 的 Upgrader.CheckOrigin 默认白名单只含
localhost/127.0.0.1/内网段/wails://（为 Wails 壳 mindx-app 定制）。
Electron 打包壳从 file:// 加载页面，Chromium 对 file:// 页面发起的
WebSocket 恒发 `Origin: file://` → 403。mindx 2.6.7 起 gort 升级带上
该闸，所有升级 daemon 的 Electron 壳用户即刻全灭。

## 排查路径（复用价值）

1. lsof 1314 看 ESTABLISHED：0 条 → 连接根本没建立（排除重连逻辑问题）
2. CDP 在渲染层裸 `new WebSocket('ws://127.0.0.1:1314/ws')` → ERROR
   （排除壳装配问题，锁定握手层）
3. curl 带与不带 `Origin: file://` 对比 → 不带 101、带 403，daemon 日志
   打出白名单原文 → 根因石锤

## 修复

mindx daemon.go initGateway 经 `gateway.WithWSConfig` 整体覆盖白名单 =
gort 默认列表 + `file://`。gort 未导出 defaultWSConfig，列表手抄，
升级 gort 时必须对齐漂移（注释已标）。

## 教训

- 新增任何网络闸门（Origin/Origin 外的任何握手校验）时，枚举全部真实
  客户端形态（CLI/Wails 壳/Electron 壳/浏览器直开）逐一过闸，缺一个
  形态就是一次全灭事故。mindx AGENTS.md 已有"新鉴权闸门必须拿真实客户端
  链路实测"军规，本次实证该军规的"客户端形态清单"还须显式含
  Electron file:// 场景。
- mindx-work 的 mx-release 技能渲染冒烟只断言"#app 有子元素、body 有
  文本"，未断言 daemon 连接状态——本次打包验证靠运气暴露不了这类问题，
  已建议在冒烟中加入连接断言（见 mindx-work 侧 notes）。
