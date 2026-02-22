package brain

import (
	"context"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"fmt"
)

type ConsciousnessManager struct {
	cfg            *config.GlobalConfig
	persona        *core.Persona
	tokenUsageRepo core.TokenUsageRepository
	logger         logging.Logger
	consciousness  core.Thinking
	leftBrain      core.Thinking
	rightBrain     core.Thinking
}

func NewConsciousnessManager(
	cfg *config.GlobalConfig,
	persona *core.Persona,
	tokenUsageRepo core.TokenUsageRepository,
	logger logging.Logger,
) *ConsciousnessManager {
	return &ConsciousnessManager{
		cfg:            cfg,
		persona:        persona,
		tokenUsageRepo: tokenUsageRepo,
		logger:         logger,
		consciousness:  nil,
		leftBrain:      nil,
		rightBrain:     nil,
	}
}

func (cm *ConsciousnessManager) Create(capability *entity.Capability) {
	cm.logger.Info(i18n.T("brain.create_consciousness"),
		logging.String(i18n.T("brain.capability"), capability.Name),
		logging.String(i18n.T("brain.model"), capability.Model))

	personaInfo := fmt.Sprintf("\n\n## 人设信息\n名字: %s\n性别: %s\n性格: %s\n%s",
		cm.persona.Name, cm.persona.Gender, cm.persona.Character, cm.persona.UserContent)
	systemPrompt := capability.SystemPrompt + personaInfo

	modelsMgr := config.GetModelsManager()
	modelConfig, err := modelsMgr.GetModel(capability.Model)
	if err != nil {
		modelConfig = &config.ModelConfig{
			Name:    capability.Model,
			BaseURL: "http://localhost:11434/v1",
		}
	}

	cm.consciousness = NewThinking(modelConfig, systemPrompt, cm.logger, cm.tokenUsageRepo, &cm.cfg.TokenBudget)
	cm.logger.Info(i18n.T("brain.consciousness_created"))
}

func (cm *ConsciousnessManager) CreateDualBrain() error {
	cm.logger.Info(i18n.T("brain.create_consciousness_dual_brain"))

	modelsMgr := config.GetModelsManager()
	brainModels := modelsMgr.GetBrainModels()

	ctx := &core.PromptContext{
		UsePersona:       true,
		UseThinking:      true,
		IsLocalModel:     false,
		PersonaName:      cm.persona.Name,
		PersonaGender:    cm.persona.Gender,
		PersonaCharacter: cm.persona.Character,
		PersonaContent:   cm.persona.UserContent,
	}
	leftBrainPrompt := core.BuildCloudModelPrompt(ctx)

	leftModelName := brainModels.ConsciousnessLeftModel
	if leftModelName == "" {
		leftModelName = brainModels.ConsciousnessModel
	}
	if leftModelName == "" {
		leftModelName = modelsMgr.GetDefaultModel()
	}
	leftModel := modelsMgr.MustGetModel(leftModelName)

	rightModelName := brainModels.ConsciousnessRightModel
	if rightModelName == "" {
		rightModelName = brainModels.ConsciousnessModel
	}
	if rightModelName == "" {
		rightModelName = modelsMgr.GetDefaultModel()
	}
	rightModel := modelsMgr.MustGetModel(rightModelName)

	cm.leftBrain = NewThinking(leftModel, leftBrainPrompt, cm.logger, cm.tokenUsageRepo, &cm.cfg.TokenBudget)
	cm.rightBrain = NewThinking(rightModel, "", cm.logger, cm.tokenUsageRepo, &cm.cfg.TokenBudget)

	cm.logger.Info(i18n.T("brain.consciousness_dual_brain_created"),
		logging.String(i18n.T("brain.left_brain"), leftModel.Name),
		logging.String(i18n.T("brain.right_brain"), rightModel.Name))

	return nil
}

func (cm *ConsciousnessManager) Get() core.Thinking {
	return cm.consciousness
}

func (cm *ConsciousnessManager) GetLeftBrain() core.Thinking {
	return cm.leftBrain
}

func (cm *ConsciousnessManager) GetRightBrain() core.Thinking {
	return cm.rightBrain
}

func (cm *ConsciousnessManager) IsNil() bool {
	return cm.consciousness == nil && cm.leftBrain == nil
}

func (cm *ConsciousnessManager) HasDualBrain() bool {
	return cm.leftBrain != nil && cm.rightBrain != nil
}

func (cm *ConsciousnessManager) Think(ctx context.Context, question string, historyDialogue []*core.DialogueMessage, refs string) (*core.ThinkingResult, error) {
	if cm.consciousness != nil {
		return cm.consciousness.Think(ctx, question, historyDialogue, refs, false)
	}
	if cm.leftBrain != nil {
		return cm.leftBrain.Think(ctx, question, historyDialogue, refs, true)
	}
	return nil, fmt.Errorf("consciousness not initialized")
}
