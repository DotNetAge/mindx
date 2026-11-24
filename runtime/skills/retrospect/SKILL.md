---
name: retrospect
description: >
  自省与经验治理：清会话债务、治 notes 与 memory 的存量、把稳定经验提纯进记忆并升级为技能或文章。
  当用户要「复盘、自省、整理经验、清理记忆、去存量、提炼技能」，或 notes 与 memory 素材明显冗余、
  会话上下文过重需要压缩时使用；提到知识沉淀、记忆整理时主动使用。
  触发词：复盘、自省、经验整理、记忆清理、去存量、技能提炼、notes 太多、上下文压缩、会话太重。
metadata:
  name_zh: 自省
  version: 2.7.0
  author: Ray
---

# 自省（Retrospect）

治理当前项目的 `.agents/notes/`、`.agents/memory/` 与会话窗口：先还会话债，再把增量聚类成稳定结论，去存量、补参考资料，最后把够格的经验蒸馏为技能、文章或规则提议。

## 何时跑

| 动作 | 触发判据 | 说明 |
| --- | --- | --- |
| 减债（Light） | 当前项目任一会话 window_tokens > 100000 | 有债就还，与时间、缓存无关 |
| 提纯（Full） | 本 Agent 在 notes 下达到 20 条 | 有 notes 产生时判一次（`scan.py count`） |
| 用户显式要求 | 无 | 按用户指定范围跑，不被其他条件否决 |

- **Light 只减债**：压超阈会话，不聚类、不动 notes/memory、不跑质量门。
- **Full 全跑**：减债 + 聚类提纯 + 去存量 + 参考资料 + 质量门；只有减债与提纯同时命中才跑完整 Full。
- 用户只要快速整理或只跑某一主线时，按指定范围跑，不另造模式名；报告写清“窄范围 + 实际做了哪几路”。
- 单轮最多压 3 个会话，按超出量降序，其余下轮。
- 定时任务只做 `scan.py debt`，不设定时提纯；一个工作区一个，不按 Agent 各建。
- 阈值只在本节维护；规则文件只放“达到阈值加载 retrospect”的指针，不重复数值。

## 工作流

进入工作流前按实际范围建任务：Light 建“盘债 / 压缩 / 报告”三项；Full 建步骤 1-7；窄范围只建对应主线。推进中用 `TaskUpdate` 标状态，中断恢复先 `TaskList`/`TaskGet`，不凭回忆对齐。

**清点动作跑脚本，不现场数**：`scan.py` 一个脚本、四个子命令；裸跑只打印 usage。

```bash
python3 {base_dir}/scripts/scan.py debt  --project-dir "$PWD" --current-session-id "$MINDX_SESSION_ID"
python3 {base_dir}/scripts/scan.py stock --agents-dir .agents
python3 {base_dir}/scripts/scan.py count --agent <本 Agent> --agents-dir .agents
python3 {base_dir}/scripts/scan.py status --project-dir "$PWD" --agent <本 Agent>
```

### 步骤 1：选模式

- 只有债 → Light；有提纯判据或用户要求深度复盘 → Full。
- 不确定时用 AskUser 给 Light / Full / 窄范围三个选项，附适用条件与耗时。
- 只治理当前项目：会话按 `project_dir` 过滤，素材只读当前 `.agents`；换项目先切上下文，不跨目录抓。

### 步骤 2：盘债与装载素材

**2.1 盘会话债**：跑 `scan.py debt --project-dir "$PWD"`。脚本只列本项目超阈会话，输出 `action`（`compact`/`defer`）、`deferred_batch` 与 `compact_command`。上下文里能拿到当前 session_id 时必须传 `--current-session-id`；拿不到时按最后活动时间判断，并在报告注明“当前会话未显式识别”。

**2.2 压超阈会话**：

- 压缩是游标前移 + 摘要，旧消息保留，摘要落 `.agents/memory/`；但活跃窗口被清空且摘要不回注当前上下文，按不可逆操作处理。
- 把 `compact_batch`、每个会话的 tokens 与命令一次性交给用户确认；确认后本轮批量执行，不逐条重复询问。
- 不压当前会话、不压 30 分钟内有活动的会话、不压最后活动未知的会话；`scan.py` 已把这三类标成 `defer`。
- 当前会话超阈时，报告必须写“待压：session_id + tokens + 空闲后执行的 compact_command”，不得写“无债可减”。
- 预检必跑：`mindx session compact --session-id <id> --dry-run`；空窗口或未达阈值属正常拒绝，跳过即可。
- 摘要只算线索，不算结论；聚类时必须回溯原文或另有独立记录交叉印证。

**2.3 装载素材**：

1. `scan.py stock` 看两处条数、分类分布、命名不合规项；memory 的压缩摘要与提炼条目分开看。
2. 先读归属自己的 notes 与 memory（主素材）；他人条目只作旁证并标注来源。
3. 扫技能进化履历：项目 `.agents/skills/*/notes/` **和** 全局 `~/.mindx/skills/*/notes/`；履历只读，不修改不搬移。
4. 跳过 `.archive/` 与已标记 deprecated 的内容。
5. Full 模式跑 `metric_scan.py failures`，从 `fail`/`rework` 判定取四段式 note；四段不齐的补齐前不升技能。
6. 以上来源全空时如实告知并终止，不硬凑结论。

### 步骤 3：经验增量

1. 按 tags 与主题聚类；同一结论在多条独立记录中重复 → 稳定路径。
2. 先落记忆：`<YYYY-MM-DD>-<agent>-<分类>-<标题>.md`，分类取标准词表或六类 `缺口-*`；正文写“结论 + 识别信号 + 触发动作”。
3. 只落已交叉印证的结论：跨会话重复 ≥2 次或有多条独立记录支撑；单次偶发留在 notes。
4. 候选知识五分流：技能候选（可写 if/else 决策路由）/ 文章候选（原理与取舍）/ 插件立项（呈现位反复缺失）/ 规则修订提议（行为约束）/ 角色招募提议（职责边界缺口）。
5. 技能候选必须自包含；发现存量技能缺陷只写修订提议到 `.agents/notes/`，不直接改技能文件。
6. 文章候选只判定不发布；过三条质量门后路由 `article-publish`。

### 步骤 4：去存量

| 对象 | 处置 |
| --- | --- |
| notes 被技能/记忆吸收 | 列清单，用户确认后删除 |
| memory 被技能/文章吸收 | `mv` 到 `.agents/memory/.archive/` |
| 冗余重复条目 | 合并为一条主记录，写明来源 |
| 过时失效条目 | 归档并注明失效原因 |

条目难取舍时跑 `fapprox.py .agents --skip-newer-than 24`：P0 只作提示，先查年龄与来源，本轮新建或 24 小时内不处置；P1 可精简，提取引用段后归档；P2 暂不处理；P3 保留。任何归档/删除仍需用户确认。

### 步骤 5：参考资料

1. 外搜前先查本地 `references/` 与社区文章库；未命中再搜官方文档、论文、规范。
2. 新增 `references/ref_XXX/`（`meta.json` + 原文），更新 `references/index.csv`。
3. 跑 `refcheck.py .agents`；`broken` 数组逐条评估引用该公理的结论是否失效。

### 步骤 6：质量门（候选技能必过）

评分方法与棘轮详见 `references/quality-gate.md`。要点：

1. 跑 `mindx skill validate --path <skill_dir>`，以 `ok`、`stats.soft_rules`、`issues` 为准；不另写脚本复刻。
2. 派 SubAgent 以纯读者视角复评具体性与失败模式；作者自评只作预筛。
3. 棘轮：新稿分数 > 旧版才落位；否则保留旧版，把被拒理由写入技能 `notes/`。
4. 同一轮只改一个维度；连续 2 轮无提升即停。

### 步骤 7：落位与报告

1. 过门候选写入 `<ProjectDir>/.agents/skills/<name>/`；多 Agent 场景标记“待交叉验证”。
2. 跑 `mindx metric baseline save round-<日期>` 存快照。
3. 报告至少写：压缩了哪些会话、各释放多少；**待压清单**（session_id / tokens / compact_command）；新记忆标题；聚类组数；归档/删除条数与理由；技能/规则产出；拒绝项与原因；量表对比（可用时）。

## 注意事项

- 归档用移动，notes 才允许删除；删除前必须过用户确认。
- 他人记忆写进候选技能前，必须在本上下文验证一次。
- 处置理由必须落盘，禁止只做不记。
- references 连锁校验不能只查一条；引文断裂比记忆过期危害更大。
- 一轮不试图清完历史存量，按本次新增素材为界。

## 失败模式（现象 → 原因 → 动作）

- 现象：裸跑 `scan.py` 只出 usage。原因：把“一个脚本”当成“一条命令”。
  动作：按 debt/stock/count/status 子命令调用。
- 现象：压到别的项目会话。原因：没传 `--project-dir` 或不在项目根执行。
  动作：从项目根跑并显式传 `--project-dir "$PWD"`。
- 现象：报告写“无债可减”，但当前会话仍超阈。原因：把 defer 当成清零。
  动作：把待压 session_id、tokens 与 compact_command 写进报告。
- 现象：轮内压掉自己的上下文。原因：对当前会话执行 compact。
  动作：当前会话永远 defer，交给空闲会话或定时任务。
- 现象：刚提炼的记忆被 fapprox 判 P0 后删除。原因：新条目尚无引用。
  动作：跑 `--skip-newer-than 24`，并对 P0 先查年龄与来源。
- 现象：质量门另写脚本复刻 `mindx skill validate`，结果与 CLI 漂移。原因：想绕过 CLI 自检。
  动作：只认 `mindx skill validate`，读 `soft_rules` 与 `issues`。
- 现象：自省看不到自己的进化履历。原因：只扫项目级技能目录。
  动作：同时扫全局 `~/.mindx/skills/*/notes/`。
- 现象：候选技能自评很好看，落位后没人用。原因：自己写自己评。
  动作：派 SubAgent 独立复评，棘轮不过不落位。
- 现象：把摘要直接当知识入库。原因：把压缩当成整理。
  动作：摘要只作线索，回流原文或独立记录交叉印证。
