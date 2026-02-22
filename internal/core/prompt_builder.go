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

const promptVersion = "v2.0"

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
		parts = append(parts, `## 思考步骤

1. 理解问题：用户真正想要什么？
2. 判断意图：需要实时数据还是常识就能回答？
3. 确定能力：我能否直接回答？
4. 判断是否为定时意图：用户是否要求在特定时间执行某些任务？
5. 判断是否为转发意图：用户是否要求将消息发送到其他渠道？`)
	}

	parts = append(parts, `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 判断能否直接回答
4. 识别是否为定时意图
5. 识别是否为转发意图`)

	parts = append(parts, `## useless 规则

useless=true 仅当用户只说"你好"、"在吗"等闲聊。
useless=false 当用户有具体问题或需求。`)

	parts = append(parts, fmt.Sprintf(`## can_answer 规则

can_answer=false 当问题含以下关键词：%s。
can_answer=true 当闲聊或常识问题。`, b.getSkillKeywordsStr()))

	parts = append(parts, `## 输出格式

输出纯JSON，不要markdown。
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
		parts = append(parts, `## 思考步骤

1. 理解问题：用户真正想要什么？
2. 识别意图和关键词
3. 确定是否需要调用工具
4. 判断是否为定时意图
5. 判断是否为转发意图`)
	}

	parts = append(parts, `## 任务

1. 识别意图和关键词
2. 判断是否为无意义闲聊
3. 如果可以直接回答就给出答案
4. 如果需要调用工具就识别需要什么工具
5. 识别是否为定时意图
6. 识别是否为转发意图`)

	parts = append(parts, `## useless 规则

useless=true 仅当用户只说"你好"、"在吗"等闲聊。
useless=false 当用户有具体问题或需求。`)

	parts = append(parts, `## 输出格式

输出纯JSON，不要markdown。
{"answer":"","intent":"","useless":false,"keywords":[],"send_to":"","has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":""}`)

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
