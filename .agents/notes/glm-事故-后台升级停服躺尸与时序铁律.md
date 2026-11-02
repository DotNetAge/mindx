# 事故与定案：后台升级先停服导致 daemon 躺尸

## 事故链（全部实证，daemon 日志 ~/.mindx/logs/mindx.log 2026-11-02 11:52-11:53）
1. 用户重开 MindX 壳（11:53:33 启动）→ main.ts 静默调 `ensureDaemonVersionCurrent`
2. 本机 daemon 2.6.10 < 随包锚定 2.6.11 → 触发 `mindx upgrade`（旧时序：**先 StopDaemon 再下载**）
3. CheckDaemon 的 pgrep fallback（`pgrep -f <exePath> + " daemon"`，daemon_control.go:293）命中开发机直跑进程 → 判 running → 停服（11:53:38 shutdown，与壳启动仅差 5s）
4. 下载 GitHub i/o timeout 失败 → daemon 已停无人拉回 → 用户感知「打开壳就把 daemon 关了」
5. 期间 daemon 自身 auto-update 也曾下载失败（11:52:47，无害——进程内升级本不停服）

## 用户定案（升级时序铁律）
- 下载阶段服务照常跑，绝不干扰用户使用
- 停服务是用户的权力，升级进程无权擅自暂停任何服务
- 仅用户确认（点击/交互 y）后才停服 → 秒级替换 → 拉回

## 落地
- mindx：`Updater.Download`（暂存，不碰服务）+ `InstallFromStage`（秒级 rename 替换）拆分，`DownloadAndInstall` 删除；cmd/upgrade.go 新时序 + `--stage`（只暂存）/`--yes`（非交互应用）+ 应用失败自动恢复服务；svc/daemon.go auto-update 与 handler_update.go（server.apply_update）同切新 API
- mindx-work：ensureDaemonVersionCurrent 静默链只跑 `upgrade --stage`；就绪后系统通知一键应用（点击 → `upgrade --yes` → waitDaemonUp → broadcastReconnect）；Windows 分流：docker 形态 stage=只 pull 镜像（不动容器）、apply=rm+run；WSL 形态 guest 内 stage/--yes

## 教训
- 「后台静默自动化」凡涉及停用户服务/杀进程，必须拆成「无干扰准备」+「用户授权应用」两段
- 停了服务就要负责到底：后续任何步骤失败必须自动拉回，否则用户功能裸死
