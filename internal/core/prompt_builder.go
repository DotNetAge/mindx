package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type PromptContext struct {
	UsePersona       bool
	UseThinking      bool
	IsLocalModel     bool
	PersonaName      string
	PersonaGender    string
	PersonaCharacter string
	PersonaContent   string
}

type PromptBuilder struct {
	segments      map[string]string
	skillKeywords []string
	keywordsMu    sync.RWMutex
}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		segments:      make(map[string]string),
		skillKeywords: []string{},
	}
}

func (b *PromptBuilder) AddSegment(name, content string) *PromptBuilder {
	b.segments[name] = content
	return b
}

func (b *PromptBuilder) SetSkillKeywords(keywords []string) {
	b.keywordsMu.Lock()
	defer b.keywordsMu.Unlock()

	uniqueKeywords := make(map[string]bool)
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" && len([]rune(kw)) <= 10 {
			uniqueKeywords[kw] = true
		}
	}

	b.skillKeywords = make([]string, 0, len(uniqueKeywords))
	for kw := range uniqueKeywords {
		b.skillKeywords = append(b.skillKeywords, kw)
	}
	sort.Strings(b.skillKeywords)
}

func (b *PromptBuilder) GetSkillKeywords() []string {
	b.keywordsMu.RLock()
	defer b.keywordsMu.RUnlock()
	return b.skillKeywords
}

func (b *PromptBuilder) Build(ctx *PromptContext) string {
	var parts []string

	if ctx.UseThinking {
		parts = append(parts, b.getThinkingSegment())
	}

	parts = append(parts, b.getTaskSegment())
	parts = append(parts, b.getUselessRulesSegment())
	parts = append(parts, b.getCanAnswerRulesSegment(ctx.IsLocalModel))
	parts = append(parts, b.getOutputFormatSegment())

	if ctx.UsePersona {
		parts = append([]string{b.getPersonaSegment(ctx)}, parts...)
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "\n\n"
		}
		result += part
	}
	return result
}

func (b *PromptBuilder) getPersonaSegment(ctx *PromptContext) string {
	return fmt.Sprintf(`## 人设

- 姓名: %s
- 性别: %s
- 性格: %s

%s`, ctx.PersonaName, ctx.PersonaGender, ctx.PersonaCharacter, ctx.PersonaContent)
}

func (b *PromptBuilder) getThinkingSegment() string {
	return `## 思考步骤

1. 理解问题：用户真正想要什么？
2. 判断意图：需要实时数据还是常识就能回答？
3. 确定能力：我能否直接回答？`
}

func (b *PromptBuilder) getTaskSegment() string {
	return `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 判断能否直接回答`
}

func (b *PromptBuilder) getUselessRulesSegment() string {
	return `## useless 规则

useless=true 仅当用户只说"你好"、"在吗"等闲聊。
useless=false 当用户有具体问题或需求。`
}

func (b *PromptBuilder) getCanAnswerRulesSegment(isLocalModel bool) string {
	b.keywordsMu.RLock()
	keywords := b.skillKeywords
	b.keywordsMu.RUnlock()

	if isLocalModel {
		keywordStr := strings.Join(keywords, "、")
		if keywordStr == "" {
			keywordStr = "天气、新闻、股价、系统、CPU、内存、邮件、发送、截图"
		}
		return fmt.Sprintf(`## can_answer 规则

can_answer=false 当问题含以下关键词：%s。
can_answer=true 当闲聊或常识问题。`, keywordStr)
	}
	return `## can_answer 规则

can_answer=true：闲聊、常识问题、已有知识可回答。
can_answer=false：需要实时数据、外部工具、用户数据、专业知识。`
}

func (b *PromptBuilder) getOutputFormatSegment() string {
	return `## 输出格式

输出纯JSON，不要markdown。示例：
{"answer":"回复","intent":"意图","useless":false,"keywords":["关键词"],"can_answer":false}`
}

var DefaultPromptBuilder = NewPromptBuilder()

func SetSkillKeywords(keywords []string) {
	DefaultPromptBuilder.SetSkillKeywords(keywords)
}

func BuildLeftBrainPrompt(ctx *PromptContext) string {
	return DefaultPromptBuilder.Build(ctx)
}
