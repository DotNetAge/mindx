# TODO

- [ ] 供应商 Key 知情勾选 + clear-creds 修正（2026-10-24 实施完毕，go build/vet/test 全绿）：下次重新打包 mindx 安装后验证——①首启向导出现「发现可一并保存的供应商 Key」确认清单（已存储条目默认不勾，勾选=覆盖新值）；②`make clear-creds` 实删 mindx service 下的 provider 条目（acct 已按 name 动态对准，不再写死引用名）；③钥匙串孤儿 anthropic/google 已手动删除。背景见 .agents/notes（ai-ecosystem）/glm-事实-mindx供应商预设与钥匙串复活机制.md
- [ ] goharness 发布新版本（含 Session.teamName 字段 / WithTeam / WithTeamNameOption / Load 恢复链路，session 包测试已过）后，移除 mindx/go.mod 中 `replace github.com/DotNetAge/goharness => ../core/goharness` 并升级依赖版本（2026-10-05，Team 实体落地引入，见 .agents/notes/trae-decision-一等Team实体落地定案与实现要害.md）
- [x] work 端团队 UI 融合（2026-10-06 用户截图指正：其运行的界面来自 mindx-work 的 AgentsManagerRow 而非 desktop）：团队迷你卡与成员同 strip 混排（「团队」徽标+成员数）、选中团队显示同构配置面板（cfgHead 同骨架+团队档案/成员两页签+只读/编辑双态+team.update 指针语义保存）、新建团队按钮上移至页签行右端、独立 TeamsRecruitRow 行整个删除（组件+装配点），pnpm typecheck+build 通过
- [ ] 三端与 daemon 联调：真实建队 → 选团队会话 → 验证注入段与重载一致性（daemon 冒烟已过，端侧未跑完整流程）
- [ ] runtime 技能存量告警治理（2026-10-18 全量扫描基线：18 技能 0 error，31 条 warning 全为旧类别——soft-rule 12（daily-report 5、deep-review 4、mindx-kv 2、plugin-planner 1、skill-creator 3）、references-frontmatter 缺 updated 17（dashboard 10、agent-creator 4、generate-agents-md 2、user-profile 1）、size 超限 2（agent-creator、skill-creator）；治理路由 retrospect/skill-creator，与 market 发布无关）
