# TODO

- [ ] daemon 侧 Windows GOOS 死代码专项清理（须先经用户确认范围）：Windows 分发形态 = 壳的 WSL 版本以 rootfs（内含 linux-amd64 二进制）分发，daemon 从不在 Windows 原生运行。疑似的死代码面：internal/setup 的 schtasks/VBS 分支、internal/update 的 extractZip 与 FindAssetForPlatform windows case、runtime 信号处理等 windows 适配——清理前逐处查证壳侧（Windows 版 mindx-work）是否存在间接调用（daemon 在 WSL 内以 linux GOOS 运行，理论上不触达 windows 分支）
- [ ] Homebrew Formula job 在发版失败时也会创建空 body 草稿 release（v2.6.11 首跑实证：draft 仅含 mindx.rb）——建议 release.yml 的 formula job 加 `if: needs.release.result == 'success'` 依赖或改为等 release 产物就绪再跑
