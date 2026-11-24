---
version: "2.6.2"
pub_date: 2026-11-24
sha: "2d4b5b2597a695ad1ba4ff7506a2fb39af7b1784ae584feeaa777aad67337b2c"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-24 count 归属匹配恒为 0（retrospect）

- 问题：流程漏洞——`scan.py` 的 `count_own()` 用 `p.name.startswith(agent + "-")` 判断归属，而 notes 命名规范是 `<YYYY-MM-DD>-<agent>-<分类>-<标题>.md`。日期在最前面，任何合规 notes 都不可能以 `<agent>-` 开头，于是 `count --agent <本 Agent>` 恒为 0；`status` 的 `curate = own >= max_notes` 永不成立，Full 提纯判据失效。复现现场：Bega 项目 `ls .agents/notes | grep -c go-programmer` 为 4，同一目录 `scan.py count --agent go-programmer` 为 0；`stock` 统计 40 条、分类正常，因为没有按 Agent 归属，掩盖了缺陷。
- 否决：改成 `name.split("-")[3]` 取第三段。否决理由：agent 名本身可含连字符（`go-programmer`），会拆成 `go` 与 `programmer`；无日期旧命名也会越界。另否决把匹配放宽成 `agent + "-" in name`。否决理由：`go` 会吞掉 `go-programmer` 的 notes，前缀型 Agent 名互相污染。
- 修正：新增 `is_own_note(name, agent)`：先用 `DATE_RE` 剥掉日期前缀（兼容 `YYYY-MM-DD` 与 `YYYYMMDD`），再按 `-<agent>-` 整段匹配，并用 `CAT_RE` 校验其后紧跟合法分类或 `缺口-*`；保留无日期旧命名，同样要求分类合法。SKILL 增一行区分本 Agent 20 条提纯阈值与 `stock --notes-threshold` 30 条项目总量提醒，脚本帮助同步标注；`status` 不再透出 stock 的 `notes_threshold` / `notes_over_threshold`，避免把总量提醒读成提纯触发。version 2.6.1 → 2.6.2。
- 验证：Bega 项目 `scan.py count --agent go-programmer` 从 0 变为 4，`scan.py stock` 保持 notes 40 / memory 4、`unclassified` 为空。构造用例覆盖：`2026-11-23-go-经验` 计入 `go`、`2026-11-23-go-programmer-经验` 不计入 `go`、`20261123-go-事实` 计入、旧命名 `go-programmer-经验` 计入、`gopher-经验` 不串入。
