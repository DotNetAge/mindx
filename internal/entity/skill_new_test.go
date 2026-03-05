package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSkill_GetEmbeddingText(t *testing.T) {
	skill := &Skill{
		Name: "weather_query",
		Goal: "查询指定地点的天气信息",
		Triggers: []string{
			"用户询问天气",
			"用户提到天气、气温关键词",
		},
	}

	text := skill.GetEmbeddingText()

	assert.Contains(t, text, "查询指定地点的天气信息")
	assert.Contains(t, text, "用户询问天气")
	assert.Contains(t, text, "用户提到天气、气温关键词")
}

func TestSkill_HasTool(t *testing.T) {
	skill := &Skill{
		RequiredTools: []string{"web_search", "http_request"},
		OptionalTools: []string{"location_service"},
	}

	// 测试必需工具
	assert.True(t, skill.HasTool("web_search"))
	assert.True(t, skill.HasTool("http_request"))

	// 测试可选工具
	assert.True(t, skill.HasTool("location_service"))

	// 测试不存在的工具
	assert.False(t, skill.HasTool("unknown_tool"))
}

func TestSkill_ToSOP(t *testing.T) {
	skill := &Skill{
		Name:          "weather_query",
		Description:   "天气查询的标准操作程序",
		Keywords:      []string{"天气", "气温"},
		RequiredTools: []string{"web_search"},
		SOP:           "1. 提取地点\n2. 调用 API\n3. 生成响应",
	}

	sop := skill.ToSOP()

	assert.Equal(t, "weather_query", sop.Name)
	assert.Equal(t, "天气查询的标准操作程序", sop.Description)
	assert.Equal(t, []string{"天气", "气温"}, sop.Keywords)
	assert.Equal(t, []string{"web_search"}, sop.RequiredTools)
	assert.Contains(t, sop.SOPContent, "提取地点")
}

func TestSkillMatch(t *testing.T) {
	skill := &Skill{
		Name: "test_skill",
		Goal: "测试技能",
	}

	match := &SkillMatch{
		Skill: skill,
		Score: 0.85,
	}

	assert.Equal(t, "test_skill", match.Skill.Name)
	assert.Equal(t, 0.85, match.Score)
}

func TestSkill_FullLifecycle(t *testing.T) {
	// 创建完整的 Skill
	now := time.Now()
	skill := &Skill{
		Name:        "math_calculation",
		Description: "数学计算的标准操作程序",
		Version:     "1.0.0",
		Author:      "mindx",
		Goal:        "执行数学计算和运算表达式",
		Triggers: []string{
			"用户要求计算",
			"用户提到算一下、计算",
		},
		SOP: `1. 识别数学表达式
2. 调用 calculator 工具
3. 生成响应`,
		Examples: []string{
			"用户: 2+3*4 等于多少？\n助手: 计算结果是 14",
		},
		RequiredTools: []string{"calculator"},
		Tags:          []string{"calculator", "math", "计算"},
		Keywords:      []string{"计算", "数学", "运算"},
		FilePath:      "/path/to/math_calculation/SKILL.md",
		UpdatedAt:     now,
		CreatedAt:     now,
	}

	// 验证基础信息
	assert.Equal(t, "math_calculation", skill.Name)
	assert.Equal(t, "1.0.0", skill.Version)
	assert.Equal(t, "mindx", skill.Author)

	// 验证核心内容
	assert.NotEmpty(t, skill.Goal)
	assert.Len(t, skill.Triggers, 2)
	assert.NotEmpty(t, skill.SOP)
	assert.Len(t, skill.Examples, 1)

	// 验证工具依赖
	assert.Len(t, skill.RequiredTools, 1)
	assert.True(t, skill.HasTool("calculator"))

	// 验证索引
	assert.Len(t, skill.Tags, 3)
	assert.Len(t, skill.Keywords, 3)

	// 验证 GetEmbeddingText
	embeddingText := skill.GetEmbeddingText()
	assert.Contains(t, embeddingText, "执行数学计算")
	assert.Contains(t, embeddingText, "用户要求计算")

	// 验证 ToSOP
	sop := skill.ToSOP()
	assert.Equal(t, "math_calculation", sop.Name)
	assert.Contains(t, sop.SOPContent, "调用 calculator 工具")
}
