package brain

import (
	"context"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/brain/processors"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// NewBrainWithPipeline 创建基于 Pipeline 的新 Brain
//
// 重要：新架构完全废弃了 V1 的左右脑概念
// - 只创建一个 Thinking 实例
// - 降级策略在 IntentProcessor 内部实现（本地模型失败 → 云端模型）
// - 不再有 LeftBrain/RightBrain 的概念
//
// TODO: TECH DEBT [TD-001] - 当前 SkillMatchProcessor 使用错误的 Skill 实现
// 参考：docs/v2/TECH-DEBT.md#TD-001
//
// TODO: TECH DEBT [TD-002] - SkillMatchProcessor 不加载 SOP，不组装 Tools
// 参考：docs/v2/TECH-DEBT.md#TD-002
func NewBrainWithPipeline(deps BrainDeps) (*core.Brain, error) {
	logger := deps.Logger.Named("brain_pipeline")
	cfg := deps.Cfg
	persona := deps.Persona
	tokenUsageRepo := deps.TokenUsageRepo

	// 获取模型配置
	modelsMgr := config.GetModelsManager()

	// 使用默认模型（通常是云端模型）
	modelName := modelsMgr.GetDefaultModel()
	model := modelsMgr.MustGetModel(modelName)

	// 构建 System Prompt
	// 重要：根据模型体量选择不同的 Prompt 优化策略
	// - 本地模型（量化模型）：使用简化的 Prompt，减少 Token 消耗
	// - 云端模型（大模型）：使用完整的 Prompt，包含更多上下文
	promptCtx := &core.PromptContext{
		UsePersona:       true,
		UseThinking:      false,
		IsLocalModel:     false, // 默认使用云端模型
		PersonaName:      persona.Name,
		PersonaGender:    persona.Gender,
		PersonaCharacter: persona.Character,
		PersonaContent:   persona.UserContent,
	}

	// 根据模型类型选择 Prompt 构建方式
	var systemPrompt string
	if model.BaseURL == "http://localhost:11434" || model.BaseURL == "http://127.0.0.1:11434" {
		// 本地 Ollama 模型，使用简化 Prompt
		promptCtx.IsLocalModel = true
		systemPrompt = core.BuildLeftBrainPrompt(promptCtx)
		logger.Info("using local model prompt", logging.String("model", modelName))
	} else {
		// 云端模型，使用完整 Prompt
		systemPrompt = core.BuildCloudModelPrompt(promptCtx)
		logger.Info("using cloud model prompt", logging.String("model", modelName))
	}

	// 创建唯一的 Thinking 实例
	thinking := NewThinking(model, systemPrompt, logger.Named("thinking"), tokenUsageRepo, &cfg.TokenBudget)

	// 创建处理器管线
	pipeline := NewPipeline(
		// 1. 意图识别
		// IntentProcessor 内部实现降级：本地模型失败 → 云端模型
		// TODO: TECH DEBT - 当前传入相同的 thinking 实例，应该在 IntentProcessor 内部实现真正的降级
		processors.NewIntentProcessor(thinking, thinking),

		// 2. 记忆检索（关键词匹配）
		// TODO: TECH DEBT [TD-007] - 应该使用向量相似度
		processors.NewMemoryRetrievalProcessor(deps.Memory, 5),

		// 3. 技能匹配（关键词匹配）
		// TODO: TECH DEBT [TD-002, TD-003] - 只是占位符，不加载 SOP，不组装 Tools
		processors.NewSkillMatchProcessor(deps.SkillMgr, 3),

		// 4. 工具执行
		// TODO: TECH DEBT [TD-002] - 因为 SkillMatchProcessor 不组装 Tools，此处理器被跳过
		processors.NewToolExecutionProcessor(thinking, deps.SkillMgr),

		// 5. 响应生成
		processors.NewResponseProcessor(thinking),
	)

	// 创建 Brain 适配器
	brainAdapter := &PipelineBrainAdapter{
		pipeline:       pipeline,
		thinking:       thinking,
		logger:         logger,
		tokenUsageRepo: tokenUsageRepo,
	}

	logger.Info(i18n.T("brain.init_success"),
		logging.String("model", modelName),
		logging.String(i18n.T("brain.persona_name"), persona.Name),
		logging.String(i18n.T("brain.persona_gender"), persona.Gender),
		logging.String(i18n.T("brain.persona_character"), persona.Character))

	// 返回 Brain
	// 注意：为了兼容现有接口，仍然填充 LeftBrain/RightBrain 字段
	// 但实际上它们指向同一个 Thinking 实例
	// TODO: Phase 2 应该移除 LeftBrain/RightBrain 字段
	return &core.Brain{
		LeftBrain:  thinking, // 兼容性：指向同一个实例
		RightBrain: thinking, // 兼容性：指向同一个实例
		Post:       brainAdapter.Post,
		OnThinkingEvent: func(sessionID string, event map[string]any) {
			// TODO: 实现事件推送
		},
	}, nil
}

// PipelineBrainAdapter 将 Pipeline 适配为旧的 Brain.Post 接口
type PipelineBrainAdapter struct {
	pipeline       *Pipeline
	thinking       core.Thinking
	logger         logging.Logger
	tokenUsageRepo core.TokenUsageRepository
}

// Post 实现 Brain.Post 接口
func (a *PipelineBrainAdapter) Post(req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
	a.logger.Info("pipeline processing started",
		logging.String("question", req.Question),
		logging.String("session_id", req.SessionID))

	// 创建 ThinkContext
	thinkCtx := entity.NewThinkContext(req.Question, req.SessionID)

	// 执行 Pipeline
	ctx := context.Background()
	if err := a.pipeline.Execute(ctx, thinkCtx); err != nil {
		a.logger.Error("pipeline execution failed", logging.Err(err))
		return nil, err
	}

	a.logger.Info("pipeline processing completed",
		logging.String("session_id", req.SessionID),
		logging.Duration("duration", thinkCtx.Duration()),
		logging.Int("response_length", len(thinkCtx.Response)))

	// 转换为 ThinkingResponse
	response := &core.ThinkingResponse{
		Answer: thinkCtx.Response,
		SendTo: thinkCtx.SendTo,
		Tools:  nil, // TODO: 从 thinkCtx.MatchedSkills 转换
	}

	// TODO: 处理定时任务（如果有）
	// 当前 MVP 暂不支持

	return response, nil
}
