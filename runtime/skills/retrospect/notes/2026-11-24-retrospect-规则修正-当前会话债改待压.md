---
version: "2.6.3"
pub_date: 2026-11-24
sha: "0c8ffdc06064b72dbe2c65d9639dd2a3ea191d84e2819c6412f44a97e3201b9d"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-24 当前会话的债被跳过且无人接手（retrospect）

- 问题：流程漏洞——`scan.py debt` 把最后活动 30 分钟内的会话标成 `in_use`，`cmd_status` 仍把它放进 `compact_batch` 并给出 `Light（只减债）`，但技能 2.2 要求跳过「正在用」的会话。自省通常就在这个重会话内部触发，当前会话必然 `in_use`，于是本轮一个会话也不压，报告只写「跳过，不压正在用的会话」，债务没有登记也没有接手人；同一个判据既是触发源又是排除项。现场：go-programmer 会话 `01M81Q25VX2DG7YR88VAQM3EKE` 566,173 tokens、cursor 0、idle 0 小时，自省报告写「减债：无动作」，本轮结束后债仍在。
- 否决：删掉 30 分钟保护，直接 `ForceCompact` 当前会话。否决理由：`executeFullCompaction` 只把 cursor 移到消息末尾，摘要经 `SummarySink` 落 `.agents/memory/`，不会回注当前上下文；轮内压会把本轮后续工具与推理上下文清空，当前任务直接断线。保护不能删，缺的是压完之后的交接。
- 修正：`scan.py debt` 对 `in_use` 会话标 `action=defer`，输出 `deferred_count` 与 `compact_command`；`status` 把可压会话与待压会话拆成 `compact_batch` / `deferred_batch`，全部待压时 mode 改为 `Light（无空闲会话，债务待压）`，提纯同时命中时为 `Full（只提纯，债务待压）`。SKILL 2.2 改为「当前会话的债不算清零」：报告必须写待压 session_id、tokens 与空闲后执行的命令，不得写「无债可减」；并明确禁止轮内压自己。version 2.6.2 → 2.6.3。
- 验证：构造会话数据验证——活跃会话 600k 且 idle 0 → `deferred_count=1`、`compact_batch=[]`；空闲会话 120k → `compact_batch` 含该会话；全部待压且本 Agent notes 20 条 → `Full（只提纯，债务待压）`。真实 Bega 项目在用户手动压缩该会话之后，`scan.py debt` 返回 `checked 2`、无超阈，`status` 的 `deferred_batch` 为空；这份空债是手动压缩的结果，不能当作技能自动减债的证据。
