# 技能双副本纪律落地（glm）

## 2026-10-25 规范 §12 双副本纪律空转（glm）

- 问题：流程漏洞——规范 §12 要求 mindx/runtime/skills 与 mindx-work/.agents/skills 两份技能副本一致，实际 work 侧副本从未存在（notes 治理后目录整个消失），所有修改只改了一份。
- 修正：整目录补齐 work 侧副本（SKILL.md + references/ + scripts/ + notes/）。
- 验证：diff 全 SYNC-OK。下次改 runtime 侧技能，我将同步 cp 到 work 侧同名技能并 diff 验证——双副本纪律靠每次修改时执行，无人执行就等于不存在。
