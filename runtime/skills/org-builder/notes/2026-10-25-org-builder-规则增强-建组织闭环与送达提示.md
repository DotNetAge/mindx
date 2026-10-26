---
version: "1.1.0"
pub_date: 2026-10-25
sha: "1ff806f8f259fdffbcb5b4bbe4a7a990f53554357bf260b56505f35af54cd333"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 老板侧闭环断点：尚无组织时技能止步，邀请送达无提示（glm）

- 问题：流程漏洞——入口判断在「没有加入任何组织」时直接停（让老板去 MindX 社区客户端手动建组织），老板侧 建组织→画表→发邀请 链条在第一环断掉；同时邀请只落服务端 org_invites 表无推送，技能未提示老板线下告知，受邀者无从得知邀请存在。根因：建组织此前无 CLI 落点（daemon 侧 market.org.create RPC 已注册，但缺客户端方法与 CLI 命令），邀请送达通道未定案。
- 否决：受邀侧自助查询（服务端新端点 + 客户端新命令——入职流程本就在无邀请时给人话报错，增益有限，改动面大）；邮件推送（需邮件服务与受邀者邮箱，现在邀请只按账号，未必有邮箱，现阶段不做）。用户定案：送达短期 = 线下告知；org-builder 暂不上架市场 contents。
- 修正：补 `mindx org create`（pkg/rpc/bundle.go 客户端方法 + cmd/org.go create 命令）；入口判断「没有加入任何组织」分支改走新增「建组织」小节（问名称 + 别名 → create → 回新建流程）；邀请入职第 3 步补「邀请不会自动通知对方，请线下告知」；命令速查加 create、纪律边界五条命令改六条、失败模式补 create 被拒。version 1.0.0 → 1.1.0。送达定案标注于 mindx-market/docs/组织化运作实现.md §6。
- 验证：`mindx org create --help` 正常渲染；mindx 仓 `go build ./... && go vet ./cmd/ ./pkg/rpc/` 通过、`go test ./pkg/rpc/` ok；SKILL.md 洁净自查（定案|定稿|用户提出|今天|实测）0 命中。
