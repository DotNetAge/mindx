# SKILL.md 迁移分析报告

> 创建日期：2026-03-05
>
> 分析范围：35 个现有 SKILL.md 文件
>
> 目标：识别不符合 agentskills.io 规范的部分，制定迁移策略

---

## 📊 分析概览

### 统计数据

- **总计**：35 个 SKILL.md 文件
- **100% 不符合规范**：所有文件都需要迁移
- **平均不符合字段数**：8-10 个/文件

### 字段使用统计

| 字段 | 使用数量 | 符合规范 | 说明 |
|------|---------|---------|------|
| name | 35 | ✅ | 保持 |
| description | 35 | ✅ | 保持（需改为 SOP 描述） |
| version | 35 | ✅ | 保持 |
| tags | 35 | ✅ | 保持 |
| **category** | **35** | ❌ | 删除（用 tags 替代） |
| **os** | **35** | ❌ | 删除（移到 Tool 定义） |
| **enabled** | **35** | ❌ | 删除（运行时决定） |
| **timeout** | **35** | ❌ | 删除（移到 Tool 定义） |
| **command** | **30** | ❌ | 删除（移到 Tool 定义） |
| **parameters** | **26** | ❌ | 删除（移到 Tool 定义） |
| **requires** | **9** | ⚠️ | 改为 required_tools |
| **homepage** | **9** | ❌ | 删除（不在规范中） |
| **is_internal** | **5** | ❌ | 删除（不需要） |
| **guidance** | **3** | ⚠️ | 合并到 Triggers/SOP |

---

## 🚨 核心问题

### 问题 1: Skill 和 Tool 概念混淆

**当前错误**：SKILL.md 包含 Tool 的定义（command, parameters, timeout, os）

```yaml
---
name: calculator
description: 计算器技能，执行数学计算
command: ./calculator_cli.py        # ❌ 这是 Tool 的属性
timeout: 30                         # ❌ 这是 Tool 的属性
os: [darwin, linux]                 # ❌ 这是 Tool 的属性
parameters:                         # ❌ 这是 Tool 的定义
  expression:
    type: string
    required: true
---
```

**正确做法**：Skill 是 SOP 文档，只声明需要哪些工具

```yaml
---
name: math_calculation
description: 数学计算的标准操作程序
required_tools: [calculator]        # ✅ 只声明需要的工具
---

# Goal
执行数学计算和运算表达式

# Triggers
- 用户要求计算
- 用户提到"算一下"、"计算"
- 用户输入数学表达式

# SOP
1. 解析用户输入的数学表达式
2. 使用 calculator 工具执行计算
3. 返回计算结果
```

---

### 问题 2: 缺少 SOP 内容

**当前错误**：正文只是工具使用说明，不是操作程序

```markdown
# 计算器技能

## 示例
\`\`\`json
{
  "name": "calculator",
  "parameters": {
    "expression": "2+3*4"
  }
}
\`\`\`
```

**正确做法**：提供完整的 SOP（Goal + Triggers + SOP + Examples）

```markdown
# Goal
执行数学计算和运算表达式，支持基础运算和科学计算

# Triggers
- 用户要求计算数学表达式
- 用户提到"算一下"、"计算"、"等于多少"
- 用户直接输入数学表达式（如"2+3*4"）

# SOP
1. 识别用户输入中的数学表达式
   - 提取表达式字符串
   - 验证表达式合法性

2. 调用 calculator 工具
   - 传入表达式参数
   - 等待计算结果

3. 生成响应
   - 如果成功："计算结果是 {result}"
   - 如果失败："表达式格式错误，请检查"

# Examples
**用户**: 2+3*4 等于多少？
**助手**: 计算结果是 14

**用户**: 帮我算一下 sin(0.5)
**助手**: 计算结果是 0.479
```

---

### 问题 3: category 字段不符合规范

**当前分类**：
- system: 12 个
- general: 17 个
- productivity: 4 个
- communication: 2 个

**正确做法**：使用 tags 替代 category

```yaml
# ❌ 错误
category: productivity
tags: [notes, 笔记]

# ✅ 正确
tags: [notes, productivity, 笔记, 备忘录]
```

---

## 📋 迁移映射规则

### 规则 1: Meta 字段映射

| 当前字段 | 新字段 | 处理方式 |
|---------|--------|---------|
| name | name | 保持不变 |
| description | description | 改为 SOP 描述（"XXX的标准操作程序"） |
| version | version | 保持不变 |
| tags | tags | 合并 category 到 tags |
| category | - | 删除，值添加到 tags |
| os | - | 删除，移到 Tool 定义 |
| enabled | - | 删除 |
| timeout | - | 删除，移到 Tool 定义 |
| command | - | 删除，移到 Tool 定义 |
| parameters | - | 删除，移到 Tool 定义 |
| requires.bins | required_tools | 转换为工具名称列表 |
| homepage | - | 删除 |
| is_internal | - | 删除 |
| guidance | Triggers/SOP | 合并到 Triggers 或 SOP |

### 规则 2: 内容结构映射

| 当前内容 | 新结构 | 处理方式 |
|---------|--------|---------|
| 正文标题 | Goal | 提取目标描述 |
| guidance | Triggers | 转换为触发条件列表 |
| 使用说明 | SOP | 重写为操作步骤 |
| 示例 | Examples | 转换为对话示例 |

---

## 🔄 迁移策略

### 策略 A: 自动迁移（70%）

**适用场景**：
- 简单的工具调用型 Skill
- 单一功能的 Skill
- 没有复杂 guidance 的 Skill

**自动处理**：
- Meta 字段映射
- 基础 SOP 生成
- Triggers 提取

**示例**：calculator, weather, terminal

---

### 策略 B: 半自动迁移（20%）

**适用场景**：
- 有 guidance 字段的 Skill
- 多步骤操作的 Skill
- 需要条件判断的 Skill

**需要人工**：
- 审核 SOP 步骤
- 调整 Triggers
- 补充 Examples

**示例**：deep_search, contacts, portcheck

---

### 策略 C: 手动迁移（10%）

**适用场景**：
- 复杂的多工具协作 Skill
- 需要重新设计 SOP 的 Skill
- 特殊逻辑的 Skill

**需要人工**：
- 完全重写 SOP
- 设计工具组合
- 编写详细示例

**示例**：github（复杂的 API 操作）

---

## 📝 迁移模板

### 模板 1: 简单工具调用型

```yaml
---
name: {skill_name}
description: {功能描述}的标准操作程序
version: 1.0.0
author: mindx
tags: [{从 category 和 tags 合并}]
required_tools: [{从 command/requires 提取}]
---

# Goal

{从 description 扩展，描述技能目标}

# Triggers

- 用户{触发条件1}
- 用户提到"{关键词1}"、"{关键词2}"
- {从 guidance 提取}

# SOP

1. {步骤1描述}
   - {子步骤1}
   - {子步骤2}

2. 调用 {tool_name} 工具
   - 传入参数：{参数说明}

3. 生成响应
   - 成功："{响应模板}"
   - 失败："{错误处理}"

# Examples

**用户**: {示例输入1}
**助手**: {示例输出1}

**用户**: {示例输入2}
**助手**: {示例输出2}
```

---

### 模板 2: 多步骤操作型

```yaml
---
name: {skill_name}
description: {功能描述}的标准操作程序
version: 1.0.0
author: mindx
tags: [{标签列表}]
required_tools: [{工具列表}]
optional_tools: [{可选工具}]
---

# Goal

{详细的目标描述，包括主要功能和预期结果}

# Triggers

- {触发条件1}
- {触发条件2}
- {触发条件3}

# SOP

1. {阶段1：信息收集}
   - {步骤1.1}
   - {步骤1.2}
   - 如果{条件}，则{操作}

2. {阶段2：执行操作}
   - 使用 {tool1} 工具{操作1}
   - 使用 {tool2} 工具{操作2}

3. {阶段3：结果处理}
   - 解析结果
   - 格式化输出

4. {阶段4：生成响应}
   - 综合信息
   - 返回友好的回复

# Examples

**场景1：正常流程**
**用户**: {输入}
**助手**: {输出}

**场景2：需要澄清**
**用户**: {模糊输入}
**助手**: {澄清问题}
**用户**: {补充信息}
**助手**: {最终输出}
```

---

## 🎯 具体迁移示例

### 示例 1: calculator（简单型）

**当前格式**：
```yaml
---
name: calculator
description: 计算器技能，执行数学计算和运算表达式
version: 1.0.0
category: general
tags: [calculator, math, 计算器, 计算, 数学, 运算]
os: [darwin, linux]
enabled: true
timeout: 30
command: ./calculator_cli.py
parameters:
  expression:
    type: string
    description: 数学表达式
    required: true
---

# 计算器技能

## 示例
\`\`\`json
{
  "name": "calculator",
  "parameters": {"expression": "2+3*4"}
}
\`\`\`
```

**新格式**：
```yaml
---
name: math_calculation
description: 数学计算的标准操作程序
version: 1.0.0
author: mindx
tags: [calculator, math, general, 计算器, 计算, 数学, 运算]
required_tools: [calculator]
---

# Goal

执行数学计算和运算表达式，支持基础运算（加减乘除）和科学计算（三角函数、对数等）

# Triggers

- 用户要求计算数学表达式
- 用户提到"算一下"、"计算"、"等于多少"
- 用户直接输入数学表达式（如"2+3*4"、"sin(0.5)"）

# SOP

1. 识别用户输入中的数学表达式
   - 提取表达式字符串
   - 验证表达式合法性（包含数字和运算符）

2. 调用 calculator 工具
   - 传入 expression 参数
   - 等待计算结果

3. 生成响应
   - 如果成功："计算结果是 {result}"
   - 如果失败："表达式格式错误，请检查语法"

# Examples

**用户**: 2+3*4 等于多少？
**助手**: 计算结果是 14

**用户**: 帮我算一下 sin(0.5)
**助手**: 计算结果是 0.479

**用户**: 100除以3
**助手**: 计算结果是 33.333
```

---

### 示例 2: weather（简单型）

**当前格式**：
```yaml
---
name: weather
description: 天气查询技能，查询全球城市天气信息
version: 1.0.0
category: general
tags: [weather, forecast, 天气, 气温, 天气预报]
os: [darwin, linux]
enabled: true
timeout: 60
command: ./weather_cli.sh
parameters:
  city:
    type: string
    required: true
  days:
    type: number
    required: false
---
```

**新格式**：
```yaml
---
name: weather_query
description: 天气查询的标准操作程序
version: 1.0.0
author: mindx
tags: [weather, forecast, general, 天气, 气温, 天气预报, 查询天气]
required_tools: [weather]
---

# Goal

查询全球城市的天气信息，包括当前天气、温度、湿度、风速和未来几天的天气预报

# Triggers

- 用户询问天气情况
- 用户提到"天气"、"气温"、"温度"、"下雨"、"天气预报"
- 用户询问是否需要带伞、穿什么衣服

# SOP

1. 提取城市信息
   - 从用户输入中识别城市名称
   - 如果未指定城市，询问"您想查询哪里的天气？"
   - 如果用户说"这里"或"当前位置"，使用默认城市

2. 确定查询天数
   - 默认查询当天天气
   - 如果用户提到"明天"、"后天"，调整天数
   - 如果用户要求"未来几天"，查询3-7天

3. 调用 weather 工具
   - 传入 city 和 days 参数
   - 获取天气数据

4. 生成响应
   - 格式化天气信息
   - 提供友好的建议（如"建议带伞"、"适合户外活动"）

# Examples

**用户**: 北京天气怎么样？
**助手**: 今天北京的天气是晴，温度15℃，湿度45%，风速3m/s。适合户外活动。

**用户**: 明天上海会下雨吗？
**助手**: 明天上海有小雨，温度12-18℃，湿度80%，建议带伞。

**用户**: 未来三天深圳天气
**助手**: 深圳未来三天天气：
- 今天：多云，22-28℃
- 明天：小雨，20-26℃
- 后天：晴，23-29℃
```

---

### 示例 3: deep_search（复杂型）

**当前格式**：
```yaml
---
name: deep_search
description: 互联网深度搜索技能
version: 1.0.0
category: general
tags: [search, ai, llm, summarize, deep-search]
os: [darwin, linux]
enabled: true
timeout: 180
is_internal: true
guidance: |
  当用户要求"搜一下"、"查一下"、"上网找"时使用
parameters:
  terms:
    type: string
    required: true
---
```

**新格式**：
```yaml
---
name: deep_search
description: 互联网深度搜索的标准操作程序
version: 1.0.0
author: mindx
tags: [search, ai, llm, summarize, deep-search, general, 搜索, 查资料, 上网查]
required_tools: [web_search, http_request, llm_summarize]
optional_tools: [content_extractor]
---

# Goal

进行互联网深度搜索，不仅返回搜索结果，还会阅读相关网页内容，使用 AI 分析和总结，提供综合性的答案和参考链接

# Triggers

- 用户要求"搜一下"、"查一下"、"上网找"、"帮我搜索"
- 用户询问需要最新信息的问题
- 用户要求"详细了解"某个主题
- 用户询问复杂问题，需要多个来源验证

# SOP

1. 理解搜索意图
   - 提取搜索关键词
   - 识别用户真正想了解的内容
   - 如果关键词不明确，询问澄清

2. 执行网页搜索
   - 使用 web_search 工具搜索关键词
   - 获取最多 20 条搜索结果

3. 筛选相关结果
   - 使用 LLM 分析搜索结果的相关性
   - 选择最相关的 3-5 条结果
   - 优先选择权威来源

4. 阅读网页内容
   - 使用 http_request 或 content_extractor 获取网页内容
   - 提取正文，过滤广告和无关内容
   - 处理多个网页（并行或串行）

5. AI 分析和总结
   - 使用 llm_summarize 综合分析所有内容
   - 提取关键信息和要点
   - 生成结构化的总结

6. 生成响应
   - 提供综合性的答案
   - 附上参考链接和来源
   - 如果信息不足，说明局限性

# Examples

**场景1：简单查询**
**用户**: 搜一下什么是机器学习
**助手**: 机器学习是人工智能的一个分支，通过算法让计算机从数据中学习规律，无需明确编程。主要包括监督学习、无监督学习和强化学习三种类型。

参考来源：
- [机器学习入门 - Wikipedia](https://...)
- [机器学习基础 - Stanford](https://...)

**场景2：复杂查询**
**用户**: 帮我查一下最新的 AI 发展趋势
**助手**: 根据最新资料，2026年 AI 发展的主要趋势包括：

1. 多模态大模型的普及
2. AI Agent 的实用化
3. 边缘 AI 的发展
4. AI 安全和伦理的重视

详细分析：
{详细内容...}

参考来源：
- [2026 AI Trends - MIT](https://...)
- [AI发展报告 - 清华](https://...)
- [Industry Analysis - Gartner](https://...)

**场景3：需要澄清**
**用户**: 搜一下 Python
**助手**: 您想了解 Python 的哪方面内容？
- Python 编程语言
- Python 蟒蛇（动物）
- 其他

**用户**: Python 编程语言
**助手**: {提供 Python 编程语言的详细信息...}
```

---

## 📊 迁移优先级

### P0 - 高频使用（优先迁移）

1. web_search
2. deep_search
3. calculator
4. weather
5. terminal
6. notes
7. reminders
8. calendar

### P1 - 中频使用

9. file_search
10. github
11. mail
12. contacts
13. screenshot
14. clipboard
15. open_url

### P2 - 低频使用

16-35. 其他 Skills

---

## ✅ 验收标准

### 自动验证

- [ ] 所有 SKILL.md 包含 YAML frontmatter
- [ ] 所有必需字段存在（name, description, version, tags）
- [ ] 不包含不符合规范的字段（category, os, enabled, timeout, command, parameters）
- [ ] 所有 SKILL.md 包含 Goal, Triggers, SOP, Examples 四个部分

### 人工审核

- [ ] SOP 步骤清晰、可执行
- [ ] Triggers 覆盖主要使用场景
- [ ] Examples 真实、有代表性
- [ ] required_tools 正确映射

### 功能验证

- [ ] 新格式可以被解析器正确解析
- [ ] 向量化索引正常工作
- [ ] 技能匹配准确率 > 85%

---

## 🚀 下一步行动

1. ✅ 完成分析报告（当前步骤）
2. ⏳ 实现自动迁移工具（Step 7）
3. ⏳ 批量迁移 P0 Skills
4. ⏳ 人工审核和调整
5. ⏳ 迁移 P1 和 P2 Skills
6. ⏳ 生成迁移报告

---

**创建时间**：2026-03-05
**分析完成**：2026-03-05
**下次更新**：迁移工具实现后
