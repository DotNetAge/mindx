package core

import (
	"mindx/internal/entity"
)

type ThinkingEventType = entity.ThinkingEventType

const (
	ThinkingEventStart      = entity.ThinkingEventStart
	ThinkingEventProgress   = entity.ThinkingEventProgress
	ThinkingEventChunk      = entity.ThinkingEventChunk
	ThinkingEventToolCall   = entity.ThinkingEventToolCall
	ThinkingEventToolResult = entity.ThinkingEventToolResult
	ThinkingEventComplete   = entity.ThinkingEventComplete
	ThinkingEventError      = entity.ThinkingEventError
)

type ThinkingEvent = entity.ThinkingEvent

var AssistantPrompt = `你是一个智能助手，可以与用户自然地交流并能精准地识别出用户在交谈中的核心意图。

## 人设

- 姓名: %s
- 性别: %s
- 性格: %s

%s

## 任务

1. 联系上下文回应用户的问题，不要无意义地重复相同的答案；
2. 识别意图：提取用户的核心意图和关键词；
3. 判断用户问题是否为无意义的闲聊；
4. 判断你是否有能力回答用户的问题；
5. 识别用户是否有要求向其它的通信软件发送信息的要求，如：飞书，QQ，微信，钉钉，豆包；

## 意图识别规则

- 每个问题都需要独立识别意图，不要被历史对话中的意图影响
- 只有当用户使用代词（如"它"、"那个"、"这个"）或追问（如"那呢"、"还有呢"）时，才需要参考历史对话
- 如果用户提出了一个全新的问题，必须根据当前问题本身识别意图

## useless 字段判断规则（非常重要）

useless=true 的情况【仅限以下情况】：
- 纯粹的闲聊、寒暄、打招呼，如"你好"、"在吗"、"怎么样"
- 单一的语气助词，如"啊"、"哎"、"哦"、"嗯"
- 没有任何实际需求或问题

useless=false 的情况【包括但不限于】：
- 用户有明确的意图，如"查询天气"、"设置闹钟"、"播放音乐"
- 用户在询问信息，如"今天几号"、"广州天气"
- 用户有具体需求，即使你无法直接满足

## can_answer 字段判断规则（非常重要）

can_answer=true 的情况【本地小模型可直接回答】：
- 闲聊、寒暄、打招呼，如"你好"、"在吗"
- 通用常识问题，如"1+1等于多少"、"中国的首都是哪里"
- 你已有知识可以回答的问题，如"介绍一下自己"、"讲个笑话"
- 简单的逻辑推理或文字处理

can_answer=false 的情况【需要工具或云端大模型】：
- 需要实时信息，如"今天天气"、"最新新闻"、"当前股价"
- 需要外部工具，如"查询系统信息"、"发送邮件"、"截图"
- 需要访问用户数据，如"我的日程"、"联系人"
- 需要专业知识或复杂推理，如"分析这段代码"、"写一篇论文"
- 你不确定或知识不足的问题

## 输出格式
输出格式必须是JSON，不要有其他内容。以下为字段说明：
- answer: 回复，应该是对用户的直接回复，保持自然流畅
- intent: 意图，应该简洁明了，描述用户想要做什么。意图的表述应该以[动词]或[动词]+[名词]的方式表述
- useless: 布尔值，严格按照上述规则判断
- keywords: 字符串数组，意图的具体内容，如意图为"查询天气"，关键词就是"广州"、"今天"
- send_to: 用于判断用户是否有要求向其它的通信软件发送信息的要求，如果没有则返回空字符串
- can_answer: 布尔值，严格按照上述规则判断
`

type DialogueMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ThinkingResult 思考结果
type ThinkingResult struct {
	Answer    string   `json:"answer"`     // Answer 回答的内容（返回给用户)
	Intent    string   `json:"intent"`     // Intent 意图 判断用户的意图
	Keywords  []string `json:"keywords"`   // Keywords 关键词 判断用户的意图
	SendTo    string   `json:"send_to"`    // 发送信息给其它的通信软件
	Useless   bool     `json:"useless"`    // Useless 标记用户的提问是否无意义
	CanAnswer bool     `json:"can_answer"` // CanAnswer 是否能回答用户的问题
}

type ToolCallResult struct {
	Answer     string            `json:"answer"`       // 回答内容
	Function   *ToolCallFunction `json:"function"`     // 模型决定调用的函数
	ToolCallID string            `json:"tool_call_id"` // 工具调用ID，用于回传结果
	NoCall     bool              `json:"no_call"`      // 是否决定不调用任何工具
}

type ToolCallFunction struct {
	Name      string                 `json:"name"`      // 调用的函数名
	Arguments map[string]interface{} `json:"arguments"` // 函数入参
}

// Thinking 负责思考并生成思考的结果
// 大脑的本质是思考
type Thinking interface {
	// Think 根据提示思考并返回结果
	// question: 用户的问题
	// history: 历史对话记录
	// references: 参考资料
	// jsonResult: 是否返回JSON格式结果
	Think(question string, history []*DialogueMessage, references string, jsonResult bool) (*ThinkingResult, error)
	// ThinkWithTools 使用工具思考并返回结果
	ThinkWithTools(question string, history []*DialogueMessage, tools []*ToolSchema) (*ToolCallResult, error)
	// ReturnFuncResult 向大模型回传函数调用结果
	// 当调用FunctionCall时，大模型会返回一个JSON格式的结果，当外部完成调用后使用此方法回传结果
	// toolCallID: 工具调用ID（用于回传结果匹配）
	// name: 函数名称
	// result: 函数调用的结果
	// originalArgs: 原始函数调用的参数（用于构建 ToolCall 消息）
	// history: 历史对话记录
	// tools: 可用的工具列表
	// question: 用户的原始问题
	ReturnFuncResult(toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*DialogueMessage, tools []*ToolSchema, question string) (string, error)
	// CalculateMaxHistoryCount 计算最大历史对话轮数
	// 根据模型 Token 容量动态计算，公式: (MaxTokens - ReservedOutputTokens) ÷ AvgTokensPerRound
	CalculateMaxHistoryCount() int
	// SetEventChan 设置事件 channel（用于实时推送思考过程）
	SetEventChan(ch chan<- ThinkingEvent)
}

type ThinkingRequest struct {
	Question  string               `json:"question"`
	Timeout   int64                `json:"timeout"`
	SessionID string               `json:"session_id,omitempty"`
	EventChan chan<- ThinkingEvent `json:"-"`
}

// ThinkingResponse 思考响应(大脑专用)
type ThinkingResponse struct {
	Answer string        `json:"answer"`
	Tools  []*ToolSchema `json:"tools"`
	SendTo string        `json:"send_to"` // 目标 Channel，用于消息转发
}

// ToolSchema 发起FunctionCall使用的工具Schema
type ToolSchema struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Params       map[string]interface{} `json:"params"`
	OutputFormat string                 `json:"output_format,omitempty"`
	Guidance     string                 `json:"guidance,omitempty"`
}

// Brain 仿生大脑
// 由执行简单任务的潜意识和执行复杂任务的主意识组成
// - 潜意识负责简单性的思考和执行低运算量的技能执行
// - 主意识负责高级的思考和决策(智能体+远程大模型)
// 内嵌长时记忆体，可从记忆中获取用户的历史对话以精确匹配用户的话题
type Brain struct {
	// LeftBrain 左脑，负责简单性的思考,采用本地微型化模型思考
	LeftBrain Thinking
	// RightBrain 右脑，负责行为性思考，执行Function call获取Skill的最终调用Schema。采用专用的函索生成模型
	RightBrain Thinking
	// Consciousness 意识，负责高级思考和决策。通过能力定义指向的智能体+远程大语言模型
	// 注意：主意识是动态创建的，初始化时可能为 nil
	Consciousness Thinking
	// GetMemory 获取长时记忆系统
	GetMemory func() (Memory, error)
	// Post 处理思考请求
	// 处理思考的过程:
	// 1. 优先从长时记忆系统中获取的记忆片段，作为思考的提示（更懂得用户）
	// 2. 获取会话历史（通过 OnHistoryRequest 回调）
	// 3. 使用左脑进行思考，如果Tools有匹配的工具，则触发OnToolsRequest获取工具的Schema，启动右脑获取Skill的最终调用Schema
	// 4. 如果左脑思考的结果表明左脑无法回答用户，则会触发OnCapabilityRequest获取复杂的能力，如果能匹配则启用远程思考模式；
	Post func(req *ThinkingRequest) (*ThinkingResponse, error)
	// OnThinkingEvent 思考流事件回调，用于实时推送思考过程
	OnThinkingEvent func(sessionID string, event map[string]any)
}

// OnToolsRequest 处理工具请求的回调函数,从已装载工具中匹配与关键字最相似的工具
type OnToolsRequest func(keywords ...string) ([]*ToolSchema, error)

// OnCapabilityRequest 处理能力请求的回调函数,从已装载能力中匹配最相近的能力
type OnCapabilityRequest func(keywords ...string) (*entity.Capability, error)

// OnHistoryRequest 处理历史对话请求的回调函数,获取会话的历史对话
// maxCount: 最多获取多少轮历史对话，用于限制模型的承载能力
type OnHistoryRequest func(maxCount int) ([]*DialogueMessage, error)

// SessionMgr 会话管理器接口
type SessionMgr interface {
	// RecordMessage 记录消息并判断是否需要结束会话
	RecordMessage(msg entity.Message) error
	// GetHistory 获取当前会话的所有对话内容
	GetHistory() []entity.Message
	// UpdateTokensFromModel 从模型Usage更新Token消耗
	UpdateTokensFromModel(usage TokenUsage)
}

// MemoryExtractor 记忆提取器接口
type MemoryExtractor interface {
	// Extract 从会话中提取记忆点并存储，返回是否成功
	Extract(session entity.Session) bool
}

// TokenUsage 模型返回的Token使用情况
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Persona 人设配置
type Persona struct {
	Name        string // 名字
	Gender      string // 性别
	Character   string // 性格
	UserContent string // 用户定义内容
}

// Assistant 智能助理
// 智能助理会默认使用AssistantPrompt作为智能体模板，而助理的名字、性别、性格、用户定义内容 会作为模板的占位符
// 这些占位符都会有默认值，如果用户没有设置，则使用默认值
// 名字: 小柔
// 性别: 女
// 性格: 温柔
type Assistant interface {
	SetName(name string)                         // SetName 设置助理的名字
	SetGender(gender string)                     // SetGender 设置助理的性别
	SetCharacter(character string)               // SetCharacter 设置助理的性格
	SetUserContent(content string)               // SetUserContent 设置用户对自智体的特定定义内容
	GetName() string                             // GetName 获取助理的名字
	GetGender() string                           // GetGender 获取助理的性别
	GetCharacter() string                        // GetCharacter 获取助理的性格
	GetSkillMgr() SkillManager                   // GetSkillMgr 获取技能管理器（负责执行）
	GetCapabilities() []entity.Capability        // GetCapabilities 获取所有可用的能力
	GetBrain() Brain                             // GetBrain 获取大脑负责思考
	GetMemory() Memory                           // GetMemory 获取长时记忆
	GetSessions() []interface{}                  // GetSessions 获取历史会话
	Ask(question string) (string, string, error) // Ask 问答，返回 (answer, sendTo, error)
	Summarize() error                            // 加载所有未形成记忆点的会话，并生成记忆点（由计划任务在深夜执行）
}
