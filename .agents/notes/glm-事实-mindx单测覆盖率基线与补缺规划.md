# mindx 单元测试覆盖率基线与补缺规划

- 分类：事实 / 待办线索
- 日期：2026-11-15

## 基线数据（go test -count=1 -cover 全量实测）

- 整体不均衡：pkg/metric 81.7%、pkg/rules 78.3%、internal/tools 58.3% 尚可；internal/svc（daemon 服务层）仅 17.9%，是最大缺口。
- 关键 0% 文件：pkg/scheduler/scheduler.go（调度器本体循环）、internal/svc/scheduled_exec.go（new 哨兵兜底层）、handler_token_usage.go（707 行）、handler_online_models.go（995 行）、internal/mcp manager.go/handler.go/client.go（6.2%）、internal/core workspace.go（12.5%，ExtractWorkspace/SyncRuntimeAssets）。
- 域内相对健康：devgate 74.3%、skillstore 60%+、bundle 核心 83.9%、credential_store 86.2%。

## 方法论

- 不能按「文件名配对 _test.go」判断缺测：本项目大量测试不以被测文件命名（handler_test.go 覆盖多个 handler），会误判。正确做法：`go test -coverprofile` 后 `go tool cover -func` 按函数聚合，再按包/文件求均值。
- pkg/rpc、internal/client/rpc.go 大量 0% 是声明式封装（方法表），TUI bubbletea 组件、setup wizard、cmd/ 薄壳属低 ROI，规划内明确不强补。

## 存量发现

- TestHandleSessionList_FilterByAgent 全量跑 internal/svc 时失败、`-run` 单跑通过：测试间共享状态污染，已记 TODO 修复项。

## 下次我看到「补单测 / 覆盖率」类任务，我将

先读本文基线与 TODO.md 单测分区，直接从 P0（调度链 + 安全闸 + 信件链）开工，不重新全量扫描；仅在做完一批后重跑 coverprofile 对照增量。
