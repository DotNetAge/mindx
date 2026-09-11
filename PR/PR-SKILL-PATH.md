# 产品技能路径

- [x] 当前是否支持从Agent目录下加载并注册Skill呢？

  **支持，且无需显式注册——技能放入 `agents/<name>/skills/` 即生效。** 三层机制：
  - 提示词目录：`Store.Catalog`（internal/core/skillstore/store.go:220）规定「Agent 级技能库中的技能自动入选」系统提示词技能目录；`RegistryFor`（store.go:163）按「先全局、后 Agent 级」组装运行时注册表，同名以 Agent 级为准（逻辑重写）。
  - 运行时检索：`LiveRegistry.GetSkill`（live_registry.go:33）对 Agent 级库**实时读盘**，不建内存快照，增删改即刻生效。
  - 注入点：Runtime 创建时经 `WithSkillRegistry(a.skills.LiveRegistryFor(agentName))` 注入检索 SPI（internal/core/app.go:727）。

- [x] 从市场安装的Agent是否会将Skill释放至  `agents/<agent-name>/skills/` 内？

  **部分会，不是全部。** 规则见 `bundle.Install`（internal/core/bundle/install.go:97-116）：包内技能与全局库**同名** → 落 `agents/<name>/skills/`（Agent 级覆盖 = 逻辑重写）；**不同名** → 直接进全局库 `~/.mindx/skills/`，Agent 的 skills 引用列表天然指向全局版本。该规则消除了安装时的同名写冲突（install.go:66-67 注释）。

- [x] 如果当前工作目录下 `<ProjectDir>/.skills` 目录存在将有Skill是否会自动从这个目录加载并注册Skill呢?

  **不会自动注册，设计为「发现式扫描 + 用户确认挂载」。**
  - `DiscoverProject`（promote.go:58）仅扫描装载返回结果，**不写入任何内存注册表**；
  - 须经 `skill.project_load` RPC（handler_skill_project.go:128）用户批量确认后，挂载为**会话级覆盖注册表**（仅当前会话可见，不持久化，daemon 重启后需重新确认，见 handler_skill_project.go:125-127 注释）；
  - 挂载后生效路径：Skill 工具执行时会话 overlay 优先（goharness/tools/skill_tool.go:84-89）；提示词目录同步并入（internal/core/prompt_builder.go:183-194）。
  - 依据：PR-PROMPTS 三级库语义——项目级是「发现式的可能经验」，入正式注册表须用户裁决（晋升管线）。

- [x] 是不是只要在注册Skill指定路径后，SkillTool就会自动定位Skill所在的目录？

  **结论成立，但机制不是「注册时指定路径」。** SkillTool 注册时只注入**按名检索函数**：`NewSkillTool(lookup)`（goharness/tools/skill_tool.go:39），lookup 由 mindx 侧提供（即上面的 LiveRegistry）。执行时按 name 走检索链：会话覆盖（项目技能）→ Agent 级实时读盘（`agents/<name>/skills/<skill>/SKILL.md`）→ 全局库注册表；命中后返回 `root_dir`（skill_tool.go:100），模型用 Read 访问该目录下的 references/scripts。
  即：**目录定位由检索链按技能名解析**（RootDir 在 `LoadSkillFromDir` 装载时确定），注册方无需也不能指定路径——对模型与工具的体验确实是「自动定位」。
