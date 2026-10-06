# Team 实体 — 一等团队、编制设计与招募

> 团队的结构事实（实体与命令）与编制设计规范（怎么切分成员领域才可路由）。
> 成员 Agent 的定义规范见 [agent-schema.md](agent-schema.md) 与 [agent-writing.md](agent-writing.md)。

## Team 实体

固定团队不是 Agent 的属性，而是独立的一等实体（`~/.mindx/teams/<team>/TEAM.md`）：

- frontmatter：`name`（内部标识）/ `nick_name`（展示主名）/ `description` / `leader`（负责人 Agent 名）/ `members`（成员 Agent 名列表，纯引用）/ `icon` / `category` / `hired`
- 正文 Profile：团队人设定义（负责人怎么协调成员、跟踪结果、汇报成果），系统在团队会话中原样注入（缺失时用兜底文案）
- **引用单向（Team→Agent）**：团队持有成员名单，Agent 定义里不写任何团队信息；同一 Agent 可被多个团队引用
- **团队会话以负责人身份发起**（团队会话 resolve 到 leader，与团队对话 = 与负责人对话）；雇佣团队（`mindx team hire`）后团队才出现在会话选择器
- 管理命令：`mindx team add/update/hire/fire/rm/export`；团队可整包进市场（`mindx team export` 产出 kind=team 包，消费者 `mindx market install team` 整队招聘）

## 编制设计规范（建队前必须完成）

团队是"用户需求 → 团队 → 成员"选择链的中层。编制设计决定这层选择是否有效：

1. **领域切分互斥且覆盖**：成员职责两两互斥（任一具体任务有且只有一个明确归属成员）且并集覆盖团队使命。检验法：把一个典型任务分别对照两个相邻成员的概览，若两者都像"归我"，切分无效，重新划界。
2. **概览保持干净**：成员 description 只写自己的概览（负责什么/能干什么/什么时候用），禁止为划界在概览里指派或指代其他成员——切分问题在编制层解决，不进 description。
3. **独特可辨自检**：某成员的概览换成同团队其他成员仍然成立 → 该成员没有独立领域，合并或重切。
4. **团队 description 也是概览**：写清这个团队承接什么需求、能产出什么、什么时候找它——这正是概览三问，同样禁止"全方位 X 服务"式空话与指代成员的话术。
5. **leader SOUL 必须写协调职责**：团队会话就是与 leader 对话——leader 的 SOUL 除个人专业职责外，必须写明任务拆解、派发成员、跟踪结果、汇报成果的方式；缺协调职责的 leader 会在团队会话里自己埋头干活。
6. **团队职责归 Team Profile**：成员间协作规则、验收口径、汇报链路写进 TEAM.md 正文，不散落进成员 SOUL。

## 招募与雇佣要点

- 成员 Agent 一律按 SKILL.md 工作流逐个招募（先查本地与市场，能复用绝不新建）。
- 建队任命负责人（必须是已招募的 Agent）：`mindx team add <team> --leader <agent> --member ... --description "团队一句话定位" --nick_name "两三字外号"`。
- 团队职责（负责人怎么协调、跟踪、汇报）落 TEAM.md 正文 Profile（`mindx team update <team> --profile "..."` 或直接编辑）。
- `mindx team hire` 后团队才出现在会话选择器；调度链保持"发起方 → 负责人 → 成员"，发起方不越过负责人直管成员。
- 整队进市场：`mindx team export` 产出 `<team-name>.mindpkg`，消费者 `mindx market install team` 一次装齐团队与成员。
