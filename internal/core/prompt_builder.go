package core

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"text/template"

	"mindx/prompts"
)

const promptVersion = "v3.0"

type PromptContext struct {
	UsePersona       bool
	UseThinking      bool
	IsLocalModel     bool
	PersonaName      string
	PersonaGender    string
	PersonaCharacter string
	PersonaContent   string
}

// promptTemplateData 模板渲染数据
type promptTemplateData struct {
	UsePersona       bool
	UseThinking      bool
	PersonaName      string
	PersonaGender    string
	PersonaCharacter string
	PersonaContent   string
	SkillKeywords    string
}

type PromptBuilder struct {
	segments       map[string]string
	skillKeywords  []string
	keywordsMu     sync.RWMutex
	localTemplate  *template.Template
	cloudTemplate  *template.Template
}

func NewPromptBuilder() *PromptBuilder {
	pb := &PromptBuilder{
		segments:      make(map[string]string),
		skillKeywords: []string{},
	}

	// 加载嵌入的模板文件
	localTmpl, err := template.ParseFS(prompts.FS, "left_brain_local.tmpl")
	if err != nil {
		log.Printf("警告: 加载本地模型 prompt 模板失败: %v, 使用内置 prompt", err)
	} else {
		pb.localTemplate = localTmpl
	}

	cloudTmpl, err := template.ParseFS(prompts.FS, "left_brain_cloud.tmpl")
	if err != nil {
		log.Printf("警告: 加载云模型 prompt 模板失败: %v, 使用内置 prompt", err)
	} else {
		pb.cloudTemplate = cloudTmpl
	}

	return pb
}

// Version 返回当前 prompt 版本号
func (b *PromptBuilder) Version() string {
	return promptVersion
}

func (b *PromptBuilder) AddSegment(name, content string) *PromptBuilder {
	b.segments[name] = content
	return b
}

// PLACEHOLDER_REST_OF_FILE

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

func (b *PromptBuilder) getSkillKeywordsStr() string {
	b.keywordsMu.RLock()
	defer b.keywordsMu.RUnlock()
	kw := strings.Join(b.skillKeywords, "、")
	if kw == "" {
		kw = "天气、新闻、股价、系统、CPU、内存、邮件、发送、截图"
	}
	return kw
}

// PLACEHOLDER_BUILD_METHODS

func (b *PromptBuilder) Build(ctx *PromptContext) string {
	// 优先使用模板
	if b.localTemplate != nil {
		data := promptTemplateData{
			UsePersona:       ctx.UsePersona,
			UseThinking:      ctx.UseThinking,
			PersonaName:      ctx.PersonaName,
			PersonaGender:    ctx.PersonaGender,
			PersonaCharacter: ctx.PersonaCharacter,
			PersonaContent:   ctx.PersonaContent,
			SkillKeywords:    b.getSkillKeywordsStr(),
		}
		var buf bytes.Buffer
		if err := b.localTemplate.Execute(&buf, data); err == nil {
			return buf.String()
		}
	}

	// 回退到硬编码 prompt
	return b.buildFallback(ctx)
}

func (b *PromptBuilder) BuildCloudModel(ctx *PromptContext) string {
	// 优先使用模板
	if b.cloudTemplate != nil {
		data := promptTemplateData{
			UsePersona:       ctx.UsePersona,
			UseThinking:      ctx.UseThinking,
			PersonaName:      ctx.PersonaName,
			PersonaGender:    ctx.PersonaGender,
			PersonaCharacter: ctx.PersonaCharacter,
			PersonaContent:   ctx.PersonaContent,
			SkillKeywords:    b.getSkillKeywordsStr(),
		}
		var buf bytes.Buffer
		if err := b.cloudTemplate.Execute(&buf, data); err == nil {
			return buf.String()
		}
	}

	// 回退到硬编码 prompt
	return b.buildCloudFallback(ctx)
}

// PLACEHOLDER_FALLBACK_METHODS

// buildFallback 使用硬编码 prompt（模板加载失败时的回退）
func (b *PromptBuilder) buildFallback(ctx *PromptContext) string {
	var parts []string

	if ctx.UseThinking {
		parts = append(parts, `## 思考步骤（请在内心逐步推理，不要输出推理过程）

1. 用户说了什么？→ 提取核心诉求
2. 这是闲聊还是有具体需求？→ 判断 useless
3. 需要实时数据/外部工具，还是常识就能回答？→ 判断 can_answer
4. 是否包含定时/转发意图？→ 判断 schedule/send_to
5. 如果有多个意图，以主要意图为准`)
	}

	parts = append(parts, `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 判断能否直接回答
4. 识别是否为定时意图，如果是则填写 has_schedule、schedule_name、schedule_cron、schedule_message
5. 识别是否为转发意图，如果是则在 send_to 填写目标渠道`)

	parts = append(parts, `## useless 规则

useless=true 仅当用户只说"你好"、"在吗"、"嗯"等无实质内容的闲聊。
useless=false 当用户有任何具体问题、需求、情绪表达或信息分享。`)

	parts = append(parts, fmt.Sprintf(`## can_answer 规则

can_answer=false 当问题含以下关键词：%s。
can_answer=false 当问题需要实时数据、外部工具或上下文才能回答。
can_answer=false 当问题涉及个人数据（联系人、电话号码、邮箱、地址、文件内容、账户信息等），你没有这些数据，绝对不能编造。
can_answer=true 当闲聊或常识问题，你可以直接给出答案。
重要：当 can_answer=false 时，answer 必须为空字符串""，不要编造任何具体数据。`, b.getSkillKeywordsStr()))

	parts = append(parts, `## 输出格式

输出纯JSON，不要markdown，不要解释。
{"answer":"","intent":"","useless":false,"keywords":[],"can_answer":false,"has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":"","send_to":""}`)

	if ctx.UsePersona {
		persona := fmt.Sprintf("## 人设\n\n- 姓名: %s\n- 性别: %s\n- 性格: %s\n\n%s",
			ctx.PersonaName, ctx.PersonaGender, ctx.PersonaCharacter, ctx.PersonaContent)
		parts = append([]string{persona}, parts...)
	}

	return strings.Join(parts, "\n\n")
}

// PLACEHOLDER_CLOUD_FALLBACK

func (b *PromptBuilder) buildCloudFallback(ctx *PromptContext) string {
	var parts []string

	if ctx.UseThinking {
		parts = append(parts, `## 思考步骤（请在内心逐步推理，不要输出推理过程）

1. 用户说了什么？→ 提取核心诉求
2. 这是闲聊还是有具体需求？→ 判断 useless
3. 需要实时数据/外部工具，还是常识就能回答？→ 判断 can_answer
4. 是否包含定时/转发意图？→ 判断 schedule/send_to
5. 如果有多个意图，以主要意图为准`)
	}

	parts = append(parts, `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 如果可以直接回答就给出答案
4. 如果需要调用工具就识别需要什么工具
5. 识别是否为定时意图，如果是则填写 has_schedule、schedule_name、schedule_cron、schedule_message
6. 识别是否为转发意图，如果是则在 send_to 填写目标渠道`)

	parts = append(parts, `## useless 规则

useless=true 仅当用户只说"你好"、"在吗"、"嗯"等无实质内容的闲聊。
useless=false 当用户有任何具体问题、需求、情绪表达或信息分享。`)

	parts = append(parts, fmt.Sprintf(`## can_answer 规则

can_answer=false 当问题含以下关键词：%s。
can_answer=false 当问题需要实时数据、外部工具或上下文才能回答。
can_answer=false 当问题涉及个人数据（联系人、电话号码、邮箱、地址、文件内容、账户信息等），你没有这些数据，绝对不能编造。
can_answer=true 当闲聊或常识问题，你可以直接给出答案。
重要：当 can_answer=false 时，answer 必须为空字符串""，不要编造任何具体数据。`, b.getSkillKeywordsStr()))

	parts = append(parts, `## 输出格式

输出纯JSON，不要markdown，不要解释。
{"answer":"","intent":"","useless":false,"keywords":[],"can_answer":false,"has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":"","send_to":""}`)

	if ctx.UsePersona {
		persona := fmt.Sprintf("## 人设\n\n- 姓名: %s\n- 性别: %s\n- 性格: %s\n\n%s",
			ctx.PersonaName, ctx.PersonaGender, ctx.PersonaCharacter, ctx.PersonaContent)
		parts = append([]string{persona}, parts...)
	}

	return strings.Join(parts, "\n\n")
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

func PromptVersion() string {
	return DefaultPromptBuilder.Version()
}
