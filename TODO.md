# TODO

- [ ] gort 发布 v0.1.7（含 gateway Client.Close 幂等修复 + Server hubLoop/removeClient map 竞争修复）后，移除 mindx/go.mod 中 `replace github.com/DotNetAge/gort => ../core/gort`（2026-10-05，见 go.mod 注释与 .agents/notes/glm-fix-gort-gateway-close-panic.md）
- [ ] goharness 发布新版本（含 Session.teamName 字段 / WithTeam / WithTeamNameOption / Load 恢复链路，session 包测试已过）后，移除 mindx/go.mod 中 `replace github.com/DotNetAge/goharness => ../core/goharness` 并升级依赖版本（2026-10-05，Team 实体落地引入，见 .agents/notes/trae-decision-一等Team实体落地定案与实现要害.md）
- [x] work 端团队 UI 融合（2026-10-06 用户截图指正：其运行的界面来自 mindx-work 的 AgentsManagerRow 而非 desktop）：团队迷你卡与成员同 strip 混排（「团队」徽标+成员数）、选中团队显示同构配置面板（cfgHead 同骨架+团队档案/成员两页签+只读/编辑双态+team.update 指针语义保存）、新建团队按钮上移至页签行右端、独立 TeamsRecruitRow 行整个删除（组件+装配点），pnpm typecheck+build 通过
- [ ] 三端与 daemon 联调：真实建队 → 选团队会话 → 验证注入段与重载一致性（daemon 冒烟已过，端侧未跑完整流程）
