---
version: "2.1.0"
pub_date: 2026-10-23
sha: "588dab76e812a77cc1ceb3913ae0de3548cee19c4e902b38bea61785c234fc9c"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-23 文章读侧（搜索/阅读）能力上线，技能只覆盖发布侧（glm）

- 问题：规则缺失——服务端文章读侧上线（GET 统一详情入口、官网公开搜索、我的/组织搜索），本技能工作流只有发布链路；执行者被问到「搜文章/读文章」时无路由可走，会误判为不适用或绕道其他命令。CLI 同步新增 `market article read <slug>` 与 `market article list` 的 `--site/--org/--query` 三范围搜索。
- 修正：SKILL.md v2.0.0 → 2.1.0——description 补「搜文章、读文章」触发词；新增「权限矩阵」节（官网公开 / 个人仅作者 / 组织仅成员，写在正文让执行者先对号再动手）；新增「读侧：搜索与阅读」路由节；登录检查补「纯读官网文章跳过登录」分支；失败模式补「文章不存在或无权查看」条目（权限信号，不当 bug 重试）；纪律边界命令清单加 `read`。evals/trigger_queries.json 补 4 条读侧正例。
- 验证：`mindx skill validate --path runtime/skills/article-publish` PASS；`go build ./... && go vet ./cmd/ ./pkg/rpc/ ./internal/svc/ ./internal/core/bundle/` 全绿。
