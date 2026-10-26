---
version: "1.5.1"
pub_date: 2026-10-25
sha: "aa504857d11fa25ba1691f1d286dd2d999895c32cc643d2337b19c2bde9df3e3"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 metadata 落位修正：homepage/author 归 electron/package.json（glm）

- 触发：第四轮构建 schema 校验失败 `unknown property 'homepage'`——electron-builder 26.15.3 不接受 homepage 放 yml 根级；报错原文 "specify project homepage / author email in the application package.json" 指向应用 package.json metadata。maintainer 放 linux 段合法（schema 未报）。
- 修正：homepage + author（MSCSharp <csharp2002@hotmail.com>）+ description 移入 electron/package.json；yml 根级留注释说明落位规则。第五轮构建重启。
- 附带实证（用户问 mac 可执行文件为何近 400M）：Contents/MacOS/MindX 实际仅 68KB 启动壳；.app 总 313M 构成 = Electron Framework 231M（Chromium+Node 固有成本）+ Resources/mindx 双架构 daemon+onnxruntime 61M + app-dist 11M。arm64 包混装 x64 daemon/onnxruntime（~21M）是「双架构都带上」既定决策，可按架构过滤瘦身（未动手，待定案）。
