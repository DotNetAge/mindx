package brain

import (
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
	}
}

func (cm *ConsciousnessManager) Create(capability *entity.Capability) {
	cm.logger.Info(i18n.T("brain.create_consciousness"),
		logging.String(i18n.T("brain.capability"), capability.Name),
		logging.String(i18n.T("brain.model"), capability.Model))

	personaInfo := fmt.Sprintf("\n\n## 人设信息\n名字: %s\n性别: %s\n性格: %s\n%s",
		cm.persona.Name, cm.persona.Gender, cm.persona.Character, cm.persona.UserContent)
	systemPrompt := capability.SystemPrompt + personaInfo

	modelConfig := &config.ModelConfig{
		Name:        capability.Model,
		APIKey:      capability.APIKey,
		BaseURL:     capability.BaseURL,
		Temperature: capability.Temperature,
		MaxTokens:   capability.MaxTokens,
	}

	cm.consciousness = NewThinking(modelConfig, systemPrompt, cm.logger, cm.tokenUsageRepo, &cm.cfg.Brain.TokenBudget)
	cm.logger.Info(i18n.T("brain.consciousness_created"))
}

func (cm *ConsciousnessManager) Get() core.Thinking {
	return cm.consciousness
}

func (cm *ConsciousnessManager) IsNil() bool {
	return cm.consciousness == nil
}

func (cm *ConsciousnessManager) Think(question string, historyDialogue []*core.DialogueMessage, refs string) (*core.ThinkingResult, error) {
	return cm.consciousness.Think(question, historyDialogue, refs, false)
}
