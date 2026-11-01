# 实证：mindx 内核与 mindx-work 壳能否独立自更新（2026-11-01）

用户挑战「更新链是真实正确还是假设正确」，逐环实测两仓自动更新链。

## 用户定案与修复（2026-11-01 当轮完成）

- 内核 Windows 资产缺口已修：release.yml 加 windows/amd64 构建目标（产出 mindx-<v>-windows-amd64.zip 进 checksums.txt），下次 mindx 发版生效；验收 = 新 release 资产含该 zip 且 checksums.txt 覆盖。
- 壳 win/linux 走私有源码库 CI 补发（用户定案：源码库 GitHub Actions 打包后直发 mindx-app，mac 本机直发不变）：已落地 .github/workflows/release-win-linux.yml。要点：WSL rootfs 须在 ubuntu job 生成（windows runner 无 linux 容器）；release 未建轮询等待 10 分钟（mac 直发与 CI 的时序竞态）；跨库上传要 MINDX_APP_TOKEN secret（用户待配）。
- 壳 nsis 无签名会被 SmartScreen 拦——与 mac ad-hoc 分发限制同类，已记 mindx-work TODO 改进项。

## 实证结论

### mindx 内核（mindx upgrade 链）——macOS/Linux 真实，Windows 断裂

| 环节 | 实证方式 | 结果 |
|---|---|---|
| 发现+版本比对 | GitHub API 实测 v2.6.10 真实存在 | 命名 `mindx-2.6.10-darwin-arm64.tar.gz` 与 FindAssetForPlatform 规则（suffix `{os}-{arch}.tar.gz` + 含 `-{version}-`）逐字吻合 ✓ |
| checksum | checksums.txt 资产实测存在；CI 用 `shasum -a 256` 生成 | 格式 `<hash>  <asset>` 与 parseChecksumFile（fields[1]==assetName → fields[0]）吻合 ✓ |
| 解包 | release.yml :175 tar 根目录单文件 mindx | 与 extractTarGz 定位（mindx/mindx.exe）吻合；zip 解包此前已实测 bsdtar ✓ |
| 替换 | 实测 `mv $TMPDIR/x ~/.mindx/bin/x` | rename 成功（macOS TMPDIR 与 ~/.mindx 同在 Data 卷）；TMPDIR 被改到别卷的场景会 EXDEV，兜底为恢复旧二进制 |
| **Windows 发现** | 实测 v2.6.10 资产清单 | **无任何 windows-*.zip——release.yml 构建矩阵只有 linux/darwin，Windows daemon 升级在 find asset 一步必败**（P0） |
| 下载可达性 | objects.githubusercontent.com 本环境超时 | 无法确证；大陆网络环境本身就是风险点 |
| 端到端 | 未实跑（会动真机 daemon） | 仅静态+分段实证 |

### mindx-work 壳（electron-updater 链）——macOS 真实，Windows/Linux 断供

| 环节 | 实证方式 | 结果 |
|---|---|---|
| 发布 feed | mindx-app 9 个 release 全量扫描 | v2.6.7 起只有 latest-mac.yml；**v2.6.6 曾发过三平台全套（latest.yml/latest-linux.yml/exe/AppImage/deb），此后 8 个版本全部缺 win/linux feed** |
| 发布脚本 | release.mjs :81 只打当前平台包（Mac 上发版=只有 mac 资产） | 本机直发模式天然无法出三平台产物 |
| 公证防线 | release.mjs :73-76 缺公证凭据即中止 | 已发布构建为公证版 → 自动引擎启用 ✓ |
| mac 自动链 | feed 三元组（latest-mac.yml+zip+blockmap）实测在库 | 真实可用（端到端未实跑） |
| dev/ad-hoc 手动链 | /releases/latest API 实测可达 | 真实 ✓ |

## 教训

- 「发布存档区」型发布（本机直发）容易被默认成「全平台可用」——实际只有发版那台机器的平台有产物；多平台必须 CI 或多机。
- 验证更新链不能只看代码：代码读三遍不如一次 release 资产清单实测；FindAssetForPlatform 这种命名耦合要在 CI 产物名变化时同步改。
- 心智模型纠偏：v2.6.6 全平台是历史孤例，不能当作「发版流程支持全平台」的证据。
