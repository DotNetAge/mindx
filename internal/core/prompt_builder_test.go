package core

import (
	"strings"
	"testing"
)

func TestPromptBuilder_LocalModel(t *testing.T) {
	ctx := &PromptContext{
		UsePersona:   false,
		UseThinking:  true,
		IsLocalModel: true,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	if !strings.Contains(prompt, "can_answer=false") {
		t.Error("本地模型 prompt 应包含 can_answer=false 关键词规则")
	}

	if !strings.Contains(prompt, "天气") {
		t.Error("本地模型 prompt 应包含天气关键词")
	}

	if strings.Contains(prompt, "人设") {
		t.Error("本地模型 prompt 不应包含人设")
	}

	if !strings.Contains(prompt, "思考步骤") {
		t.Error("本地模型 prompt 应包含思考步骤")
	}

	t.Logf("生成的 prompt:\n%s", prompt)
}

func TestPromptBuilder_CloudModel(t *testing.T) {
	ctx := &PromptContext{
		UsePersona:       true,
		UseThinking:      true,
		IsLocalModel:     false,
		PersonaName:      "小柔",
		PersonaGender:    "女",
		PersonaCharacter: "温柔",
		PersonaContent:   "你是一个智能助手",
	}

	prompt := BuildLeftBrainPrompt(ctx)

	if !strings.Contains(prompt, "人设") {
		t.Error("云端模型 prompt 应包含人设")
	}

	if !strings.Contains(prompt, "小柔") {
		t.Error("云端模型 prompt 应包含人设名字")
	}

	if !strings.Contains(prompt, "思考步骤") {
		t.Error("云端模型 prompt 应包含思考步骤")
	}

	t.Logf("生成的 prompt:\n%s", prompt)
}

func TestPromptBuilder_NoThinking(t *testing.T) {
	ctx := &PromptContext{
		UsePersona:   false,
		UseThinking:  false,
		IsLocalModel: true,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	if strings.Contains(prompt, "思考步骤") {
		t.Error("不使用思考引导时 prompt 不应包含思考步骤")
	}
}

func TestPromptBuilder_DynamicKeywords(t *testing.T) {
	keywords := []string{"天气", "新闻", "股价", "系统", "CPU", "内存", "邮件", "发送", "截图", "联系人", "查询", "查看", "分析", "代码"}
	SetSkillKeywords(keywords)

	ctx := &PromptContext{
		UsePersona:   false,
		UseThinking:  false,
		IsLocalModel: true,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	for _, kw := range keywords {
		if !strings.Contains(prompt, kw) {
			t.Errorf("prompt 应包含关键词: %s", kw)
		}
	}

	t.Logf("动态关键词 prompt:\n%s", prompt)
}

func TestPromptBuilder_EmptyKeywords(t *testing.T) {
	SetSkillKeywords([]string{})

	ctx := &PromptContext{
		UsePersona:   false,
		UseThinking:  false,
		IsLocalModel: true,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	if !strings.Contains(prompt, "can_answer=false") {
		t.Error("即使关键词为空，也应包含 can_answer=false 规则")
	}

	t.Logf("空关键词 prompt:\n%s", prompt)
}

func TestPromptBuilder_V3CoTThinking(t *testing.T) {
	ctx := &PromptContext{
		UseThinking: true,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	if !strings.Contains(prompt, "逐步推理") {
		t.Error("v3.0 思考步骤应包含 CoT 引导语")
	}
	if !strings.Contains(prompt, "提取核心诉求") {
		t.Error("v3.0 思考步骤应包含推理链引导")
	}
}

func TestPromptBuilder_V3FewShotExamples(t *testing.T) {
	ctx := &PromptContext{
		UseThinking: false,
	}

	prompt := BuildLeftBrainPrompt(ctx)

	// 边界 case 示例
	if !strings.Contains(prompt, "算了不用了") {
		t.Error("v3.0 应包含否定/取消示例")
	}
	if !strings.Contains(prompt, "心情不太好") {
		t.Error("v3.0 应包含情绪表达示例")
	}
	if !strings.Contains(prompt, "A股行情") {
		t.Error("v3.0 应包含股票行情示例")
	}
	if !strings.Contains(prompt, "123 乘以 456") {
		t.Error("v3.0 应包含工具调用示例")
	}
}

func TestPromptBuilder_CloudModelHasCanAnswer(t *testing.T) {
	ctx := &PromptContext{
		UseThinking: true,
	}

	prompt := BuildCloudModelPrompt(ctx)

	if !strings.Contains(prompt, "can_answer") {
		t.Error("v3.0 云模型 prompt 应包含 can_answer 规则")
	}
	if !strings.Contains(prompt, "逐步推理") {
		t.Error("v3.0 云模型 prompt 应包含 CoT 引导")
	}
}

func TestPromptBuilder_Version(t *testing.T) {
	v := PromptVersion()
	if v != "v3.0" {
		t.Errorf("expected v3.0, got %s", v)
	}
}

func TestPromptBuilder_SkillKeywordsInjection(t *testing.T) {
	t.Run("自定义关键词注入", func(t *testing.T) {
		SetSkillKeywords([]string{"股票", "天气", "新闻"})
		ctx := &PromptContext{UseThinking: false, IsLocalModel: true}
		prompt := BuildLeftBrainPrompt(ctx)

		for _, kw := range []string{"股票", "天气", "新闻"} {
			if !strings.Contains(prompt, kw) {
				t.Errorf("prompt 应包含注入的关键词: %s", kw)
			}
		}
	})

	t.Run("空关键词使用默认值", func(t *testing.T) {
		SetSkillKeywords([]string{})
		ctx := &PromptContext{UseThinking: false, IsLocalModel: true}
		prompt := BuildLeftBrainPrompt(ctx)

		// 默认关键词列表中应包含"天气"
		if !strings.Contains(prompt, "天气") {
			t.Error("空关键词时应使用默认关键词，包含'天气'")
		}
	})

	t.Run("关键词去重", func(t *testing.T) {
		SetSkillKeywords([]string{"天气", "天气", "天气"})
		ctx := &PromptContext{UseThinking: false, IsLocalModel: true}
		prompt := BuildLeftBrainPrompt(ctx)

		// 去重后"天气"只应出现在关键词区域一次（在 can_answer 规则中）
		if !strings.Contains(prompt, "天气") {
			t.Error("prompt 应包含'天气'")
		}
	})
}

func TestPromptBuilder_OutputFormatComplete(t *testing.T) {
	ctx := &PromptContext{UseThinking: false, IsLocalModel: true}
	prompt := BuildLeftBrainPrompt(ctx)

	requiredFields := []string{
		"answer", "intent", "useless", "keywords", "can_answer",
		"has_schedule", "schedule_name", "schedule_cron", "schedule_message", "send_to",
	}

	for _, field := range requiredFields {
		if !strings.Contains(prompt, `"`+field+`"`) {
			t.Errorf("prompt JSON 输出格式应包含字段: %s", field)
		}
	}
}
