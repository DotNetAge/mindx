---
version: "1.5.1"
pub_date: 2026-10-25
sha: "d7ae75de8a621dbf20a631283015cce00c888ada9a2634d42fc8899668d77a9a"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 Linux 元数据两连修：executableName 定案缩写 mw + deb 署名（glm）

- executableName 二次定案：首修 `mindx` 被用户否决——与 mindx daemon 可执行程序重名，定案缩写 `mw`。electron-builder.yml 注释 + SKILL 坑 11 同步（用户否定=最强信号，重名冲突这个理由当时确实漏想，daemon 名字本就是 mindx）。
- deb 阶段连环报错（AppImage 过后 FpmTarget 接棒爆）：缺 project homepage + maintainer email（deb 强制）。AskUser 定案：maintainer = MSCSharp <csharp2002@hotmail.com>（本机 git 身份，名字取主体去 @），homepage = https://github.com/DotNetAge/mindx-work（remote 对应页面），均写入 electron-builder.yml（顶层 homepage + linux.maintainer）。
- 教训：新增 Linux target 时 metadata 前置自查清单——executableName（不与既有 CLI 重名）、maintainer、homepage 三项缺一必炸；首配就应对公署名问用户，不要等构建报错才补。
