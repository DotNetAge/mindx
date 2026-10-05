# 修复 gort gateway Client.Close 的 close of closed channel panic

日期：2026-10-05
背景：`mindx skill add` 末尾偶发 panic，栈指向 gort@v0.1.6 `gateway.(*Client).Close`（client.go:97）← `readLoop`（client.go:325）。

## 根因

`Close()` 不幂等，且有两条并发调用路径：

1. 调用方 defer（mindx cmd/skill.go 的 `defer cl.Close()`）
2. 库内部 `readLoop()` 退出时的 `defer c.Close()`（client.go:310）

时序：主 goroutine 的 defer Close 先执行 `close(c.done)` 并关闭底层连接 → readLoop 的 `ReadMessage()` 报错退出 → 其 defer Close 再次执行 `close(c.done)` → panic。panic 栈三帧与源码逐行吻合（Close:97 close、readLoop:325 return、Connect:76 go readLoop）。

关键线索：`Client` 结构体第 26 行声明了 `once sync.Once` 但从未使用——写了一半的修复意图，v0.1.5/v0.1.6 代码相同，bug 自始存在。

## 修复（core/gort）

1. **Client.Close 幂等**（pkg/gateway/client.go）：整个清理体（setState、close(done)、清 pending、conn.Close）全部收进 `c.once.Do`，错误经闭包变量返回。注意：`conn.Close()` gorilla 不 panic 但重复关闭返回 "use of closed network connection" 错误，所以不能只把 close(done) 放进 once、conn.Close 留在外面——测试抓出了这个问题。
2. **顺带修复 Server 侧既有数据竞争**（pkg/gateway/server.go）：写回归测试开 `-race` 后暴露——`hubLoop`（原 494/498 行）与 `removeClient`（原 816 行）在 `s.mu.Unlock()` 之后读 `len(s.clients)`，与锁内 map 写构成竞争。修复：统计值在锁内提取为局部变量再使用。

## 验证方法（可复用）

- 回归测试：core/gort/pkg/gateway/client_close_test.go（TestCloseIdempotent 模拟双 Close；TestCloseAfterServerDisconnect 模拟服务端断连后再 Close）
- race 测试是 flaky 的（时序竞争），单次通过不算数，连跑 8 次 `-race` 全绿才稳定
- 端到端：mindx `make restart` 后连续 6 次 `mindx skill add` 无 panic

## 生效路径与遗留

- mindx/go.mod 打开了 `replace github.com/DotNetAge/gort => ../core/gort`（项目开发期惯例，gorag 同款），注释写明"gort 发布 v0.1.7 后移除"，已记入 TODO.md
- 教训：依赖库里的 panic 栈指到模块缓存源码时，直接读 `~/go/pkg/mod/github.com/!dot!net!age/gort@v0.1.6/...`（大写字母转义为 `!x!` 形式）；同名仓库源码在 core/gort，改动要落在仓库源码而非缓存
