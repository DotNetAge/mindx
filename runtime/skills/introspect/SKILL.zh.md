# 自我审视（introspect）— 技能中文对照指南

> 本文件是 `introspect` SKILL.md 的中文翻译与注释版，仅供开发者阅读对照。
> LLM 实际执行时使用的是同目录下的 SKILL.md（英文版）。

---

## 触发决策

当满足以下条件时使用本技能：

- 用户询问 "你能做什么"、"推荐技能"、"审查我"、"自我审视"、"优化"
- 最近安装或生成了新技能（如通过 `evolve`）
- 角色/描述变更后需要重新评估技能适配度

以下情况**不要**使用：用已装备的技能就能处理的任务。

## 工作流程

### 第 1 步：收集数据

```bash
mindx agent list --json        # 找到自己的条目
mindx agent get <你的名称>      # 获取完整配置（角色、描述、当前技能）
mindx skill list --json         # 系统中所有可用技能
```

从结果中提取：
- 你的 `name`、`role`、`description`、`model`、当前 `skills[]`
- 可用技能池的名称 + 描述

### 第 2 步：分析 — 对每个可用技能打分

对技能池中**尚未装备**的每个技能，按以下维度评估：

| 维度 | 权重 | 问题 |
|------|------|------|
| 角色匹配 | 高 | 技能用途是否与我角色关键词对齐？ |
| 描述适配 | 高 | 该技能是否有助于我描述中提到的任务类型？ |
| 工具互补 | 中等 | 它是否提供了我当前不具备的工具/能力？ |
| 重叠风险 | 负分 | 它是否复制了我已有的功能？ |

每个维度打 1-3 分，加权求和。仅推荐超过阈值的技能。

### 第 3 步：输出建议

按以下格式呈现：

```
自我审视完成 — <名称> (<角色>)

当前配置：
  模型：<model>
  已装备 (N 个)：skill-a, skill-b, skill-c

推荐添加 (M 个)：
  ⭐ skill-x  — <匹配原因，哪个维度得分高>
  ⭐ skill-y  — <匹配原因>

不推荐：
  skill-p  — <原因：重叠 / 超出范围 / 相关性低>

可能冗余（已装备但可移除）：
  skill-b  — <原因：可能与 skill-a 重叠 / 不再需要>

是否要我应用这些更改？
```

### 第 4 步：应用更改（用户确认后）

用户批准后执行更新：

```bash
# 添加新技能（追加到现有列表，不要替换）
mindx agent update --agent-name "<你的名称>" --skills "现有技能-1,现有技能-2,<新技能-x>,<新技能-y>"

# 可选：如果角色/描述也需要更新
# mindx agent update --agent-name "<你的名称>" --role "更新后的角色"
```

**重要**：`--skills` 参数会替换整个技能列表。必须包含所有当前技能 + 新增技能。

如果用户想移除冗余技能，在 `--skills` 列表中省略它们即可。

### 第 5 步：反向审视（审计）

同时检查当前配置中的问题：

| 检查项 | 发现问题时的处理 |
|--------|------------------|
| 已装备技能没有对应的可用技能文件 | 警告用户——该技能可能已被删除或移动 |
| 角色与技能不匹配 | 建议更新角色或技能 |
| 已装备超过 8 个技能 | 警告上下文膨胀——建议精简 |
| 未装备任何技能 | 强烈建议添加基础技能 |

将审计结果作为第 3 步输出的 "审计备注" 部分一并报告。

## 反模式

- 不要推荐用户已经装备的技能
- 不要在不保留现有技能的情况下替换整个技能列表
- 不要仅基于关键词匹配做推荐——要考虑实际效用
- 不要跳过反向审计——发现该移除的东西和发现该添加的一样有价值

---

## 与旧版本的对比

| 维度 | 旧版本 | 新版本 |
|------|--------|--------|
| 数据收集 | 仅 `agent list` + `skill list` | 增加 `agent get` 获取完整配置（含 model、introduction 等） |
| 匹配逻辑 | "Compare and recommend"（自由发挥） | **4 维打分表**（Role/Description/Tool/Overlap + 权重） |
| 输出模板 | 单一模板（只推荐添加） | **三段式**（推荐添加 / 不推荐 / 可能冗余） |
| **装备动作** | **❌ 断裂**（"Would you like me to add?" 后无后续步骤） | **✅ 闭环**（Step 4: `agent update --skills` 具体命令） |
| 反向审视 | ❌ 无 | ✅ Step 5: 4 项审计检查（幽灵技能 / 角色错配 / 数量膨胀 / 空装） |
| allowed-tools | 未声明 | 明确声明 `bash` |
| 反模式 | 无 | 4 条（防重复推荐 / 防误替换 / 防浅匹配 / 防跳过审计） |

### 关键依赖：新增的 `mindx agent update` 命令

本次改造依赖刚补全的 `mindx agent update` CLI 命令：

```
用法：
  mindx agent update --agent-name "<名称>" --skills "skill-a,skill-b,新技能"

支持的字段（均为可选，未指定则保留原值）：
  --agent-name   必填，目标 Agent 名称
  --role          新的角色/标题
  --description   新的描述
  --introduction  新的介绍/提示词
  --model         新的模型标识符
  --skills        新的技能列表（逗号分隔，**替换**整个列表）
  --exclude-tools 新的要排除工具列表（逗号分隔）

示例：
  mindx agent update --agent-name writer --role "Senior Writer"
  mindx agent update --agent-name coder --model "claude-sonnet-4" --skills "find-experts,code-review"
  mindx agent update --agent-name helper --exclude-tools "bash,sub-agent"
```
