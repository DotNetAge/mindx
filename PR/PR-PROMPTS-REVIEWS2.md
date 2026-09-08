# PR:PROMPTS-REVIEWS2


- [ ] 既然将 Prompt的拼接退役，那么 goharness 中的 agents/base_rules.go 为何还会存在，你不理解我在PR中说明 用一个 Agents.md 又或 AgentRules.go 替代并置于 mindx 中的原意吗？
- [ ] 由此推断，你当前的 System Prompt 拼接逻辑很可能存在重大问题，首先 runtime / agents 目录中的 agent 规则已经应该改成 基于agent名称的目录，即在PR中的声明，而 AgentRegister又是如何加载 IDENTITY.md, SOULD.md 等文件的，又是如何将这些文件的内容读入至Agent ?
- [ ] System Prompt 的拼接顺序是怎么样的，请详细说明，带过程，图例以及最终效果模拟；