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
	parts = append(parts, b.getScheduleRulesSegment())
	parts = append(parts, b.getSendToRulesSegment())
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

func (b *PromptBuilder) BuildCloudModel(ctx *PromptContext) string {
	var parts []string

	if ctx.UseThinking {
		parts = append(parts, b.getCloudThinkingSegment())
	}

	parts = append(parts, b.getCloudTaskSegment())
	parts = append(parts, b.getUselessRulesSegment())
	parts = append(parts, b.getScheduleRulesSegment())
	parts = append(parts, b.getSendToRulesSegment())
	parts = append(parts, b.getCloudOutputFormatSegment())

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

func (b *PromptBuilder) getCloudThinkingSegment() string {
	return `## 思考步骤

1. 理解问题：用户真正想要什么？
2. 识别意图和关键词
3. 确定是否需要调用工具
4. 判断是否为定时意图：用户是否要求在特定时间执行某些任务？
5. 判断是否为转发意图：用户是否要求将消息发送到其他渠道？`
}

func (b *PromptBuilder) getCloudTaskSegment() string {
	return `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 如果可以直接回答就给出答案
4. 如果需要调用工具就识别需要什么工具
5. 识别是否为定时意图（如"每周六帮我写日报"、"明天早上8点提醒我开会"等），如果是则在 has_schedule、schedule_name、schedule_cron、schedule_message 字段中填写信息
6. 识别是否为转发意图（如"把这个消息发给微信"等），如果是则在 send_to 字段中填写目标渠道`
}

func (b *PromptBuilder) getCloudOutputFormatSegment() string {
	return `## 输出格式

输出纯JSON，不要markdown。示例：
{"answer":"回复","intent":"意图","useless":false,"keywords":["关键词"],"send_to":"","has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":""}`
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
3. 确定能力：我能否直接回答？
4. 判断是否为定时意图：用户是否要求在特定时间执行某些任务？
5. 判断是否为转发意图：用户是否要求将消息发送到其他渠道？`
}

func (b *PromptBuilder) getTaskSegment() string {
	return `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 判断能否直接回答
4. 识别是否为定时意图（如"每周六帮我写日报"、"明天早上8点提醒我开会"等），如果是则在 has_schedule、schedule_name、schedule_cron、schedule_message 字段中填写信息
5. 识别是否为转发意图（如"把这个消息发给微信"等），如果是则在 send_to 字段中填写目标渠道`
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

func (b *PromptBuilder) getScheduleRulesSegment() string {
	return `## schedule 规则

当用户表达定时意图时（例如"每周六帮我写日报"、"明天早上8点提醒我开会"），请设置：
- has_schedule: true
- schedule_name: 任务名称（例如"每周写日报"）
- schedule_cron: Cron 表达式（例如"0 9 * * 6"表示每周六早上9点）
- schedule_message: 定时要发送的消息（例如"帮我写日报"）

如果没有定时意图，请设置：
- has_schedule: false

Cron 表达式格式：分 时 日 月 周
- 分：0-59
- 时：0-23
- 日：1-31
- 月：1-12
- 周：0-7（0或7都表示周日）

示例：
- 每天早上9点："0 9 * * *"
- 每周一早上8点："0 8 * * 1"
- 每月1号下午3点："0 15 1 * *"
- 每周六早上9点："0 9 * * 6"`
}

func (b *PromptBuilder) getSendToRulesSegment() string {
	return `## send_to 规则

当用户表达转发意图时（例如"把这个消息发给微信"、"转发给QQ"），请在 send_to 字段中填写目标渠道名称。
如果没有转发意图，send_to 字段为空字符串""。`
}

func (b *PromptBuilder) getOutputFormatSegment() string {
	return `## 输出格式

输出纯JSON，不要markdown。示例：
{"answer":"回复","intent":"意图","useless":false,"keywords":["关键词"],"can_answer":false,"has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":"","send_to":""}`
}

var DefaultPromptBuilder = NewPromptBuilder()

func SetSkillKeywords(keywords []string) {
	DefaultPromptBuilder.SetSkillKeywords(keywords)
}

func BuildLeftBrainPrompt(ctx *PromptContext) string {
	return DefaultPromptBuilder.Build(ctx)
}

func BuildCloudModelPrompt(ctx *PromptContext) string {
	return DefaultPromptBuilder.BuildCloudModel(ctx)
}
