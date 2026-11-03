# TODO

- [ ] 壳侧采纳 terminal.start 的 session_id 参数：daemon 已支持 cwd 解析顺序 = 显式 cwd > 会话 project_dir > daemon cwd，但壳不传 session_id 时仍落 daemon 进程 cwd（launchd 下通常是 /）——mindx-work / mindx-desktop 打开终端时带上当前会话 ID
- [ ] 会话标题提炼 + 轮词元统计已落地待实机验收：`make restart`（吃进 goharness v0.2.54 的 Usage 回填）后 ①新建会话发首问看侧栏标题为提炼标题；②跑一次未绑定会话的调度任务看任务会话标题；③对话含工具调用的轮次末尾出现词元统计行；④重新加载该历史会话，统计行仍在（恢复链路读 token_usage）。注意 make restart 会中断运行中的会话，挑空闲时机执行
- [ ] daemon 侧 Windows GOOS 死代码专项清理（须先经用户确认范围）：Windows 分发形态 = 壳的 WSL 版本以 rootfs（内含 linux-amd64 二进制）分发，daemon 从不在 Windows 原生运行。疑似的死代码面：internal/setup 的 schtasks/VBS 分支、internal/update 的 extractZip 与 FindAssetForPlatform windows case、runtime 信号处理等 windows 适配——清理前逐处查证壳侧（Windows 版 mindx-work）是否存在间接调用（daemon 在 WSL 内以 linux GOOS 运行，理论上不触达 windows 分支）
- [ ] Homebrew job 的 fallback 步在 release 不存在时会自建残缺 release（v2.6.11 首跑实证：step 日志 "Creating new GitHub release"，产物 draft 仅含 mindx.rb、空正文；成功路径被 Release job 就地补齐掩盖）。修法待定：fallback 步限 workflow_dispatch 模式执行，或 homebrew job 加对 release job 成功的依赖
