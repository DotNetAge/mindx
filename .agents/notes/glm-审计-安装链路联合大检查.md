# 安装链路联合大审计（mindx + mindx-work）

- 发生时间：2026-11-01
- 依据：先补齐规划资产再审计（用户要求）——mindx-work/docs/安装流程图.html（archify workflow，showcase 9/9）+ mindx-work/docs/安装链路说明.md（file:line 级链路事实 + §9 审计检查单）。
- 审计方式：三个 general-purpose 对抗性审计代理分段（向导链 / 安装器+验证链 / daemon CLI 链），逐行读码取证。注意：Explore 型代理不适合对抗审计——两次只产出结构地图无发现，换 general-purpose 才出真结果。
- 战果：26 条发现（P0×4、P1×11、P2×11），全部修复并验证（go build/vet/test + pnpm typecheck + electron build 全绿，未 commit）。
- P0 四连（都是「装一半必坏」级）：
  1. mindx-work runWsl timeoutMs 可 undefined，wsl.exe 服务损坏时状态机永久卡 detecting/registering（wsl.exe 挂起是已知行为）→ 缺省 60s。
  2. mindx SetupDaemon 用 os.Executable()（源路径）写 plist ProgramArguments，从 Downloads 装完删源即服务坏死 → 改传 destExe。
  3. Linux 注册 mindx-daemon.service 但 Check/Start/Stop 全打 mindx.service——Linux 全链控制落空 → 统一 linuxServiceName。
  4. 【2026-11-02 推翻】Windows 更新资产 zip 一条是伪问题：mindx 不发 Windows 原生产物，Windows daemon = WSL 内 linux 二进制，升级链走 linux 资产。extractZip 的系统 tar 解包保留（防御性，无消费方）。
- 连带发现（已随复验清理，用户确认后执行）：release.yml 旧名 mindx-daemon.service 系统单元投递段已删、scripts/mindx-daemon.service 已删、postinstall.sh 的 systemctl enable/start 段已改指 `mindx install`（deb/rpm 包只投二进制，服务注册统一走用户级 mindx.service）；preremove.sh 保留旧名 stop/disable——升级机器上旧版包曾投递过该系统单元，预删除脚本要负责停掉它。
  - takeCredential 一次性删除的修复把清理时机移到 uninstallProbeIpc 的 credentialValues.clear()——若将来新增常驻通道消费凭据需重新评估。
- 复验（用户挑战「禁止假设正确，必须验证」）后新抓两个修复自身的 bug：
  - probe.ts 审计代理写的 DNS 错误映射是死代码：Node/undici 实测 DNS 失败时 `err.code` 为 undefined，系统错误码全挂 `err.cause.code`（ENOTFOUND/ECONNREFUSED）；超时是原生 TimeoutError（err.code=23、无 cause）。映射已重写并以三种真实网络错误断言 3/3 通过。另：端口 1 等保留端口被 fetch 规范 bad-port 黑名单拦截（cause.message="bad port"），根本不发起连接——拿端口 1 测「连接拒绝」是无效样本。
  - daemon_control.go 修复注册路径 `setupDaemonMacOS(workspaceDir, "")` 直传空 binPath，而空值回落 os.Executable() 的逻辑只在 SetupDaemon 里——plist 会写进空 ProgramArguments。已改传同函数上文已解析的 exePath。教训：拆函数签名时所有调用点要逐一核对「缺省回落」放在哪一层，直调内层函数不会继承外层兜底。
- extractZip tar 方案已本机实证：macOS bsdtar 对 zip 包 `tar -C dest -xf` 解包成功（zip 是 bsdtar 原生格式）；Windows 10+ 自带同族 bsdtar 属文档论断，本机无法实测，已如实标注。
- 经验：
  - wsl.exe 无输出挂起必须逐个调用点核超时；execFileAsync 的 timeout 选项不传就是无界。
  - 服务名/路径这类「注册侧 vs 控制侧」双写常量必须收敛到单一常量（linuxServiceName），双写必漂移。
  - mindx-work 根 typecheck 只覆盖 app/tsconfig，electron 侧要单独 tsc -p tsconfig.json --noEmit。
  - svc 包测试偶发 FAIL 与本机开发 daemon 抢资源相关，复跑即绿——CI 之外判定失败先复跑再查。
  - 沙箱 shell 里 pnpm run 报 `tsc: command not found` 但 `node_modules/.bin/tsc` 实际存在——显式 `PATH="$PWD/node_modules/.bin:$PATH"` 直调即可，不要误判依赖缺失。
