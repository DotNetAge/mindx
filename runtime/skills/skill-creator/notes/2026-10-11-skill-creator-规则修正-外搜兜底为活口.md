## 2026-10-11 修正「外搜兜底」——外搜是活口不是禁令（skill-creator）

- 问题：指令冲突——1.4.0 写入的「参考资料自包含」写死"禁止运行时外搜"，用户纠正：外搜是"活口"，不能禁死；仅当本地参考资料不足或过时（资料缺失、版本失效、规范更新）时才允许外搜，并标注补到了什么。本地资料永远可能过时，堵死外搜等于自断退路。
- 修正：SKILL.md 核心原则「本地优先，外搜兜底」+ 渐进式披露条同步；best-practices.md 专节要点改为"本地优先，外搜兜底"并加"禁止用'禁止外搜'堵死退路"。version 1.4.0 → 1.4.1。
- 验证：`mindx skill validate --path mindx/runtime/skills/skill-creator` 通过（存量 issue 不计）；正本与 `~/.mindx/skills/skill-creator/` 副本同步且 md5 一致。
