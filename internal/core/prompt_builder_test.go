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
