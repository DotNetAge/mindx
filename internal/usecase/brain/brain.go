package brain

import (
	"context"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"time"
)

type processingContext struct {
	historyDialogue []*core.DialogueMessage
	refs            string
}

type BionicBrain struct {
	cfg              *config.GlobalConfig
	leftBrain        core.Thinking
	rightBrain       core.Thinking
	contextPreparer  *ContextPreparer
	toolCaller       *ToolCaller
	consciousnessMgr *ConsciousnessManager
	responseBuilder  *ResponseBuilder
	fallbackHandler  *FallbackHandler
	memory           core.Memory
	logger           logging.Logger
	toolsRequest     core.OnToolsRequest
	capRequest       core.OnCapabilityRequest
	historyRequest   core.OnHistoryRequest
	persona          *core.Persona
	tokenUsageRepo   core.TokenUsageRepository
	cronScheduler    cron.Scheduler
	brain            *core.Brain
}

func NewBrain(
	cfg *config.GlobalConfig,
	persona *core.Persona,
	memory core.Memory,
	skillMgr *skills.SkillMgr,
	toolsRequest core.OnToolsRequest,
	capRequest core.OnCapabilityRequest,
	historyRequest core.OnHistoryRequest,
	logger logging.Logger,
	tokenUsageRepo core.TokenUsageRepository,
	cronScheduler cron.Scheduler,
) (*core.Brain, error) {

	modelsMgr := config.GetModelsManager()
	brainModels := modelsMgr.GetBrainModels()

	leftBrainPrompt := buildLeftBrainPrompt(persona)

	leftModelName := brainModels.SubconsciousLeftModel
	if leftModelName == "" {
		leftModelName = brainModels.SubconsciousModel
	}
	if leftModelName == "" {
		leftModelName = modelsMgr.GetDefaultModel()
	}
	leftModel := modelsMgr.MustGetModel(leftModelName)

	rightModelName := brainModels.SubconsciousRightModel
	if rightModelName == "" {
		rightModelName = brainModels.SubconsciousModel
	}
	if rightModelName == "" {
		rightModelName = modelsMgr.GetDefaultModel()
	}
	rightModel := modelsMgr.MustGetModel(rightModelName)

	lbrain := NewThinking(leftModel, leftBrainPrompt, logger, tokenUsageRepo, &cfg.TokenBudget)
	rbrain := NewThinking(rightModel, "", logger, tokenUsageRepo, &cfg.TokenBudget)

	contextPreparer := NewContextPreparer(memory, historyRequest, logger)
	toolCaller := NewToolCaller(skillMgr, logger)
	consciousnessMgr := NewConsciousnessManager(cfg, persona, tokenUsageRepo, logger)
	responseBuilder := NewResponseBuilder()
	fallbackHandler := NewFallbackHandler(rbrain, toolCaller, responseBuilder, logger)

	impl := &BionicBrain{
		cfg:              cfg,
		leftBrain:        lbrain,
		rightBrain:       rbrain,
		contextPreparer:  contextPreparer,
		toolCaller:       toolCaller,
		consciousnessMgr: consciousnessMgr,
		responseBuilder:  responseBuilder,
		fallbackHandler:  fallbackHandler,
		memory:           memory,
		logger:           logger,
		toolsRequest:     toolsRequest,
		capRequest:       capRequest,
		historyRequest:   historyRequest,
		persona:          persona,
		tokenUsageRepo:   tokenUsageRepo,
		cronScheduler:    cronScheduler,
	}

	brain := &core.Brain{
		LeftBrain:     impl.leftBrain,
		RightBrain:    impl.rightBrain,
		Consciousness: impl.consciousnessMgr.Get(),
		GetMemory:     impl.getMemory,
		Post:          impl.post,
	}

	impl.brain = brain

	logger.Info(i18n.T("brain.init_success"),
		logging.String(i18n.T("brain.left_brain"), leftModel.Name),
		logging.String(i18n.T("brain.right_brain"), rightModel.Name),
		logging.String(i18n.T("brain.persona_name"), persona.Name),
		logging.String(i18n.T("brain.persona_gender"), persona.Gender),
		logging.String(i18n.T("brain.persona_character"), persona.Character))

	return brain, nil
}

func (b *BionicBrain) post(req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
	b.logger.Info(i18n.T("brain.start_process"), logging.String(i18n.T("brain.question"), req.Question))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	question := req.Question
	capabilityName, actualQuestion := b.parseCapabilityPrefix(question)
	if capabilityName != "" {
		b.logger.Info("检测到能力前缀，使用指定能力",
			logging.String("capability", capabilityName),
			logging.String("question", actualQuestion))
		return b.handleWithConsciousness(ctx, req, capabilityName, actualQuestion)
	}

	pctx, err := b.contextPreparer.Prepare(req.Question, b.leftBrain)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare context: %w", err)
	}

	eventChan := req.EventChan

	b.leftBrain.SetEventChan(eventChan)

	thinkResult, err := b.leftBrain.Think(ctx, req.Question, pctx.historyDialogue, pctx.refs, true)

	if err != nil {
		b.leftBrain.SetEventChan(nil)
		b.logger.Error(i18n.T("brain.left_think_failed"), logging.Err(err))
		return nil, fmt.Errorf("left brain think failed: %w", err)
	}

	b.logger.Info(i18n.T("brain.left_think_complete"),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String(i18n.T("brain.useless"), fmt.Sprintf("%v", thinkResult.Useless)),
		logging.String(i18n.T("brain.send_to"), thinkResult.SendTo),
		logging.String(i18n.T("brain.can_answer"), fmt.Sprintf("%v", thinkResult.CanAnswer)))

	if thinkResult.HasSchedule {
		b.logger.Info("[Cron] 检测到定时意图",
			logging.String("name", thinkResult.ScheduleName),
			logging.String("cron", thinkResult.ScheduleCron),
			logging.String("message", thinkResult.ScheduleMessage))

		if b.cronScheduler != nil {
			job := &cron.Job{
				Name:    thinkResult.ScheduleName,
				Cron:    thinkResult.ScheduleCron,
				Message: thinkResult.ScheduleMessage,
			}

			id, err := b.cronScheduler.Add(job)
			if err != nil {
				b.logger.Warn("[Cron] 创建任务失败", logging.Err(err))
				thinkResult.Answer = fmt.Sprintf("抱歉，创建定时任务失败：%v", err)
			} else {
				b.logger.Info("[Cron] 任务创建成功", logging.String("id", id))
				thinkResult.Answer = fmt.Sprintf("好的，我已经为你创建了定时任务「%s」，会在 %s 执行。",
					thinkResult.ScheduleName, thinkResult.ScheduleCron)
			}
		} else {
			thinkResult.Answer = "抱歉，当前系统不支持定时任务功能。"
		}

		b.leftBrain.SetEventChan(nil)
		return b.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
	}

	var leftBrainSearchedTools []*core.ToolSchema
	b.logger.Info("[右脑] 判断是否执行右脑处理",
		logging.Bool("useless", thinkResult.Useless),
		logging.String("intent", thinkResult.Intent),
		logging.Int("keywords_count", len(thinkResult.Keywords)))

	hasValidIntent := thinkResult.Intent != "" && len(thinkResult.Keywords) > 0 // 防止小模型在判断时出现抖动导致useless判断错误

	if !thinkResult.Useless || hasValidIntent {
		answer, tools, searchedTools := b.tryRightBrainProcess(ctx, req.Question, thinkResult, pctx.historyDialogue, req.SessionID, eventChan)
		if answer != "" {
			b.leftBrain.SetEventChan(nil)
			return b.responseBuilder.BuildToolCallResponse(answer, tools, thinkResult.SendTo), nil
		} else {
			b.logger.Info(i18n.T("brain.right_no_result"))
			leftBrainSearchedTools = searchedTools
		}
	}

	if !thinkResult.CanAnswer {
		resp, err := b.activateConsciousness(ctx, req.Question, thinkResult, pctx.refs, pctx.historyDialogue, leftBrainSearchedTools, req.SessionID, eventChan)
		b.leftBrain.SetEventChan(nil)
		return resp, err
	}

	b.leftBrain.SetEventChan(nil)
	return b.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
}

func (b *BionicBrain) tryRightBrainProcess(ctx context.Context, question string, thinkResult *core.ThinkingResult, historyDialogue []*core.DialogueMessage, sessionID string, eventChan chan<- ThinkingEvent) (string, []*core.ToolSchema, []*core.ToolSchema) {
	b.logger.Info("[右脑] 开始处理",
		logging.String("question", question),
		logging.String("intent", thinkResult.Intent))

	searchKeywords := []string{question}

	if thinkResult.Intent != "" {
		searchKeywords = append(searchKeywords, thinkResult.Intent)
	}

	if len(thinkResult.Keywords) > 0 {
		searchKeywords = append(searchKeywords, thinkResult.Keywords...)
	}

	b.logger.Info(i18n.T("brain.right_search_keywords"),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String("search_keywords", fmt.Sprintf("%v", searchKeywords)))

	tools, err := b.toolsRequest(searchKeywords...)
	if err != nil {
		b.logger.Warn("[右脑] 工具搜索失败", logging.Err(err))
		return "", nil, nil
	}

	b.logger.Info("[右脑] 工具搜索结果",
		logging.Int("tools_count", len(tools)),
		logging.Err(err))

	if len(tools) == 0 {
		b.logger.Warn("[右脑] 没有找到匹配的工具")
		return "", nil, nil
	}

	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	if eventChan != nil {
		eventChan <- ThinkingEvent{
			Type:     ThinkingEventProgress,
			Content:  fmt.Sprintf(i18n.T("brain.found_tools"), len(tools)),
			Progress: 50,
			Metadata: map[string]any{"tools": toolNames},
		}
	}

	b.logger.Info(i18n.T("brain.right_found_skill"),
		logging.String(i18n.T("brain.question"), question),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String(i18n.T("brain.matched_tools"), fmt.Sprintf("%v", toolNames)),
		logging.Int(i18n.T("brain.tools_count"), len(tools)))

	b.rightBrain.SetEventChan(eventChan)
	answer, err := b.toolCaller.ExecuteToolCall(ctx, b.rightBrain, question, historyDialogue, tools)
	b.rightBrain.SetEventChan(nil)

	if err != nil {
		b.logger.Warn(i18n.T("brain.right_tool_call_failed"), logging.Err(err))
		return "", nil, tools
	}

	return answer, tools, tools
}

func (b *BionicBrain) activateConsciousness(ctx context.Context, question string, thinkResult *core.ThinkingResult, refs string, historyDialogue []*core.DialogueMessage, leftBrainSearchedTools []*core.ToolSchema, sessionID string, eventChan chan<- ThinkingEvent) (*core.ThinkingResponse, error) {
	b.logger.Info(i18n.T("brain.left_cannot_answer"))

	if eventChan != nil {
		eventChan <- ThinkingEvent{
			Type:     ThinkingEventProgress,
			Content:  i18n.T("brain.activating_consciousness"),
			Progress: 60,
		}
	}

	capability, err := b.capRequest(thinkResult.Intent)
	if err == nil && capability != nil {
		b.logger.Info(i18n.T("brain.found_capability"), logging.String("capability", capability.Name))
		return b.activateConsciousnessWithCapability(ctx, question, thinkResult, refs, historyDialogue, leftBrainSearchedTools, sessionID, eventChan, capability)
	}

	b.logger.Info(i18n.T("brain.no_capability_found"), logging.String("intent", thinkResult.Intent))
	return b.activateConsciousnessDualBrain(ctx, question, refs, historyDialogue, eventChan)
}

func (b *BionicBrain) activateConsciousnessWithCapability(ctx context.Context, question string, thinkResult *core.ThinkingResult, refs string, historyDialogue []*core.DialogueMessage, leftBrainSearchedTools []*core.ToolSchema, sessionID string, eventChan chan<- ThinkingEvent, capability *entity.Capability) (*core.ThinkingResponse, error) {
	if b.consciousnessMgr.IsNil() {
		b.consciousnessMgr.Create(capability)
	}

	if b.consciousnessMgr.IsNil() {
		b.logger.Warn(i18n.T("brain.consciousness_create_failed"))
		return b.fallbackHandler.Handle(ctx, question, thinkResult, historyDialogue, leftBrainSearchedTools)
	}

	var tools []*core.ToolSchema
	if len(capability.Tools) > 0 {
		tools, err := b.toolsRequest(capability.Tools...)
		if err != nil {
			b.logger.Warn(i18n.T("brain.get_consciousness_tool_failed"), logging.Err(err))
			tools = make([]*core.ToolSchema, 0)
		} else {
			toolNames := make([]string, 0, len(tools))
			for _, tool := range tools {
				toolNames = append(toolNames, tool.Name)
			}
			b.logger.Info(i18n.T("brain.consciousness_found_tools"),
				logging.Int(i18n.T("brain.tools_count"), len(tools)),
				logging.String(i18n.T("brain.matched_tools"), fmt.Sprintf("%v", toolNames)))
		}
	}

	if len(leftBrainSearchedTools) > 0 {
		toolMap := make(map[string]*core.ToolSchema)
		for _, tool := range tools {
			toolMap[tool.Name] = tool
		}
		for _, tool := range leftBrainSearchedTools {
			if _, exists := toolMap[tool.Name]; !exists {
				toolMap[tool.Name] = tool
			}
		}
		tools = make([]*core.ToolSchema, 0, len(toolMap))
		for _, tool := range toolMap {
			tools = append(tools, tool)
		}
		b.logger.Info(i18n.T("brain.consciousness_merge_left"),
			logging.Int(i18n.T("brain.left_brain_tools"), len(leftBrainSearchedTools)),
			logging.Int(i18n.T("brain.total_tools"), len(tools)))
	}

	if len(tools) > 0 {
		resp, err := b.consciousnessWithTools(ctx, question, thinkResult, historyDialogue, tools, eventChan, capability.SystemPrompt)
		if err != nil {
			b.logger.Warn(i18n.T("brain.consciousness_tool_call_failed"), logging.Err(err))
			return b.fallbackHandler.Handle(ctx, question, thinkResult, historyDialogue, leftBrainSearchedTools)
		}
		return resp, nil
	}

	b.consciousnessMgr.Get().SetEventChan(eventChan)
	result, err := b.consciousnessMgr.Think(ctx, question, historyDialogue, refs)
	b.consciousnessMgr.Get().SetEventChan(nil)

	if err != nil {
		b.logger.Warn(i18n.T("brain.consciousness_think_failed"), logging.Err(err))
		return b.fallbackHandler.Handle(ctx, question, thinkResult, historyDialogue, leftBrainSearchedTools)
	}

	return b.responseBuilder.BuildToolCallResponse(result.Answer, nil, thinkResult.SendTo), nil
}

func (b *BionicBrain) activateConsciousnessDualBrain(ctx context.Context, question string, refs string, historyDialogue []*core.DialogueMessage, eventChan chan<- ThinkingEvent) (*core.ThinkingResponse, error) {
	b.logger.Info(i18n.T("brain.activating_consciousness_dual_brain"))

	if !b.consciousnessMgr.HasDualBrain() {
		err := b.consciousnessMgr.CreateDualBrain()
		if err != nil {
			b.logger.Warn(i18n.T("brain.consciousness_dual_brain_failed"), logging.Err(err))
			return nil, fmt.Errorf("failed to create consciousness dual brain: %w", err)
		}
	}

	leftBrain := b.consciousnessMgr.GetLeftBrain()
	rightBrain := b.consciousnessMgr.GetRightBrain()

	leftBrain.SetEventChan(eventChan)

	thinkResult, err := leftBrain.Think(ctx, question, historyDialogue, refs, true)
	if err != nil {
		leftBrain.SetEventChan(nil)
		b.logger.Error(i18n.T("brain.consciousness_left_think_failed"), logging.Err(err))
		return nil, fmt.Errorf("consciousness left brain think failed: %w", err)
	}

	b.logger.Info(i18n.T("brain.consciousness_left_think_complete"),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String(i18n.T("brain.useless"), fmt.Sprintf("%v", thinkResult.Useless)),
		logging.String(i18n.T("brain.send_to"), thinkResult.SendTo))

	if thinkResult.Useless || thinkResult.Answer != "" {
		leftBrain.SetEventChan(nil)
		return b.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
	}

	hasValidIntent := thinkResult.Intent != "" && len(thinkResult.Keywords) > 0
	if !hasValidIntent {
		leftBrain.SetEventChan(nil)
		return b.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
	}

	b.logger.Info("[主意识右脑] 判断是否执行右脑处理",
		logging.String("intent", thinkResult.Intent),
		logging.Int("keywords_count", len(thinkResult.Keywords)))

	answer, tools, _ := b.tryConsciousnessRightBrainProcess(ctx, question, thinkResult, historyDialogue, eventChan, rightBrain)
	if answer != "" {
		leftBrain.SetEventChan(nil)
		return b.responseBuilder.BuildToolCallResponse(answer, tools, thinkResult.SendTo), nil
	}

	leftBrain.SetEventChan(nil)
	return b.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
}

func (b *BionicBrain) tryConsciousnessRightBrainProcess(ctx context.Context, question string, thinkResult *core.ThinkingResult, historyDialogue []*core.DialogueMessage, eventChan chan<- ThinkingEvent, rightBrain core.Thinking) (string, []*core.ToolSchema, []*core.ToolSchema) {
	b.logger.Info("[主意识右脑] 开始处理",
		logging.String("question", question),
		logging.String("intent", thinkResult.Intent))

	searchKeywords := []string{question}
	if thinkResult.Intent != "" {
		searchKeywords = append(searchKeywords, thinkResult.Intent)
	}
	if len(thinkResult.Keywords) > 0 {
		searchKeywords = append(searchKeywords, thinkResult.Keywords...)
	}

	b.logger.Info(i18n.T("brain.right_search_keywords"),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String("search_keywords", fmt.Sprintf("%v", searchKeywords)))

	tools, err := b.toolsRequest(searchKeywords...)
	if err != nil {
		b.logger.Warn("[主意识右脑] 工具搜索失败", logging.Err(err))
		return "", nil, nil
	}

	b.logger.Info("[主意识右脑] 工具搜索结果",
		logging.Int("tools_count", len(tools)),
		logging.Err(err))

	if len(tools) == 0 {
		b.logger.Warn("[主意识右脑] 没有找到匹配的工具")
		return "", nil, nil
	}

	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	if eventChan != nil {
		eventChan <- ThinkingEvent{
			Type:     ThinkingEventProgress,
			Content:  fmt.Sprintf(i18n.T("brain.found_tools"), len(tools)),
			Progress: 70,
			Metadata: map[string]any{"tools": toolNames},
		}
	}

	b.logger.Info(i18n.T("brain.right_found_skill"),
		logging.String(i18n.T("brain.question"), question),
		logging.String(i18n.T("brain.intent"), thinkResult.Intent),
		logging.String(i18n.T("brain.keywords"), fmt.Sprintf("%v", thinkResult.Keywords)),
		logging.String(i18n.T("brain.matched_tools"), fmt.Sprintf("%v", toolNames)),
		logging.Int(i18n.T("brain.tools_count"), len(tools)))

	rightBrain.SetEventChan(eventChan)
	answer, err := b.toolCaller.ExecuteToolCall(ctx, rightBrain, question, historyDialogue, tools)
	rightBrain.SetEventChan(nil)

	if err != nil {
		b.logger.Warn(i18n.T("brain.right_tool_call_failed"), logging.Err(err))
		return "", nil, tools
	}

	return answer, tools, tools
}

func (b *BionicBrain) consciousnessWithTools(ctx context.Context, question string, thinkResult *core.ThinkingResult, historyDialogue []*core.DialogueMessage, tools []*core.ToolSchema, eventChan chan<- ThinkingEvent, customSystemPrompt ...string) (*core.ThinkingResponse, error) {
	b.logger.Info(i18n.T("brain.consciousness_use_tools"), logging.Int(i18n.T("brain.tools_count"), len(tools)))

	b.consciousnessMgr.Get().SetEventChan(eventChan)
	answer, err := b.toolCaller.ExecuteToolCall(ctx, b.consciousnessMgr.Get(), question, historyDialogue, tools, customSystemPrompt...)
	b.consciousnessMgr.Get().SetEventChan(nil)

	if err != nil {
		b.logger.Error(i18n.T("brain.consciousness_tool_failed"), logging.Err(err))
		return nil, fmt.Errorf("consciousness tool call failed: %w", err)
	}

	return b.responseBuilder.BuildToolCallResponse(answer, tools, thinkResult.SendTo), nil
}

func (b *BionicBrain) getMemory() (core.Memory, error) {
	return b.memory, nil
}

func (b *BionicBrain) sendThinkingEvent(sessionID string, event *ThinkingEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.logger.Info("[思考流] 准备发送思考事件",
		logging.String("session_id", sessionID),
		logging.String("type", string(event.Type)),
		logging.Bool("brain_nil", b.brain == nil),
		logging.Bool("callback_nil", b.brain == nil || b.brain.OnThinkingEvent == nil))

	if b.brain != nil && b.brain.OnThinkingEvent != nil && sessionID != "" {
		eventData := map[string]any{
			"type":      string(event.Type),
			"content":   event.Content,
			"progress":  event.Progress,
			"timestamp": event.Timestamp.Unix(),
			"metadata":  event.Metadata,
		}
		b.logger.Info("[思考流] 发送思考事件到 Channel",
			logging.String("session_id", sessionID),
			logging.String("type", string(event.Type)),
			logging.String("content", event.Content),
			logging.Float64("progress", event.Progress))
		b.brain.OnThinkingEvent(sessionID, eventData)
	} else {
		if b.brain == nil || b.brain.OnThinkingEvent == nil {
			b.logger.Warn("[思考流] OnThinkingEvent 回调未设置")
		}
		if sessionID == "" {
			b.logger.Warn("[思考流] sessionID 为空")
		}
	}
}

func buildLeftBrainPrompt(persona *core.Persona) string {
	ctx := &core.PromptContext{
		UsePersona:       false,
		UseThinking:      true,
		IsLocalModel:     true,
		PersonaName:      persona.Name,
		PersonaGender:    persona.Gender,
		PersonaCharacter: persona.Character,
		PersonaContent:   persona.UserContent,
	}
	return core.BuildLeftBrainPrompt(ctx)
}

func (b *BionicBrain) parseCapabilityPrefix(question string) (capabilityName, actualQuestion string) {
	if len(question) == 0 || question[0] != '/' {
		return "", question
	}

	spaceIndex := -1
	for i, c := range question {
		if c == ' ' {
			spaceIndex = i
			break
		}
	}

	if spaceIndex == -1 {
		capabilityName = question[1:]
		actualQuestion = ""
	} else {
		capabilityName = question[1:spaceIndex]
		actualQuestion = question[spaceIndex+1:]
	}

	return capabilityName, actualQuestion
}

// TODO: 需要在 SystemPrompt中补充的对技能的使用引导
func (b *BionicBrain) handleWithConsciousness(ctx context.Context, req *core.ThinkingRequest, capabilityName, actualQuestion string) (*core.ThinkingResponse, error) {
	pctx, err := b.contextPreparer.Prepare(actualQuestion, b.leftBrain)
	if err != nil {
		return nil, fmt.Errorf("准备上下文失败: %w", err)
	}

	eventChan := req.EventChan

	capability, err := b.capRequest(capabilityName)
	if err != nil {
		b.logger.Warn("获取能力失败", logging.String("capability", capabilityName), logging.Err(err))
		return b.responseBuilder.BuildToolCallResponse(fmt.Sprintf("抱歉，获取能力 '%s' 时出错。", capabilityName), nil, ""), nil
	}

	if capability == nil {
		b.logger.Warn("指定的能力不存在", logging.String("capability", capabilityName))
		return b.responseBuilder.BuildToolCallResponse(fmt.Sprintf("抱歉，能力 '%s' 不存在。", capabilityName), nil, ""), nil
	}

	if !capability.Enabled {
		b.logger.Warn("指定的能力已禁用", logging.String("capability", capabilityName))
		return b.responseBuilder.BuildToolCallResponse(fmt.Sprintf("抱歉，能力 '%s' 已禁用。", capabilityName), nil, ""), nil
	}

	b.logger.Info("使用指定能力", logging.String("capability", capability.Name))

	if b.consciousnessMgr.IsNil() {
		b.consciousnessMgr.Create(capability)
	}

	var tools []*core.ToolSchema
	if len(capability.Tools) > 0 {
		tools, err = b.toolsRequest(capability.Tools...)
		if err != nil {
			b.logger.Warn(i18n.T("brain.get_consciousness_tool_failed"), logging.Err(err))
			tools = make([]*core.ToolSchema, 0)
		} else {
			toolNames := make([]string, 0, len(tools))
			for _, tool := range tools {
				toolNames = append(toolNames, tool.Name)
			}
			b.logger.Info(i18n.T("brain.consciousness_found_tools"),
				logging.Int(i18n.T("brain.tools_count"), len(tools)),
				logging.String(i18n.T("brain.matched_tools"), fmt.Sprintf("%v", toolNames)))
		}
	}

	if len(tools) > 0 {
		resp, err := b.consciousnessWithTools(ctx, actualQuestion, nil, pctx.historyDialogue, tools, eventChan, capability.SystemPrompt)
		return resp, err
	}

	b.consciousnessMgr.Get().SetEventChan(eventChan)
	result, err := b.consciousnessMgr.Think(ctx, actualQuestion, pctx.historyDialogue, pctx.refs)
	b.consciousnessMgr.Get().SetEventChan(nil)

	if err != nil {
		b.logger.Warn(i18n.T("brain.consciousness_think_failed"), logging.Err(err))
		return b.fallbackHandler.Handle(ctx, actualQuestion, nil, pctx.historyDialogue, nil)
	}

	return b.responseBuilder.BuildToolCallResponse(result.Answer, nil, ""), nil
}
