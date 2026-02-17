package bootstrap

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/brain"
	"mindx/internal/usecase/capability"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"time"
)

// Assistant 智能助理实现
// Assistant 是最重要的宿主，负责协调所有模块的交互
type Assistant struct {
	name        string
	gender      string
	character   string
	userContent string

	persona        *core.Persona // 保存 persona 引用，方便更新
	brain          *core.Brain
	cfg            *config.GlobalConfig
	sessionMgr     core.SessionMgr // 会话管理器
	capMgr         *capability.CapabilityManager
	skillMgr       *skills.SkillMgr
	logger         logging.Logger
	tokenUsageRepo core.TokenUsageRepository
}

// NewAssistant 创建智能助理
func NewAssistant(
	cfg *config.GlobalConfig,
	capMgr *capability.CapabilityManager,
	sessionMgr core.SessionMgr,
	skillMgr *skills.SkillMgr,
	mem *memory.Memory,
	logger logging.Logger,
	tokenUsageRepo core.TokenUsageRepository,
) *Assistant {

	// 构建人设
	persona := &core.Persona{
		Name:        "小柔", // 默认名字
		Gender:      "女",  // 默认性别
		Character:   "温柔", // 默认性格
		UserContent: "",   // 默认用户定义内容
	}

	// 创建历史对话回调（使用新的 SessionMgr）
	historyRequest := func(maxCount int) ([]*core.DialogueMessage, error) {
		if sessionMgr == nil {
			return []*core.DialogueMessage{}, nil
		}

		// 获取当前会话的所有消息（已由 SessionMgr 去重）
		messages := sessionMgr.GetHistory()

		// 转换消息格式
		dialogueMessages := make([]*core.DialogueMessage, len(messages))
		for i, msg := range messages {
			dialogueMessages[i] = &core.DialogueMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		// 限制数量
		if maxCount > 0 && len(dialogueMessages) > maxCount {
			dialogueMessages = dialogueMessages[len(dialogueMessages)-maxCount:]
		}

		return dialogueMessages, nil
	}

	// 创建工具请求回调
	toolsRequest := func(keywords ...string) ([]*core.ToolSchema, error) {
		if skillMgr == nil {
			return []*core.ToolSchema{}, nil
		}

		skillsList, err := skillMgr.SearchSkills(keywords...)
		if err != nil {
			logger.Warn(i18n.T("infra.search_skill_failed"), logging.Err(err))
			return []*core.ToolSchema{}, nil
		}

		tools := make([]*core.ToolSchema, 0, len(skillsList))
		for _, skill := range skillsList {
			name := skill.GetName()

			// 获取完整的技能信息，包括参数定义
			info, exists := skillMgr.GetSkillInfo(name)
			if !exists {
				logger.Warn(i18n.T("infra.skill_info_not_exist"), logging.String(i18n.T("infra.skill"), name))
				tools = append(tools, &core.ToolSchema{
					Name:        name,
					Description: "",
					Params:      map[string]interface{}{},
				})
				continue
			}

			// 构建参数 Schema (JSON Schema 格式)
			params := map[string]interface{}{}
			if info.Def != nil && len(info.Def.Parameters) > 0 {
				properties := map[string]interface{}{}
				required := []string{}
				for paramName, paramDef := range info.Def.Parameters {
					properties[paramName] = map[string]interface{}{
						"type":        paramDef.Type,
						"description": paramDef.Description,
					}
					if paramDef.Required {
						required = append(required, paramName)
					}
				}
				params = map[string]interface{}{
					"type":       "object",
					"properties": properties,
					"required":   required,
				}
			}

			tools = append(tools, &core.ToolSchema{
				Name:         info.Def.Name,
				Description:  info.Def.Description,
				Params:       params,
				OutputFormat: info.Def.OutputFormat,
				Guidance:     info.Def.Guidance,
			})
		}

		return tools, nil
	}

	// 创建能力请求回调
	capRequest := func(keywords ...string) (*entity.Capability, error) {
		if capMgr == nil {
			return nil, nil
		}

		query := strings.Join(keywords, " ")
		caps, err := capMgr.RouteCapability(query, 1)
		if err != nil || len(caps) == 0 {
			return nil, nil
		}
		return caps[0], nil
	}

	// 创建大脑（传入人设）
	brain, err := brain.NewBrain(
		cfg,
		persona,
		mem,      // memory
		skillMgr, // skillMgr
		toolsRequest,
		capRequest,
		historyRequest,
		logger,
		tokenUsageRepo,
	)
	if err != nil {
		logger.Error(i18n.T("infra.create_brain_failed"), logging.Err(err))
		return nil
	}

	return &Assistant{
		name:           persona.Name,
		gender:         persona.Gender,
		character:      persona.Character,
		userContent:    persona.UserContent,
		persona:        persona, // 保存 persona 引用
		brain:          brain,
		cfg:            cfg,
		sessionMgr:     sessionMgr, // 会话管理器
		capMgr:         capMgr,
		skillMgr:       skillMgr,
		logger:         logger,
		tokenUsageRepo: tokenUsageRepo,
	}
}

// SetName 设置助理的名字
func (a *Assistant) SetName(name string) {
	a.name = name
	a.persona.Name = name
}

// SetGender 设置助理的性别
func (a *Assistant) SetGender(gender string) {
	a.gender = gender
	a.persona.Gender = gender
}

// SetCharacter 设置助理的性格
func (a *Assistant) SetCharacter(character string) {
	a.character = character
	a.persona.Character = character
}

// SetUserContent 设置用户对自智体的特定定义内容
func (a *Assistant) SetUserContent(content string) {
	a.userContent = content
	a.persona.UserContent = content
}

// GetName 获取助理的名字
func (a *Assistant) GetName() string {
	return a.name
}

// GetGender 获取助理的性别
func (a *Assistant) GetGender() string {
	return a.gender
}

// GetCharacter 获取助理的性格
func (a *Assistant) GetCharacter() string {
	return a.character
}

// GetSkillMgr 获取技能管理器（负责执行）
func (a *Assistant) GetSkillMgr() core.SkillManager {
	if a.skillMgr == nil {
		return nil
	}
	return a.skillMgr
}

// SetSkillMgr 设置技能管理器
func (a *Assistant) SetSkillMgr(skillMgr *skills.SkillMgr) {
	a.skillMgr = skillMgr
}

// GetCapabilities 获取所有可用的能力
func (a *Assistant) GetCapabilities() []interface{} {
	if a.capMgr == nil {
		return []interface{}{}
	}

	caps := a.capMgr.ListCapabilities()
	result := make([]interface{}, len(caps))
	for i, cap := range caps {
		result[i] = cap
	}

	return result
}

// GetBrain 获取大脑负责思考
func (a *Assistant) GetBrain() core.Brain {
	return *a.brain
}

// SetOnThinkingEvent 设置思考流事件回调
func (a *Assistant) SetOnThinkingEvent(callback func(sessionID string, event map[string]any)) {
	if a.brain != nil {
		a.brain.OnThinkingEvent = callback
	}
}

// GetMemory 获取长时记忆
func (a *Assistant) GetMemory() core.Memory {
	if a.brain.GetMemory == nil {
		return nil
	}
	mem, _ := a.brain.GetMemory()
	return mem
}

// GetSessions 获取历史会话
func (a *Assistant) GetSessions() []interface{} {
	if a.sessionMgr == nil {
		return []interface{}{}
	}

	sess, ok := a.sessionMgr.(*session.SessionMgr).GetCurrentSession()
	if !ok || sess == nil {
		return []interface{}{}
	}

	// 从存储中加载所有会话
	allSessions, err := a.sessionMgr.(*session.SessionMgr).GetAllSessions()
	if err != nil {
		a.logger.Warn(i18n.T("infra.load_sessions_failed"), logging.Err(err))
		return []interface{}{*sess}
	}

	sessions := make([]interface{}, len(allSessions))
	for i, s := range allSessions {
		sessions[i] = s
	}
	return sessions
}

// Ask 问答
// Assistant 作为核心宿主，将问题转发给 Brain 处理
// Brain 处理后的信息回调也是通过 Assistant 转发
// 返回值: (answer, sendTo, error)
// - answer: 回答内容
// - sendTo: 目标 Channel（用于消息转发），为空表示不需要转发
func (a *Assistant) Ask(question string, sessionID string, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
	a.logger.Info(i18n.T("infra.receive_question"), logging.String(i18n.T("infra.question"), question))

	if a.sessionMgr != nil {
		_ = a.sessionMgr.RecordMessage(entity.Message{
			Role:    "user",
			Content: question,
			Time:    time.Now(),
		})
	}

	req := &core.ThinkingRequest{
		Question:  question,
		Timeout:   30,
		SessionID: sessionID,
		EventChan: eventChan,
	}

	resp, err := a.brain.Post(req)
	if err != nil {
		a.logger.Error(i18n.T("infra.brain_process_failed"), logging.Err(err))
		return "", "", fmt.Errorf("大脑处理失败: %w", err)
	}

	// 记录助手回复消息到新的 SessionMgr
	if a.sessionMgr != nil {
		_ = a.sessionMgr.RecordMessage(entity.Message{
			Role:    "assistant",
			Content: resp.Answer,
			Time:    time.Now(),
		})
	}

	// 注意：技能执行已在 Brain 内部通过 toolCaller.ExecuteToolCall 完成
	// resp.Tools 仅用于记录已执行的工具信息，不需要再次执行

	// 如果有 SendTo 字段，记录日志
	if resp.SendTo != "" {
		a.logger.Info(i18n.T("infra.forward_intent"),
			logging.String(i18n.T("infra.answer"), resp.Answer),
			logging.String(i18n.T("infra.send_to"), resp.SendTo),
		)
	}

	return resp.Answer, resp.SendTo, nil
}

// Summarize 记忆点重整
// Assistant 从 SessionManager 获取所有会话，对未被记忆的会话进行记忆提取
func (a *Assistant) Summarize() error {
	a.logger.Info(i18n.T("infra.start_memory_reorg"))

	if a.sessionMgr == nil {
		a.logger.Warn(i18n.T("infra.sessionmgr_not_init"))
		return nil
	}

	// 获取未被记忆的会话
	unmemorizedSessions := a.sessionMgr.(*session.SessionMgr).CleanupUnmemorizedSessions()
	a.logger.Info(i18n.T("infra.unmemorized_count"), logging.Int(i18n.T("infra.count"), len(unmemorizedSessions)))

	// 创建记忆提取器
	mem := a.GetMemory()
	if mem == nil {
		a.logger.Warn(i18n.T("infra.memory_not_init"))
		return nil
	}

	leftBrain := a.GetBrain().LeftBrain
	if leftBrain == nil {
		a.logger.Warn(i18n.T("infra.leftbrain_not_init"))
		return nil
	}

	extractor := memory.NewLLMExtractor(leftBrain, mem)

	// 遍历所有未记忆的会话
	for _, sess := range unmemorizedSessions {
		a.logger.Info(i18n.T("infra.process_unmemorized"),
			logging.String(i18n.T("infra.session_id"), sess.ID),
			logging.Int(i18n.T("infra.messages"), len(sess.Messages)),
		)

		// 跳过空会话
		if len(sess.Messages) == 0 {
			continue
		}

		// 提取记忆
		success := extractor.Extract(sess)
		if success {
			a.logger.Info(i18n.T("infra.memory_extract_success"), logging.String(i18n.T("infra.session_id"), sess.ID))
			// 记录 CheckPoint
			a.sessionMgr.(*session.SessionMgr).RecordCheckPoint(sess.ID)
		} else {
			a.logger.Error(i18n.T("infra.memory_extract_failed"), logging.String(i18n.T("infra.session_id"), sess.ID))
		}
	}

	a.logger.Info(i18n.T("infra.memory_reorg_complete"))
	return nil
}

// getSystemPrompt 生成真正的智能体定义
// 已废弃：systemPrompt 的格式化由 Brain 负责处理
func (a *Assistant) getSystemPrompt() string {
	prompt := fmt.Sprintf(core.AssistantPrompt, a.name, a.gender, a.character, a.userContent)
	return prompt
}
