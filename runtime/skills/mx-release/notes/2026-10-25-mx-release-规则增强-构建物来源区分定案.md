---
version: "1.4.0"
pub_date: 2026-10-25
sha: "d6036debe8b5c7badd2dbba257f229d734816978c02a5ee0362ef8aab740b97e"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 构建物来源区分定案：mindx 一律取自 GitHub，本机零构建（glm）

- 触发：用户定案补齐架构区分——mindx-work（App）本机构建直发；**mindx（daemon）由 mindx 仓库 GitHub CI 构建**，App 打包只从其 GitHub Release 下载，不在本机重新构建；Windows WSL rootfs 直接 `docker pull dotnetage/mindx:latest`（docker.hub 最新镜像），本机不构建镜像。
- 实证基线：build-wsl-rootfs.mjs 现状已是 docker pull（无 gap）；pack.mjs L79/L90 调用均不带 --local；唯一 gap = build-mindx-bundle.mjs 的 `--local` 本地构建模式（go build 旁路，~60 行）。
- 修正：删除 build-mindx-bundle.mjs 的 --local 模式（buildMindxLocal 函数、LOCAL_MODE、MINDX_REPO、头注释更新为「本机零构建」定案）；SKILL.md 新增「构建物来源区分」节。version 1.3.1 → 1.4.0。
- 保留说明：daemon 下载保留**版本锚 = mindx 仓库 git tag**（非 floating latest）——「禁止运行时 latest」是既有用户定案（防 2.5.6/2.5.3 错配事故）；rootfs 用 latest 是本次定案明确选择，两者取舍已写入 SKILL。若用户意图是 daemon 也改 floating latest，另行定案再改。
- 验证：`node --check` 过；grep 无 local/MINDX_REPO 残留；`mindx skill validate` 全绿。
