# internal/mcp

- daemon 由 launchd 拉起，进程 PATH 精简（仅 /usr/bin:/bin 级别）：所有 stdio 子进程
  exec 必须走 `resolveCommand`（增强 PATH 解析绝对路径）+ `cmd.Env` 注入
  `PATH=enhancedPath()`（client.go buildCommand 已统一处理，新增启动路径勿绕过）。
  直接 `exec.Command("npx")` 会报 executable file not found——Go 构造期 LookPath
  只看 daemon 自身 PATH，cmd.Env 不参与。
- 用户 PATH 探测用 `$SHELL -l -i -c`（交互式登录）：nvm 等版本管理器普遍只写在
  交互 rc（~/.zshrc），纯 `-l -c` 拿不到。
