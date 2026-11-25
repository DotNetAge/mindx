---
updated: 2026-11-25
scope: 壳侧 Buddy 宿主 v1
---

# 宿主验收清单

组件契约的另一半在壳里。逐项完成并给出判定方法，全部满足才算宿主可用。

| # | 能力 | 完成判定 |
| - | ---- | -------- |
| 1 | 目录枚举：扫 `~/.mindx/buddies/<id>/buddy.json`，同 id 用户目录优先，内置资源兜底 | 放一个目录进去，设置页出现条目；删掉后条目消失且当前宠物回退默认 |
| 2 | 资源协议：`mx-buddy://<id>/<path>` 映射到组件目录，越界路径（`..`）拒绝 | 组件内拼 `ctx.assets + 'x.glb'` 可加载；请求 `../../../etc/passwd` 返回 403 |
| 3 | 装载器：按 `renderer` 分派 gltf-clip / sprite / custom，异步 + 3 s 硬超时 | 把 GLB 换成 0 字节文件，宠物回退默认且壳不崩、启动时间不变 |
| 4 | 事件映射：`complete/failed/notice/update` → `notify(kind)`；`ask` → `question()`；会话执行 → `busy(on)`；点中 → `play()`；拖拽 → `drag(on/off)` | 主窗 `mascot.setState({kind:'notify',title:'测试'})` 触发，组件 `notify` 被调用一次 |
| 5 | 视线：光标 50 ms 归一化送 `lookat(x,y)` | 移动鼠标，组件收到连续 `-1..1` 值，宠物窗隐藏时仍持续（`backgroundThrottling:false` 已有） |
| 6 | 几何：`viewport` 决定宠物区尺寸与锚点，气泡条高度按锚点算；删除 `BALLOON_STRIP`/`SIZES` 硬编码 | 换成 96×128 的 sprite 组件，气泡不压头、窗口尺寸随之变化 |
| 7 | 命中：对画布做 alpha 低频采样（100 ms 节流，鼠标移动时才算），或调组件 `hit()` | 点透明区域桌面应用可点；点宠物可摸头；拖拽跟手 |
| 8 | 瞬发排队：FIFO 上限 3，按 `durations` 或 1.2 s 基线判定结束 | 连续触发 4 次 `notify`，宠物只播 3 次且不叠加 |
| 9 | 失败隔离：任一方法抛错 → 记日志、该法降级为兜底，不卸载组件 | 组件 `question()` 内 `throw`，其余动作照常 |
| 10 | 选择持久化：`mascot.buddyId` 落 preferences；启动时按此装载 | 切到新组件后重启壳，仍是新组件 |
| 11 | 主题：`ctx.theme` 与 `data-theme` 双通道 | 切深色，组件 `mount` 读到变更或 CSS 变量生效 |
| 12 | 设置页「宠物」一栏：列出已装组件（名称/作者/版本），支持卸载 | 列表与目录一致；卸载后回到默认 |

## 与现有代码的对应改动

| 现有实现 | 处理 |
| -------- | ---- |
| `electron/src/mascot.ts` 的 `mx-pet` 协议（`modelsRoot()` 指向 `app-dist/models`） | 退役。默认包同样走 `mx-buddy://<id>/<version>/`，协议实现只保留 `mx-buddy` 一套 |
| `BALLOON_STRIP = 112`、`SIZES = { S:67,M:107,L:160 }` | 由 `viewport` + 锚点计算，常量仅作缺省 |
| `app/src/pet/cow.ts` 的 498 行程序动画 | 全量抽成包内 `rig.json` + 通用 `app/src/pet/procedural.ts` 驱动器，代码分件与动作参数一并迁出；`cow.ts` 删除，壳内零 cow 字样 |
| `cow.hitBody/hitHead` 射线检测 | 通用路径改为 alpha 采样；需要精确部位命中的组件可在 `custom` 里实现 `hit()` |
| `POLL_MS = 50` 光标轮询 | 复用，转成 `lookat` |
| 气泡队列（`balloonQueue`，上限 3） | 复用，与瞬发动作队列各自独立（气泡归壳，动作归组件） |
