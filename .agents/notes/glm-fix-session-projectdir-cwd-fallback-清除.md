# session project_dir 为空时的 Getwd 兜底是「会话落进 ~/.mindx」的元凶，已全部拔除

日期：2026-11-01

## 现象（用户报告，全新 macOS 首装复现）

选完工作区后 ProjectDir 被加了前缀，会话落进 ~/.mindx 而非所选目录。

## 排查结论（全链路实证，非猜测）

- **daemon RPC 与客户端均不装饰路径**：`mindx session create --project-dir /tmp/mindx-probe-20261101` 实测返回的 `project_dir`/`session_dir` 原样落在测试目录，`~/.mindx` 下零分片（实验后 session.delete + 清单恢复 `[]`）。
- chatflow 侧唯一 `session.create` 调用点 store.ts L1179，`currentProjectDir` 只来自服务端值或 `fs.choose_dir` 返回，无 localStorage、无拼接；`fs.choose_dir`（osascript）返回绝对路径原样透传。
- **真正的伪造源是 daemon 侧 5 处 `os.Getwd()` 兜底**：daemon cwd = ~/.mindx（launchd 拉起），任何空 project_dir 走到兜底即把会话/元数据/提示词环境段悄悄锚到数据目录。routed_store.Create 已拒绝空值，但兜底点在它上游/旁路，把它绕过去了。

## 修复（5 处，全部改为空值报错或如实留空）

- `internal/core/app.go` CreateSession：空 project_dir → 报错（原 Getwd 兜底）。
- `internal/core/app.go` ClearCurrentSession：旧会话无 project_dir → 报错（原 Getwd 兜底）。
- `pkg/session/file_store.go` Create：空 ProjectDir → 报错（原 Getwd 兜底；file_store_test 唯一依赖点已补 WithProjectDirOption）。
- `pkg/session/file_store.go` GetMeta 读路径：meta.json 缺失时不再用 cwd 填 ProjectDir，留空如实。
- `internal/core/prompt_builder.go` buildEnvSection：空 ProjectDir 不再用 cwd 伪造「项目目录」行。

## 保留的 Getwd（合法 cwd 语义，勿误伤）

- `EnsureSession` / `/chat new` / client.go rootModel / cmd/*：CLI/TUI 进程 cwd 就是用户终端工作目录，属设计语义；daemon 进程里这些路径不会被触发。

## 验证

go build ./... 全绿；go test pkg/session + internal/core + internal/svc 全过。

## 经验

- 「空值兜底」在多进程架构下是跨边界污染：客户端的 cwd 兜底语义（CLI）搬到 daemon 进程里就变成把 ~/.mindx 当用户工作区。判据是**进程身份**，不是代码形状。
- 实证 RPC 行为直接用 CLI 子命令（`mindx session create/delete/meta`）即可，不必手写 ws 客户端；实验产物（临时目录 + session_dirs.json 条目）用完必须恢复。
