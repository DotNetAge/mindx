package processors

import (
	"context"
	"mindx/internal/core"
	"mindx/internal/entity"
)

// MockThinking 模拟 Thinking 接口（导出供其他包使用）
type MockThinking struct {
	ThinkFunc          func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error)
	ThinkWithToolsFunc func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error)
}

func (m *MockThinking) Think(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
	if m.ThinkFunc != nil {
		return m.ThinkFunc(ctx, question, history, references, jsonResult)
	}
	return &core.ThinkingResult{
		Intent:   "test_intent",
		Keywords: []string{"test"},
	}, nil
}

func (m *MockThinking) ThinkWithTools(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
	if m.ThinkWithToolsFunc != nil {
		return m.ThinkWithToolsFunc(ctx, question, history, tools, customSystemPrompt...)
	}
	return &core.ToolCallResult{NoCall: true}, nil
}

func (m *MockThinking) ReturnFuncResult(ctx context.Context, toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (string, error) {
	return "", nil
}

func (m *MockThinking) ReturnFuncResults(ctx context.Context, results []core.ToolExecResult, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (*core.ToolCallResult, error) {
	return nil, nil
}

func (m *MockThinking) CalculateMaxHistoryCount() int {
	return 10
}

func (m *MockThinking) SetEventChan(ch chan<- core.ThinkingEvent) {}

func (m *MockThinking) GetSystemPrompt() string {
	return ""
}

// MockMemory 模拟 Memory 接口（导出供其他包使用）
type MockMemory struct {
	SearchFunc func(terms string) ([]core.MemoryPoint, error)
}

func (m *MockMemory) Record(point core.MemoryPoint) error {
	return nil
}

func (m *MockMemory) Search(terms string) ([]core.MemoryPoint, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(terms)
	}
	return []core.MemoryPoint{}, nil
}

func (m *MockMemory) Optimize() error {
	return nil
}

func (m *MockMemory) ClusterConversations(conversations []entity.ConversationLog) error {
	return nil
}

// MockSkillManager 模拟 SkillManager 接口（导出供其他包使用）
type MockSkillManager struct {
	SearchSkillsFunc func(keywords ...string) ([]*core.Skill, error)
	ExecuteFuncFunc  func(function core.ToolCallFunction) (string, error)
}

func (m *MockSkillManager) Execute(skill *core.Skill, params map[string]interface{}) error {
	return nil
}

func (m *MockSkillManager) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	if m.ExecuteFuncFunc != nil {
		return m.ExecuteFuncFunc(function)
	}
	return "", nil
}

func (m *MockSkillManager) GetSkills() ([]*core.Skill, error) {
	return []*core.Skill{}, nil
}

func (m *MockSkillManager) SearchSkills(keywords ...string) ([]*core.Skill, error) {
	if m.SearchSkillsFunc != nil {
		return m.SearchSkillsFunc(keywords...)
	}
	return []*core.Skill{}, nil
}

func (m *MockSkillManager) RegisterInternalSkill(name string, fn func(params map[string]any) (string, error)) {
}
