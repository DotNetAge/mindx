# RAG 退役阶段三：embedder Go 链全量清理记录

日期：2026-10-06。RAG 全量退役的第三阶段，删除 embedder（BGE ONNX 嵌入模型）Go 代码全链并清理 go.mod 依赖。rag.seg/graph/entity_tags RPC 与技能目录已于前两阶段删除。

## 本次删除清单（供追溯）

- internal/core：app.go 的 embedder 字段/兜底补写段/构造段/Embedder() 访问器/goragcore import；embedder.go 整文件；config.go 的 EmbedderModel 字段、EmbedderModelPath()、HasEmbedder()；app_test.go 对应断言
- internal/svc：handler_user.go 的 embedder_model 返回字段
- internal/setup：wizard.go（downloadProgressMsg/downloadObserver/runEmbedderDownload/Embedder 任务条目等约 177 行）、setup.go、data/data.go、component/memoryconfig/ 整目录、examples/memoryconfig/ 整目录、examples/Makefile 三个提及点
- cmd：doctor.go 第 5 项检查（6/7 顺位前移）、status.go 三处
- docs/AGENTS-SKILLS.md：第三节 memory_hook 动态注入相关全部改写（3.1 注入条目、3.2 Hook 段、3.3 动态区、3.4 示例历史摘要段、3.5 ⑦ 条目），MemorySearch 引用一并清除
- go.mod：gograph v0.4.0、gorag/v2 v2.0.21 删除后 go mod tidy，govector indirect 自动消失，gort v0.1.8 保留

## 教训一：删 handler 必须同步删测试段（本次最大发现）

阶段二删除 handler_graph.go 时漏删了 handler_test.go 中约 250 行 graph 测试段（5 个测试函数 + 注释头）。`go build ./...` 不编译测试代码所以静默通过，直到本次 `go vet ./...` 才暴露。已记入 mindx/AGENTS.md 必须禁止区：完成门槛必须是 vet 而非仅 build。mustJSON helper 因被其他测试文件使用而保留——删除前先 grep 引用面。

## 教训二：wizard autoTasks 硬索引修正法

删除 autoTasks 中间条目（原 [2] embedder）后，尾部条目 [3] path 前移为 [2]。硬索引共 4 处消费点需同步：runFirstRunWizard 结果提取、startAutoSetup 的 `}(3)` 起始索引与 `taskCount > 3` 守卫、renderComplete PATH 段。核对方法：`grep -n "autoTasks\[" wizard.go` 逐处列出，确认 [0]=daemon、[1]=python、[2]=path 全部连续。

## 遗留事实（下次清理线索）

- i18n 死键仍在：`wizard.autosetup.task.embedder`、`wizard.complete.item.embedder.*`、`setup.memory.*`（downloading/download.progress/complete/model.open.failed 等）位于 internal/setup/msg 下非 Go 文件，超出本次 *.go grep 范围，留待 i18n 梳理时清理
- runtime/mindx.json 若用户目录存在历史 embedder_model 字段，加载后因结构体无此字段被忽略，无害，不需迁移
- daemon.go 的 gofmt 对齐残留（阶段二删 graphStore 字段后未跑 gofmt）已顺手修复
