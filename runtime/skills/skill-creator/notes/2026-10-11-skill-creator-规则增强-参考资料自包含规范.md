## 2026-10-11 新增「参考资料自包含」原则（skill-creator）

- 问题：规则缺失——原规范只有"渐进式披露"（深度参考材料放 references/ 按需加载），未强制"资料必须预置、禁止运行时外搜"。实践中技能若不自带资料，LLM 要么凭记忆猜（写错），要么现场去 Web 搜——浪费运行次数与时间，搜回来的资料还可能与本技能方法论不匹配（用户长期使用实践经验，明确要求写入本技能）。
- 修正：SKILL.md 核心原则新增「### 参考资料自包含（references 收纳）」节（资料全部收纳 / 禁止运行时外搜 / 引来源不引搜索 / 核验后收纳），「写作风格-渐进式披露」条强化为"一律预置 references/，禁止运行时自行搜索"；references/best-practices.md 新增「### 参考资料自包含（references 收纳）」专节（含索引表示例与要点）。version 1.3.0 → 1.4.0。
- 验证：`mindx skill validate --path mindx/runtime/skills/skill-creator` 通过；正本与 `~/.mindx/skills/skill-creator/` 副本同步且 md5 一致。
