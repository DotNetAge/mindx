# project list CLI（项目聚合视图）

日期：2026-10-01

## 任务
新增 `mindx project list`：枚举全系统会话 → 按 project_dir 去重聚合，返回项目名 / project_dir / leader / last_agent / last_activity_at / session_count。

## 关键设计共识
- `.sessions` 迁移后枚举全系统会话**依然可行**：`RoutedSessionStore`（pkg/session/routed_store.go）由 `~/.mindx/data/session_dirs.json` 工作目录清单驱动，`ListSessions()` 天然覆盖全部项目分片并集，无需额外索引。
- **leader 映射规则**（用户确认）：Agent 与 project_dir 无直接关联字段，leader 经会话 Sponsor 推断——项目会话的 Sponsor 命中团队负责人候选集（`agentstore.Agent.IsLeader()`，Members 非空派生）即视为该项目已委派负责人；多个命中取最近活跃者；未命中则为空 = 用户亲自 handle。
- last_agent = 项目内 LastActivityAt 最新会话的 Sponsor（空 = 用户）。

## 落点
- RPC：`project.list`（pkg/rpc/project.go + internal/svc/handler_project.go，注册于 handler_registry.go）
- CLI：cmd/project.go（requireDaemon，--json 支持）
- 测试：internal/svc/handler_project_test.go（去重聚合 + leader 派生两用例）

## 经验
- 测试构造多会话时 LastActivityAt 顺序不可假设：连续 Create+Append 的 UpdatedAt 差值可能小于断言所需精度，需 time.Sleep(10ms) 错开且按期望的活跃顺序创建。
- 新增 RPC 方法后，本机运行中的旧 daemon 会报 `Method not found`，需重建并 `mindx restart` 后端到端生效。
