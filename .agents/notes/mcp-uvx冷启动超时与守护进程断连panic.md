# uvx 类 MCP 连不上：两个真凶与修法

## 现象

`mindx mcp tools <uvx 服务>` 报 `Error: context deadline exceeded`，紧接着下一次调用报
`websocket connect failed: dial tcp [::1]:1314: connect: connection refused`。
而同一台机器上 `npx` 类的 `filesystem` 一切正常，`mcp list/enable/disable` 也正常。

## 真凶一：客户端超时断开 → 网关 panic → 整个 daemon 死掉

`gort@v0.1.10/pkg/gateway/server.go` 的 `sendToClient` 直接 `c.send <- data`，
而 `removeClient` 会 `close(c.send)`。客户端超时断开后，handler 协程回写结果时正好撞上已关闭的
channel，触发 `panic: send on closed channel`（server.go:947，由 handleRequest.func1 触发）。
一个连接器的超时能带走整个 daemon，其他连接器跟着一起不可用——这就是「装上了但跑不起来」的放大器。

修法（third_party/gort 本地补丁 + go.mod replace）：给 client 加 `sendMu sync.RWMutex` 与
`sendClosed bool`，`sendToClient` 持读锁期间写入，`removeClient` 持写锁置位后再 close。通道关闭后
写入变成丢弃 + debug 日志，而不是 panic。

## 真凶二：uvx 冷启动解析依赖，直连 pypi.org 在国内长时间挂住

`uvx markitdown-mcp` 在用户 shell 里 39 秒能起（shell 里有 `UV_DEFAULT_INDEX` 指向国内镜像）；
daemon 由 GUI 拉起，环境里没有这个变量，uv 走 pypi.org 解析依赖，`initialize` 迟迟不回。
补救顺序：先补镜像变量，8 秒响应急降到 8 秒内；再加大超时。

两个配套改动：
- `internal/mcp/client.go` 的 `buildCommand` 对 `uv`/`uvx` 注入默认 `UV_DEFAULT_INDEX`
  （用户已配 `UV_INDEX_URL`/`UV_DEFAULT_INDEX` 或 mcp.json env 时不动；可用 `MINDX_UV_DEFAULT_INDEX` 换源）。
- `mindx mcp tools` 补 `--timeout`（默认 180 秒）。原实现走 `CallWithTimeout` 的 30 秒默认值，
  首次冷启动必然误报成超时。

## 判定要点（下次遇到别猜）

1. 先看是不是 uvx 类：`npx` 能过、`uvx` 不能过，先怀疑环境与源，不要怀疑协议。
2. 超时后 daemon 是否还活着：`context deadline` 后面紧跟 `connection refused` 就是 panic 链路，
   去 `~/.mindx/logs/daemon.err.log` 找 `panic: send on closed channel`。
3. 手动跑一次裸进程 + 手写 initialize 行，能量出真实启动耗时；别用 `timeout`（macOS 没有该命令）。

## 验证

`mindx mcp tools markitdown` 8 秒返回工具清单；`run-mcp` 技能
`python3 scripts/mcp.py run markitdown convert_to_markdown --args-json '{"uri":"file:///tmp/mcp-e2e/sample.md"}'`
退出码 0 并返回中文 Markdown。注意 markitdown 的参数名是 `uri`（file:// 形式），不是 `path`。
