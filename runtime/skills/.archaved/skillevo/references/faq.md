# 反模式与FAQ

> 反例黑名单、架构红线、异常场景、指令映射集中查阅。

---

### FAQ：常见问题速查

**Q: 只想打分不修改？**
A: 说"评估 {skill_name} 质量"，走 Phase 0.5-1 仅评估不改。

**Q: 优化卡住？**
A: 查[异常与边界条件](#异常场景速查)表，按"处理动作"列操作。常见：不在 git 仓库、results.tsv 损坏、子 agent 不可用。

**Q: Quick 和 Full 区别？**
A: Quick 轻量（3 轮、dry_run 推演、.bak 回退），Full 深度（5 轮、多评委、git 分支、安全门控）。默认 Quick，分数低或有 revert 时自动升级 Full。

**Q: 优化后分数下降？**
A: 棘轮机制自动回滚。新分不如旧分时自动 `git checkout` 恢复。

**Q: 指令触发哪个模块？**
A: 见下表：

| 指令 | 触发模块 |
|------|----------|
| "优化所有 skills" | 鲁班核心引擎 Phase 0-3（全量） |
| "优化 {skill_name}" | 鲁班核心引擎 Phase 0-3（单个） |
| "评估所有 skills 质量" | Phase 0.5-1（仅评估不改） |
| "检查技能健康" | SkillOps 巡检 |
| "这个技能有问题 / 不对" | EvoSkill 失败修补 |
| "更新技能知识" | CASCADE 知识更新 |
| "精简技能 / 瘦身" | Skill Distill |
| "规则硬化 / 硬一点" | HASP 规则硬化 |
| "安全审计 / 安全检查" | Sentinel 安全扫描 |
| "看看优化历史" | 读取 optimization-registry.tsv |

**Q: dim8 得分很低？**
A: dim8 占 20 分权重最高。低分原因：① dry_run 模式降权；② 编辑同源检测（×0.5）；③ 测试 prompt 不合理。

**Q: 什么是"触顶信号"？**
A: 连续 2 轮 Δ < 2 分 → 自动停止。

**Q: 怎么恢复优化前的 skill？**
A: Full 模式 `git checkout` 回退 commit；Quick 模式 `.bak` 文件恢复。

---

### 反模式黑名单（操作层面）

来自早期 40 次 0 revert 的教训。每轮 Phase 2 Step 2 改动前对照。

| # | 反模式 | 正确做法 |
|---|--------|----------|
| 1 | 同 context 自评自改 | spawn 独立子 agent 评分 |
| 2 | `git reset --hard` 当回滚 | 用 `git checkout` 保留追溯链 |
| 3 | 为凑分增冗余 | 触顶信号（Δ<2 连续 2 轮）→ break |
| 4 | 跳过 test-prompts | Phase 0.5 强制设计 2-3 prompts |
| 5 | 轮内改多个维度 | 每轮只改 1 个维度 |
| 6 | dry_run 比例 > 30% | 强制至少 1 个 full_test |
| 7 | 静默跳过异常 | 异常表 fallback 先告知用户 |
| 8 | 忽视维度相关性 | 看相关簇短板再决定先修哪个 |

---

### 架构红线（运行时检测）

违反任一条触发阻断或降权：

| # | 红线 | 后果 |
|---|------|------|
| 1 | 禁止 self-edit-self-evaluate | dim8 降权 ×0.5 |
| 2 | 禁止跨维度打包修改 | 整轮回滚 |
| 3 | 禁止 dry_run 冒充 full_test | results.tsv 标记 invalid |
| 4 | 禁止 bypass gate | 中断流程 |

---

### 异常场景速查

| 场景 | 触发条件 | 处理动作 |
|:---|:---|:---|
| 不在 git 仓库 | `git rev-parse` 失败 | 询问用户：`git init` 或文件备份 |
| results.tsv 缺失 | 文件不存在 | 新建并写表头 |
| results.tsv 损坏 | 列数不匹配 | 备份后重建 |
| 分支已存在 | `git checkout -b` 失败 | 分支名加 `-2`/`-3` |
| git revert 失败 | 冲突/工作树脏 | `git stash` 重试；仍失败手动恢复 |
| MAX_ROUNDS 触顶 | 已达上限 | 询问用户继续/重写/收工 |
| 优化后超 150% 体积 | 新文件 > 原 ×1.5 | 拒绝提交，先精简 |
| test-prompts.json 已存在 | 文件已存在 | 复用/重写/追加三选一 |
| SKILL.md 找不到 | 目录无主文件 | 终止此 skill |
| 消费者基线失败 | 模型能力不足 | 阻断优化 |
| 子 Agent 不可用 | dispatch_task 报错 | 降级模式 |
| 分数精度漂移 | 总分差 < 0.05 | 不靠四舍五入 |
