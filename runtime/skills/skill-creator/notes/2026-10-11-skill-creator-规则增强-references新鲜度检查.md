## 2026-10-11 新增「references 新鲜度检查」（skill-creator）

- 问题：规则缺失——技能进化审查（步骤 C）覆盖引文断裂（refcheck 只查"引用是否存在"），但没有 references 内容新鲜度检查；领域知识持续变化（引擎 CLI/API/包名/标准），陈旧 references 会让技能持续"腐烂"，且"本地优先"策略使过时更难暴露（LLM 默认读本地旧资料不触发警告）。用户指出这是关键治理缺口。
- 修正：步骤 C 检查清单新增「references 新鲜度」条目（核验日期/URL 可达/版本对照/超期标待复核/过时更新或移除）；best-practices「参考资料自包含」节新增两小节——「references 元信息头」（核验日期/适用范围/来源，无日期无法判断新鲜度）与「references 新鲜度检查」（五步操作细则 + 超期阈值 + 与 refcheck 的区分）。version 1.4.1 → 1.4.2。
- 验证：`mindx skill validate --path mindx/runtime/skills/skill-creator` 通过（存量 issue 不计）；game 引擎 4 技能 references 已补元信息头（核验日期 2026-10-11）；正本与 `~/.mindx/skills/skill-creator/` 副本同步且 md5 一致。
