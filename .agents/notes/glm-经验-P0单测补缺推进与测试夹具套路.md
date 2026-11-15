# mindx P0 单测补缺推进记录（调度链/设备闸/信件链/token 用量）

- 分类：经验 / 事实
- 日期：2026-11-15

## 成果

- svc 包 17.9% → 25.2%；pkg/scheduler 85.9%。目标文件：scheduler.go 92.9%、handler_token_usage.go 83.6%、handler_mail.go 61.9%、handler_device.go 49.4%、mail_dispatcher.go 30%、scheduled_exec.go 32.9%。

## 偶发测试失败根因（高价值）

- 现象：svc 包全量跑时随机 `TempDir RemoveAll cleanup: directory not empty`，失败测试每次不同。
- 根因：NewDaemon 构造期起了一个无生命周期管控的协程做 MCP 远程同步（20s 超时窗口），拉真网 + 重写测试临时目录里的 mcp.json，与 t.TempDir 清理竞争。
- 修法：Daemon 加 `bgCancel context.CancelFunc` + `bgWG sync.WaitGroup`，构造期协程纳入管控，stopBackgroundServices 先 cancel 再 Wait 收敛。已验证连跑三次全绿。
- 下次我看到「测试偶发 directory not empty / TempDir cleanup 失败」，我将先查被测构造函数里有没有无 WaitGroup 管控的写文件后台协程。

## 测试夹具套路（svc 包）

- `newTestDaemon(t)` 已隔离 HOME（t.Setenv）与全部数据目录；`mustCreateSession`、`loadTestSession`、`appendRound`、`mustJSON` 是现成夹具。
- 设备闸测试：真实 CredentialStore 写路径在无头测试环境会阻塞在钥匙串交互上——凡触 SetAllowed/handshake 持久化，必须用 `swapDeviceGateWithMemStore` 换内存桩（fake 只需 Get/Set 两方法）。
- 模拟本机直通连接：`d.loadOrCreateDaemonToken()` 拿令牌 → `d.devGate.Handshake(clientID, ..., token, "", nil)` → `gateway.WithClientID(ctx, clientID)`。LocalVerified/Authorized 即成立。
- goharness agents 的 PermissionSink/AskSink 只有无 ctx setter 无 getter，sink 本体不可从外部触发断言，只能测登记消费点（markSessionBlock → sessionBlockOf）。
- goharness Message.Timestamp 语义分叉：anchorQuestion 按「秒级 Unix」设计，而 reclaim 测试用 UnixMilli——两处口径不一致，可能是潜在 bug，未验证（待办）。

## 覆盖率方法论（重申）

- 按文件名配对 _test.go 会误判；用 `go test -coverprofile` + `go tool cover -func` 按函数聚合才是实况。
- 网络/钥匙串/mDNS 边界在单测层只能覆盖闸门与校验分支，正向全链路挂集成夹具（已记 TODO）。

## P2 低垂果实结果（2026-11-15 续推）

- diffutil 100%、i18n 91.9%、pkg/scheduler 87.5%、commands 74.8%、tools 包 59.2%（dynamic.go 本体 100%，剩余为其他工具文件，P1 范围）、mcp 包 21.8%（env_path.go 本体除 enhancedPath 空 PATH 回退分支外全覆盖，剩余是 P1 的 manager/handler 等）。
- commands 上限卡在 RespondWithType/ctx.Server()：gort gateway.Server 的 server 字段不导出，无 WS 客户端就无法构造带 server 的 CommandContext——成功路径只能挂集成夹具。
- 顺手修了一个真 bug：LanguageName 对 zh-Hant（language.TraditionalChinese 的字符串形式）漏判，设置页语言列表繁体中文显示成「简体中文」；修法是补 zh-Hant/zh-MO 前缀判断。教训：golang.org/x/text 的 Tag.String() 规范化会吞 region（zh-TW → zh-Hant），前缀判断必须同时覆盖 script 形式。
- 发现待决策缺陷：ResolveTag 对 en_US.UTF-8 系统 locale 落回简体中文（matchTag 只特判中文），记 TODO 待用户拍板。
- vet 陷阱：i18n.T(key, args...) 被 go vet 识别为 printf 包装器，常量键调用会误报「has arguments but no formatting directives」——测试里把键存变量即可绕开（vet 不追非字面量格式串）。

## P1 svc handler 批量补齐结果（2026-11-15 续推）

- svc 包 25.2% → 42.6%（P0 后 25.2%）。已完成：handler_kvstore、handler_fs、handler_online_models、ask_handler、handler_metric_judgment、handler_mcp、handler_team、handler_rule、harvest_intent、turnlog、diff_utils、daemon.go 小函数。
- 集成账（单元层不可达，已记 TODO）：fetchSiliconFlowPricing/fetchRate*（URL 硬编码，仅进程缓存分支可测——用 seedSFPricing/seedFXRate 预置 t.Cleanup 恢复）、wireAskEvents/AskBuilder（需完整 agent 运行时）、handleFSReveal 成功路径（真弹 Finder）。

## P1 新增夹具与语义实证（下次写 svc 测试先看这段）

- 新夹具：`mustParams(t, v)`（handler_fs_test.go，参数 marshal）、`setProviders`（d.app.SetProviderConfigs 注入供应商，BaseURL 指向 httptest）、`withMCPManager`（真实 MCPManager，配置落隔离 HOME，add/remove 只写文件不拉进程）、`withNilKVStore`/`withNilMCPManager`（测未初始化分支后必须恢复原指针，cleanup 会 Close）。
- 供应商 fetch handler 测试套路：httptest 服务器当 BaseURL → 覆盖成功/HTTP 错误/非法 JSON/无 Key/连接拒绝全分支，零真连网；价格/汇率用缓存种子。7 家供应商一个套路打穿。
- 语义实证（与直觉相反处）：rules.Register 是 upsert（重复创建同 ID 不报错）；mcp.server.remove 对不存在条目幂等成功；mcp.server.update 必须带 type 字段（add 可省略按 command 推断）；mcp.server.list 直接返回 []mcp.ServerListEntry；RuleGet 返回 *rules.Rule 指针；goharnesssession.CreateSession 强制要求 project_dir（无目录会话不存在，「无项目目录」分支测不了）；turnlog 同秒二次落盘加 -2 序号（ReadDir 字典序，别按索引断言第二个文件）。
- decodeKVItem 裸 JSON 回退分支只有「合法 JSON 但类型与 kvItem struct 不匹配」（如数组）才触发——未知字段会被静默忽略，`{"raw":1}` 走不到回退。
- metric 返工回溯链路测法：两个会话同 Agent 同 TaskHint 的 round 事件 + rework 上报 → 历史会话事件带 rework_trace 判定；断言必须重读 store（AppendEvent 返回的内存副本不回填判定）。
- go vet 把 %q 用于指针类型（如 *SessionInfo）报格式错——用 %+v。

## 下次我继续推 P1 剩余补测（MCP 子系统 / 基础设施），我将

先读本文全部套路段；internal/mcp 的 manager.go/client.go 涉及真实进程拉起与市场 URL，先扫依赖面找可注入点；pkg/session 的 file_token_usage_store 用 t.TempDir + bbolt 套路照 kvstore。
