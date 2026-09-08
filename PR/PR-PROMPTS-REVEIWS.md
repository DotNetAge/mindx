# PR: Prompts Review

- [x] 既然我们已经定下了将 System Prompt 的生成完全外放至mindx，在goharness只是通过Hook向System Prompt注入记忆，那么 SkillRegister, RuleRegister 这些注册表还有存在于 GoHarness之中的必要吗？

  **结论（2026-09-06，代码证据核实）：两者处理方式不同。**

  - **SkillRegistry：保留但收窄为只读检索 SPI。** Skill 工具的构造与执行在 goharness runtime 内（runtime.go:295 `tools.NewSkillTool(rt.prompt.skillReg.GetSkill)`），P0 定案即「goharness 只保留 Skill 工具的检索契约」。但 goharness 从不注册技能——所有 `RegisterSkill` 调用方都在 mindx 侧（skillstore、handler_skill_project 的会话 overlay），`DefaultSkillRegistry` 仅剩 runtime 默认空实现（测试脚手架残留）。落地：goharness 接口删除 `RegisterSkill`，只留 `GetSkill`；`DefaultSkillRegistry` 退役；session 的 `skillOverlay` 换用 mindx 侧类型承载注册语义。
  - **RuleRegistry：不保留，整体退役。** rule 包（862 行：类型 + 接口含 `FormatPromptSection` + File/YAML 实现 + 解析）虽住在 goharness，但数据所有权全在 mindx（规则文件在 `settings.DataRulesFile()`、RPC 管理在 handler_rule.go、权限规则合并逻辑在 app.go），且 rule 包在 goharness 内部的唯一消费方是 agents 包的提示词拼接。移入 mindx 后，规则段经 `WithBaseSystemPrompt` 由 mindx 拼装，goharness 的 `WithRuleRegistry` option 与 prompt_assembler 的「扩展规则」段一并删除。

- [x] 承接上述问题，AgentRegister，SkillRegister,RuleRegister 这些注册器既然已经退役，那么我们是否应该将它们移进 mindx 更加合理 ？

  **结论：三者状态各异，不能一刀切。**

  | 注册器        | 现状                                                   | 处置                                                                               |
  | ------------- | ------------------------------------------------------ | ---------------------------------------------------------------------------------- |
  | AgentRegister | P0 已全部退役，goharness 零结构依赖                    | 无需处理                                                                           |
  | SkillRegister | 实现已移完（mindx skillstore 实现 goharness SPI 接口） | 接口留在 goharness 作为接缝（Skill 工具构造在 goharness），按上条收窄为 `GetSkill` |
  | RuleRegister  | 完全未移，rule 包整体借住在 goharness                  | **整体迁入 mindx**（类型 + File/YAML 存储 + 解析），数据所有权与实现归 mindx       |

  迁移顺带修复一个真实缺陷：app.go:644 先注入 `WithRuleRegistry(a.rules)`（用户规则），app.go:709 权限规则存在时再次注入 `WithRuleRegistry(permReg)`——同一 RuntimeConfig 槽位被覆盖，**用户规则被静默丢弃**。规则合并逻辑收口到 mindx 侧经 baseBuilder 输出后，该 bug 自然消解。

- [x] 因此，在 goharness 中都不应该出现与 System Prompt 拼接的工能性代码；

  **结论：方向成立，但需区分两类残留，机制段是 P1 定案的例外。**

  - **「扩展规则」段（`ruleReg.FormatPromptSection`，prompt_assembler.go:52-56）：违规残留，随规则迁移移除。** 这是应用语义（用户自定义行为规则），归属 mindx 的 baseBuilder 拼装路径。
  - **机制段（`defaultBehavioralRules` / `buildOutputEfficiency`）：保留。** PR-PROMPTS P1 职责切分定案「goharness 只保留与运行时机制同步的机制段（行为准则、扩展规则、沟通风格）」——行为准则与沟通风格和 goharness 运行时行为强耦合（工具调用准则等），外放会产生版本漂移风险（goharness 升级机制变化而 mindx 提示词未跟进）。保留机制段不违反本条原则的本意：本条针对的是「应用语义」不得在 goharness 拼接。
  - **Memory hook（executor.go Hook 修改 systemSections）：保留。** 动态注入随轮次变化，与静态拼装是两条路径，PR-PROMPTS 已定案（memory_hook 为动态路径）。
