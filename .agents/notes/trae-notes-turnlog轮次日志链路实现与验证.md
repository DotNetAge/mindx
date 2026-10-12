# 轮次日志（logs）日报链路实现与验证

- 发生时间：2026-10-12
- 分类：实现记录 + 验证经验

## 定稿链路（零算力日报事实流）

daemon 每轮 Ask 结束 → 原文落盘 `<ProjectDir>/.agents/logs/<YYYY-MM-DD-HHMMSS>-<agent_name>.md` → `mindx report collect --date <日期>` 两阶段收割（先聚合落盘校验成功、后删源）→ daily-report 技能消费生成日报。

- 落点在 mindx 应用层（internal/svc/turnlog.go + daemon.go Run() 前后各 4 行），**不改 goharness**（用户明确纠正：那是机制层）。
- 最终回复提取：Run() 前快照 `len(s.All())`，Run() 后只在新增区间找最后一条 `FinishReason=="stop"` 的 assistant 消息。快照的必要性：(a) AskUser 追答会插入多条 user/assistant 消息，"最后一条 user 消息"不是可靠边界；(b) 本轮失败无新消息时防止误取上一轮的答案。
- 文件冲突：O_EXCL 独占创建 + 同秒同名追加 -N 序号（并发安全）。
- collect 的 --agent 过滤靠文件名解析：`<date>-<HHMMSS>-<agent>[-N].md`，agent 名可含连字符，尾部纯数字段剥离为序号。
- collected-*.md 命名不以日期开头，天然不会被下次 collect 二次收割。

## 下次我看到"用 stash 验证测试失败是否 pre-existing"，我将：

改用「看失败断言 + git diff 改动范围」判断，不再 git stash。原因：stash 不收 untracked 文件，stash 后变成 HEAD + untracked 混合状态（如 handler_device.go 引用 untracked 的 internal/devgate/），直接编译失败，验证无效；且 stash/pop 本身有风险。

## 遗留事实（非本次引入）

`go test ./internal/svc/ -run TestHandleMarketAccountGet_Anonymous` 失败：`handler_market_account_test.go:58 has_token = true, want false`。工作区存在大量未提交的市场账号体系重构改动（handler_market_account.go 等均为 modified），失败归属那条工作线，与 turnlog 改动无关（turnlog 只动 daemon.go Run() 前后 4 行 + 新增独立文件）。
