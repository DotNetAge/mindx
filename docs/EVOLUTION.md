# Evolution — Agent 自我进化机制

MindX 的进化分两个维度：

```
大进化（纵向）             小进化（横向）
━━━━━━━━━━━━━━            ━━━━━━━━━━━━━━
从历史经验中学习            从能力地图中定位
过去的经验 → 未来的技能     系统的能力 → 自己的装备

会话 → 提取模式 → 生成技能   扫技能目录 → 匹配身份 → 装载技能
```

两者都是 Skill + Script 实现，零 Go 代码入侵。

---

## 小进化：自我审视（Introspect）

### 核心逻辑

Agent 启动或用户触发时：

```
技能仓库（~/.mindx/skills/*/SKILL.md）
        │
        ▼ 扫描所有可用技能
全部技能列表
        │
        ▼ 对照自己的身份
Agent 定义（~/.mindx/agents/<name>.yml）
  ├── role: "软件工程师"
  ├── description: "负责编码、审查、测试"
  └── skills: [目前已装载的技能]
        │
        ▼ 匹配
哪些技能的描述与我的 role/description 匹配？
        │
        ├── 已匹配但尚未装载 → 推荐装载
        └── 已装载但不匹配 → 提醒卸载
```

### 为什么不需要新定义

Agent YAML 已经有 `skills` 字段：

```yaml
name: developer
role: 软件工程师
description: 负责编码、审查、测试
skills:
  - file-organizer      # 手动配的
  - pdf                 # 手动配的
```

小进化只负责**推荐**，不自动修改。推荐的技能由用户（或 LLM 推荐后用户确认）追加到 `skills` 列表：

```yaml
skills:
  - file-organizer
  - pdf
  - verify              # ← 小进化推荐
  - bug-hunter          # ← 小进化推荐
  - code-reviewer       # ← 小进化推荐
```

Agent 下次加载时，这些技能的 Instructions 会自动注入到 System Prompt 中的 SkillsCatalog 区段。

### 去重天然成立

已在 `skills` 列表中的技能不会被重复推荐。Agent 不需要知道"这个技能我装过没有"——YAML 就是真相来源。

---

## 大进化：经验提炼（Evolve）

### 核心逻辑

从会话中提取两种知识：

| 知识类型 | 识别依据 | 产出 | 去向 |
|---------|---------|------|------|
| 工作流（怎么做） | 多步操作连续重复出现 | SKILL.md | `~/.mindx/skills/evolved/<name>/` |
| 偏好（喜欢什么） | 反复出现的倾向或习惯 | 结构化文本 | `~/.mindx/evolved/preferences.md` |

### 工作流 → 技能

重复出现的工作流（如"每次改代码都先 grep → read → edit → test"）被提取为 SKILL.md：

```markdown
---
name: evolved-code-review-flow
description: 提 PR 前的标准审查流程
allowed-tools: bash, grep, read, write
---

# 代码审查流程

## 触发条件
用户要求提 PR 或合并代码时

## 步骤
1. 运行测试套件
2. 检查 lint
3. 更新 CHANGELOG
4. 生成 diff 摘要
```

生成的技能放在 `~/.mindx/skills/evolved/` 下，`FileSystemSkillLoader` 自动发现，Agent 重启即用——与手动编写的技能无差别。

### 偏好 → 长期记忆

用户的编码风格、工具习惯、沟通偏好写入 `~/.mindx/evolved/preferences.md`。

如果该目录已被 Daemon 的 `FileWatchService` 监控，文件会自动索引进 LongTerm Memory。Agent 在需要时通过 `MemorySearch` 工具检索。

### 去重

- **技能去重**：生成前检查 `evolved/<name>/` 目录是否存在
- **偏好去重**：追加写入时附带时间戳，后续可做语义合并

### 触发方式

| 方式 | 命令 / 配置 |
|---|---|
| 手动触发 | Agent 对话中提及"进化"、"反省"等关键词时自动匹配 evolve 技能 |
| 定时触发 | `/job-add @developer new 每周进化反省 expr="0 6 * * 0"` |

---

## 两进化的关系

```
                  ┌──────────────────────┐
                  │   小进化（Introspect） │
                  │   扫技能目录 → 匹配身份│
                  │   → 更新 Agent YAML   │
                  └──────────┬───────────┘
                             │ Agent 知道自己有什么技能
                             ▼
                  ┌──────────────────────┐
                  │   日常对话            │
                  │   使用已装备的技能     │
                  └──────────┬───────────┘
                             │ 积累会话数据
                             ▼
                  ┌──────────────────────┐
                  │   大进化（Evolve）     │
                  │   分析会话 → 提取模式 →│
                  │   生成新 SKILL.md     │
                  └──────────┬───────────┘
                             │ 生成了新技能
                             ▼
                  ┌──────────────────────┐
                  │   小进化（再次）        │
                  │   "新技能适合我吗？"   │
                  │   若有匹配 → 推荐装载  │
                  └──────────┬───────────┘
                             │
                             ▼
                  循环...（能力持续增长）
```

---

## 实现方式

| 技能 | 文件 | 功能 |
|---|---|---|
| `evolve` | `skills/evolve/SKILL.md` + `scripts/evolve` | 大进化：分析会话 → 生成技能/偏好 |
| `introspect` | `skills/introspect/SKILL.md` + `scripts/introspect` | 小进化：扫描技能 → 匹配 Agent 身份 → 推荐装载 |

两个技能都只需要 SKILL.md + helper script，**零 Go 代码改动**。

SkillLoader 自动扫描 `skills/` 子目录，Agent 通过关键字匹配自动发现和触发这些技能。

---

## 为什么这是对的

1. **Agent 自己的 YAML 已经存了 `skills`** — 不需要新数据格式
2. **SkillLoader 已经会扫目录** — 新技能放进去自动被发现
3. **LLM 本身就是最好的模式识别器** — 不需要写规则引擎
4. **Cron 调度器已经就绪** — `/job-add` 即可设置定期进化
5. **脚本处理脏活** — YAML 解析、base64 解码这些 LLM 干不了的事交给 script
6. **技能即应用** — SKILL.md 是 AgentOS 上的交付形态，不需要重新编译
