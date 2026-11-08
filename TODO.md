# TODO

- [ ] 壳侧采纳 terminal.start 的 session_id 参数：daemon 已支持 cwd 解析顺序 = 显式 cwd > 会话 project_dir > daemon cwd，但壳不传 session_id 时仍落 daemon 进程 cwd（launchd 下通常是 /）——mindx-work / mindx-desktop 打开终端时带上当前会话 ID
- [ ] 会话标题提炼 + 轮词元统计已落地待实机验收：`make restart`（吃进 goharness v0.2.54 的 Usage 回填）后 ①新建会话发首问看侧栏标题为提炼标题；②跑一次未绑定会话的调度任务看任务会话标题；③对话含工具调用的轮次末尾出现词元统计行；④重新加载该历史会话，统计行仍在（恢复链路读 token_usage）。注意 make restart 会中断运行中的会话，挑空闲时机执行
- [ ] 会话绑定 agent 抢答修复待实机验收（与上条共用一次 make restart）：user.message 新增 agent_name 载荷（mindx-work 发送时携带输入框选定 agent），daemon 按「@目标 > 载荷 agent_name > 会话绑定 agent_name > 全局当前 agent」裁定执行者。验收：①与大秘聊完，AgentSelector 切到从未聊过的梁工直接发消息，应答者必须是梁工；②选中梁工会话内继续聊，应答者仍为梁工
- [ ] daemon 侧 Windows GOOS 死代码专项清理（须先经用户确认范围）：Windows 分发形态 = 壳的 WSL 版本以 rootfs（内含 linux-amd64 二进制）分发，daemon 从不在 Windows 原生运行。疑似的死代码面：internal/setup 的 schtasks/VBS 分支、internal/update 的 extractZip 与 FindAssetForPlatform windows case、runtime 信号处理等 windows 适配——清理前逐处查证壳侧（Windows 版 mindx-work）是否存在间接调用（daemon 在 WSL 内以 linux GOOS 运行，理论上不触达 windows 分支）
- [ ] 发布工作流 Homebrew 公式修复待实证：homebrew job 已改 `needs: [prepare, build, release]`，校验和步骤加 `set -euo pipefail` + 非空断言（v2.6.17 起 tap 公式 `sha256 ""`，brew 装不上）。验证方式二选一：下次 tag 发版，或 `workflow_dispatch` 指定 v2.6.21 跑一次，核对 tap 仓库 `Formula/mindx.rb` 的 sha256 与 Release `checksums.txt` 一致
