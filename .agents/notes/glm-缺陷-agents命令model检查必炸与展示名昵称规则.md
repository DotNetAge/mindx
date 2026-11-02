# 缺陷：/agents 命令 model 检查必炸 + 智能体展示名军规落地

## 用户纠正（最强信号，2026-11-02 怒斥）

「CLI 绝对不能改！我只叫你改显示给用户看的内容，CLI 大多情况下是显示给 Agent 看的，你缺了 name Agent 怎么能正确加载其它的 Agent?!」

- 我把 cmd/agent.go 的 agent list/info 表格 Name 列换成了昵称、表头中文化——**越界**。CLI 输出是 Agent 的消费数据源（team leader 靠 `agent info` 的 name 列加载成员），昵称替换直接破坏机器可读性。
- **边界判定**：改「显示给用户看的内容」= chatflow 等 GUI 透传的错误文案 + 界面内渲染层；`cmd/` CLI、gateway 命令的机器消费字段（value/rows/JSON）一律不碰。catalog.go 的 label/value 分离是安全形态（label 显示用、value 机器用），但凡是「整列替换」都必须先问消费方是谁。
- 下次我看到「显示规则类需求」，我将：先逐一定位每个改动点的消费方（人 or Agent），只动人那一侧；CLI 文件默认机器消费，改动前列入需要用户确认的范围。

## 用户报告与实锤链

用户在 chatflow 看到 `. agent "xxxxx" : no model configured`（英文 + 内部名），要求：错误提示绝不英文、agent 只能用 NickName 显示。

- 错误文案源：`internal/core/app.go` createRuntime 的 `agent %q: %w` 包住 resolveModelName 的 `no model configured`——两段拼起来正是用户看到的字样，chatflow 错误节点原样透传 daemon 错误。
- **连带真 bug**：`/agents`（internal/commands/catalog.go handleAgents）检查 `item["model"] == ""`，但 wiring.go 的 listAgents 从不产出 model 字段 → 该检查对所有智能体恒真 → `/agents` 必然报错。架构上「Agent 不持有模型属性」（resolveModelName 注释明示：模型是用户级 LastModel/DefaultModel），检查本身已过时，直接删除。
- 修复结果：错误全汉化说人话（含 handler_agent/handler_bundle 同批英文错误），展示名统一昵称（/agents 表格+label、CLI agent list/info 表格、错误消息），`/agents` 表头接 i18n。

## 规则沉淀（下次直接遵守）

1. **展示名规则 = `NickName` 优先，未配置回退内部名**（client.go displayAgent 同源先例）。任何给用户看的智能体名（错误消息、表格、列表 label、日志之外的 UI 文案）都必须走这条；value 等内部标识字段保持内部名不动。
2. **daemon 错误文案 = 界面文案**：chatflow 的 ErrorNodeView 原样透传 daemon `fmt.Errorf` 字符串，daemon 侧写英文错误等于界面上英文。写错误时直接中文说人话（bundle/export.go 先例），不指望壳侧翻译。
3. **跨层数据契约变更要先查 wiring**：catalog.go 检查一个上游根本不提供的字段，恒真检查潜伏两个月没炸是因为没人跑过 `/agents`——新命令上线路径必须实跑一次，禁止只看编译绿。

## 下次看到「界面出现英文错误」识别信号，我将

先 `grep -rn "fmt.Errorf(" internal/` 定位文案源（错误是 daemon 原样透传的），而不是在壳侧找翻译层；修完顺带扫同目录其余英文错误一批清掉。
