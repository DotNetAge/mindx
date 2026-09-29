# GUI 拉起的 daemon PATH 精简：MCP stdio 子进程 exec 找不到用户级命令

日期：2026-09-29

## 现象

连接器「Excalidraw 画图」测试报 `start process: exec: "npx": executable file not found in $PATH`，
但用户终端里 npx 正常（nvm 安装）。

## 根因（两层，缺一仍复现）

1. **daemon 进程 PATH 精简**：launchd 拉起（`~/.mindx/bin/mindx daemon`，KeepAlive），
   实测 PATH=`/Users/ray/.mindx/bin:/usr/local/bin:/usr/bin:/bin`——nvm/homebrew 等
   用户级运行时不在其中。用户 shell 里的 PATH 与 daemon 无关。
2. **Go exec.Command 构造期 LookPath 只看 daemon 自身 PATH**：给 `cmd.Env` 注入
   增强 PATH 只影响子进程内部查找（如 npx 的 shebang `env node`），
   救不了「npx 本身」的解析——必须先解析出绝对路径再构造。

## 修复（internal/mcp/env_path.go + client.go buildCommand）

- `loginShellPath`：`$SHELL -l -i -c 'echo $PATH'` 探测用户 PATH（进程级缓存
  sync.OnceValue，5s 超时，回退 /bin/zsh → /bin/bash，失败回退 daemon 原 PATH）。
  **坑：`-l -c` 非交互不读 ~/.zshrc**——nvm 普遍只写在交互 rc 里，实测纯净环境
  （`env -i`）下 `-l -c` 拿不到 nvm、`-l -i -c` 能拿到（VSCode 同款交互式登录）。
- `resolveCommand`：裸命令名在增强 PATH 里解析为绝对路径（跳过非绝对段、
  检查可执行位；解析失败返回原名保留 exec 原生错误语义）。
- `buildCommand`：`cmd.Env` 在 os.Environ() 与用户 env 之间插入
  `PATH=enhancedPath()`（append 同名保留最后 → 用户显式配置的 PATH 仍最高优先）。

## 验证

- memory 连接器（`npx -y @modelcontextprotocol/server-memory@latest`）测试通过：
  daemon 日志 `connecting → connected → connection test passed`，全程 1.7s。
- 手动复刻纯净环境探测：`env -i HOME=... SHELL=/bin/zsh /bin/zsh -l -i -c 'echo $PATH'`。
- 重建部署：`make restart`（先编译到 ~/.mindx/bin 再 launchctl kickstart -k，
  KeepAlive 会用新二进制复活）。

## 顺带发现（未处理，需产品决策）

- 连接器预置包名 `@modelcontextprotocol/server-excalidraw` 在 npm **404 不存在**
  （npx 真实运行后的网络报错），PATH 修复救不了它——包名错误需换包。
- 前端手动测连成功无任何通知（store.ts `test()` 成功静默），失败才有红色横幅。
- 前端 `mcp.server.test` RPC 超时上限较短（~15s），npx 冷下载慢的包会误报超时。
