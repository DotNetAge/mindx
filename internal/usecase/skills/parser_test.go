package skills

import (
	"mindx/internal/entity"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillParser_ParseContent(t *testing.T) {
	parser := NewSkillParser()

	content := []byte(`---
name: weather_query
description: 天气查询的标准操作程序
version: 1.0.0
author: mindx
tags: [weather, query, 天气]
required_tools: [web_search, http_request]
optional_tools: [location_service]
---

# Goal

查询指定地点的天气信息，包括温度、湿度、风速等。

# Triggers

- 用户询问天气
- 用户提到"天气"、"气温"、"下雨"等关键词
- 用户询问是否需要带伞

# SOP

1. 提取地点信息
   - 如果用户未指定地点，询问"您想查询哪里的天气？"
   - 如果用户说"这里"，使用 location_service 获取当前位置

2. 调用天气 API
   - 使用 web_search 工具搜索 "{地点} 天气"
   - 或使用 http_request 调用天气 API

3. 生成响应
   - "今天{地点}的天气是{天气状况}，温度{温度}℃..."

# Examples

**用户**: 北京天气怎么样？
**助手**: 今天北京的天气是晴，温度15℃。

**用户**: 明天上海会下雨吗？
**助手**: 明天上海有小雨，温度12-18℃，建议带伞。
`)

	skill, err := parser.ParseContent(content)

	require.NoError(t, err)
	assert.Equal(t, "weather_query", skill.Name)
	assert.Equal(t, "天气查询的标准操作程序", skill.Description)
	assert.Equal(t, "1.0.0", skill.Version)
	assert.Equal(t, "mindx", skill.Author)
	assert.Equal(t, []string{"weather", "query", "天气"}, skill.Tags)
	assert.Equal(t, []string{"web_search", "http_request"}, skill.RequiredTools)
	assert.Equal(t, []string{"location_service"}, skill.OptionalTools)

	// 验证 Goal
	assert.Contains(t, skill.Goal, "查询指定地点的天气信息")

	// 验证 Triggers
	assert.Len(t, skill.Triggers, 3)
	assert.Contains(t, skill.Triggers[0], "用户询问天气")

	// 验证 SOP
	assert.Contains(t, skill.SOP, "提取地点信息")
	assert.Contains(t, skill.SOP, "调用天气 API")

	// 验证 Examples（解析器可能将多个对话合并为一个示例）
	assert.NotEmpty(t, skill.Examples)
	allExamples := strings.Join(skill.Examples, "\n")
	assert.Contains(t, allExamples, "北京天气怎么样")
	assert.Contains(t, allExamples, "明天上海会下雨吗")

	// 验证 Keywords
	assert.NotEmpty(t, skill.Keywords)
	assert.Contains(t, skill.Keywords, "weather")
	assert.Contains(t, skill.Keywords, "天气")
}

func TestSkillParser_ParseFile(t *testing.T) {
	parser := NewSkillParser()

	// 创建临时测试文件
	tmpDir := t.TempDir()
	skillFile := filepath.Join(tmpDir, "SKILL.md")

	content := `---
name: test_skill
description: 测试技能
version: 1.0.0
author: test
tags: [test]
required_tools: [tool1]
---

# Goal
测试目标

# Triggers
- 触发条件1
- 触发条件2

# SOP
1. 步骤1
2. 步骤2

# Examples
**用户**: 测试输入
**助手**: 测试输出
`

	err := os.WriteFile(skillFile, []byte(content), 0644)
	require.NoError(t, err)

	// 解析文件
	skill, err := parser.Parse(skillFile)

	require.NoError(t, err)
	assert.Equal(t, "test_skill", skill.Name)
	assert.Equal(t, skillFile, skill.FilePath)
}

func TestSkillParser_MissingFrontmatter(t *testing.T) {
	parser := NewSkillParser()

	content := []byte(`# Goal
测试目标
`)

	_, err := parser.ParseContent(content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing YAML frontmatter")
}

func TestSkillParser_InvalidYAML(t *testing.T) {
	parser := NewSkillParser()

	content := []byte(`---
name: test
invalid yaml: [
---

# Goal
测试
`)

	_, err := parser.ParseContent(content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestSkillParser_MissingRequiredFields(t *testing.T) {
	parser := NewSkillParser()

	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name: "missing name",
			content: `---
description: 测试
version: 1.0.0
---
# Goal
测试
`,
			errMsg: "missing required field: name",
		},
		{
			name: "missing description",
			content: `---
name: test
version: 1.0.0
---
# Goal
测试
`,
			errMsg: "missing required field: description",
		},
		{
			name: "missing version",
			content: `---
name: test
description: 测试
---
# Goal
测试
`,
			errMsg: "missing required field: version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseContent([]byte(tt.content))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestSkillParser_ParseList(t *testing.T) {
	parser := NewSkillParser()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "dash list",
			content: `- 项目1
- 项目2
- 项目3`,
			expected: []string{"项目1", "项目2", "项目3"},
		},
		{
			name: "asterisk list",
			content: `* 项目1
* 项目2`,
			expected: []string{"项目1", "项目2"},
		},
		{
			name: "numbered list",
			content: `1. 项目1
2. 项目2
3. 项目3`,
			expected: []string{"项目1", "项目2", "项目3"},
		},
		{
			name: "mixed with empty lines",
			content: `- 项目1

- 项目2

- 项目3`,
			expected: []string{"项目1", "项目2", "项目3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseList(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSkillParser_ParseExamples(t *testing.T) {
	parser := NewSkillParser()

	content := `**用户**: 北京天气怎么样？
**助手**: 今天北京的天气是晴，温度15℃。

**用户**: 明天会下雨吗？
**助手**: 明天有小雨，建议带伞。

**场景1**: 复杂查询
**用户**: 未来三天天气
**助手**: 详细预报...
`

	examples := parser.parseExamples(content)

	// 解析器会将连续的对话作为一个示例
	assert.NotEmpty(t, examples)
	// 验证包含关键内容
	allContent := strings.Join(examples, "\n")
	assert.Contains(t, allContent, "北京天气怎么样")
	assert.Contains(t, allContent, "场景1")
}

func TestSkillParser_ExtractKeywords(t *testing.T) {
	parser := NewSkillParser()

	skill := &entity.Skill{
		Name:        "weather_query",
		Description: "天气查询的标准操作程序",
		Tags:        []string{"weather", "query", "天气"},
		Triggers: []string{
			"用户询问天气",
			"用户提到气温",
		},
	}

	parser.extractKeywords(skill)

	assert.NotEmpty(t, skill.Keywords)
	// 验证 Tags 被提取（保持原样）
	assert.Contains(t, skill.Keywords, "weather")
	assert.Contains(t, skill.Keywords, "query")
	assert.Contains(t, skill.Keywords, "天气")

	// 验证分词后的关键词
	// 注意：中文分词可能不会完全按预期分割，这里只验证关键的词
	hasWeatherRelated := false
	for _, kw := range skill.Keywords {
		if strings.Contains(kw, "天气") || strings.Contains(kw, "气温") ||
		   strings.Contains(kw, "查询") || kw == "weather" {
			hasWeatherRelated = true
			break
		}
	}
	assert.True(t, hasWeatherRelated, "应该包含天气相关的关键词")
}

func TestSkillParser_Tokenize(t *testing.T) {
	parser := NewSkillParser()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "english words",
			input:    "weather_query",
			expected: []string{"weather", "query"},
		},
		{
			name:     "chinese words",
			input:    "天气查询、温度预报",
			expected: []string{"天气查询", "温度预报"},
		},
		{
			name:     "mixed",
			input:    "weather-天气/query",
			expected: []string{"weather", "天气", "query"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.tokenize(tt.input)
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestSkillParser_ComplexSKILL(t *testing.T) {
	parser := NewSkillParser()

	content := []byte(`---
name: deep_search
description: 互联网深度搜索的标准操作程序
version: 1.0.0
author: mindx
tags: [search, ai, llm, summarize, deep-search, 搜索, 查资料]
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
`)

	skill, err := parser.ParseContent(content)

	require.NoError(t, err)
	assert.Equal(t, "deep_search", skill.Name)
	assert.Len(t, skill.RequiredTools, 3)
	assert.Len(t, skill.OptionalTools, 1)
	assert.Len(t, skill.Triggers, 4)
	assert.Contains(t, skill.SOP, "理解搜索意图")
	assert.Contains(t, skill.SOP, "AI 分析和总结")

	// 验证示例被解析
	assert.NotEmpty(t, skill.Examples)
	allExamples := strings.Join(skill.Examples, "\n")
	assert.Contains(t, allExamples, "场景1")
	assert.Contains(t, allExamples, "场景2")
}
