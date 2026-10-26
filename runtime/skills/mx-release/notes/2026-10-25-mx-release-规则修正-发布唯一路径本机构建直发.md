---
version: "1.2.0"
pub_date: 2026-10-25
sha: "78c2f98ea4556336c261cb16951f3ced68630719b8a9f9b9957abbe642103f3f"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 发布定案推翻 CI 路径：唯一路径 = 本机构建 + 本机直发（glm）

- 问题：指令冲突——上一节按 release.yml 把「CI 打包」（三平台矩阵 + secrets 表）写成发布工作流之一，用户定案推翻：**mindx-work 发布永远只走本机构建 + 本机直发，GitHub Releases 仅是发布存档区（electron-updater 更新源 + 产物存档），不存在 GitHub CI 发布路径**。release.yml 属误导性遗留。
- 否决：保留 release.yml 并标注「非发布路径」——用户拍板直接删除文件，防未来 agent 再误判（git 历史可找回）。
- 修正：SKILL.md 删「CI 打包」段与 secrets 表，新增「发布定案」节，「本机直发」升为唯一发版动作，description 去 CI 触发词；AGENTS.md 军规去「CI tag 触发」；electron-builder.yml publish 段与 pack.mjs MX_PUBLISH 注释同步（开关无消费者，仅历史兼容保留）。version 1.1.0 → 1.2.0。
- 验证：`mindx skill validate --path` 全绿；全仓 grep release.yml 复核——electron/scripts/build-mindx-bundle.mjs 的引用指 mindx 仓库（daemon）自身 CI，与本项目无关；notes/TODO 中引用均为历史记录不改。
