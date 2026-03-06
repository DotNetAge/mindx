package processors

import (
	"context"
	"mindx/internal/core"
	"mindx/internal/entity"
)

// MockThinking - Mock for core.Thinking interface
type MockThinking struct {
	ThinkFunc             func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error)
	ThinkWithToolsFunc    func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error)
	ReturnFuncResultFunc  func(ctx context.Context, toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (string, error)
	ReturnFuncResultsFunc func(ctx context.Context, results []core.ToolExecResult, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (*core.ToolCallResult, error)
	MaxHistoryCount       int
	SystemPrompt          string
	EventChan             chan<- core.ThinkingEvent
}

func (m *MockThinking) Think(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
	if m.ThinkFunc != nil {
		return m.ThinkFunc(ctx, question, history, references, jsonResult)
	}
	return &core.ThinkingResult{}, nil
}

func (m *MockThinking) ThinkWithTools(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
	if m.ThinkWithToolsFunc != nil {
		return m.ThinkWithToolsFunc(ctx, question, history, tools, customSystemPrompt...)
	}
	return &core.ToolCallResult{NoCall: true}, nil
}

func (m *MockThinking) ReturnFuncResult(ctx context.Context, toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (string, error) {
	if m.ReturnFuncResultFunc != nil {
		return m.ReturnFuncResultFunc(ctx, toolCallID, name, result, originalArgs, history, tools, question)
	}
	return "", nil
}

func (m *MockThinking) ReturnFuncResults(ctx context.Context, results []core.ToolExecResult, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (*core.ToolCallResult, error) {
	if m.ReturnFuncResultsFunc != nil {
		return m.ReturnFuncResultsFunc(ctx, results, history, tools, question)
	}
	return &core.ToolCallResult{NoCall: true}, nil
}

func (m *MockThinking) CalculateMaxHistoryCount() int {
	if m.MaxHistoryCount > 0 {
		return m.MaxHistoryCount
	}
	return 10 // default
}

func (m *MockThinking) SetEventChan(ch chan<- core.ThinkingEvent) {
	m.EventChan = ch
}

func (m *MockThinking) GetSystemPrompt() string {
	if m.SystemPrompt != "" {
		return m.SystemPrompt
	}
	return "default system prompt"
}

// MockMemory - Mock for Memory interface
type MockMemory struct {
	SearchFunc func(terms string) ([]core.MemoryPoint, error)
}

func (m *MockMemory) Search(terms string) ([]core.MemoryPoint, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(terms)
	}
	return []core.MemoryPoint{}, nil
}

// MockSkillSearcher - Mock for SkillSearcher interface
type MockSkillSearcher struct {
	SearchFunc func(query string, topK int) ([]*entity.SkillMatch, error)
}

func (m *MockSkillSearcher) Search(query string, topK int) ([]*entity.SkillMatch, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(query, topK)
	}
	return []*entity.SkillMatch{}, nil
}

// MockToolAssembler - Mock for ToolAssembler interface
type MockToolAssembler struct {
	AssembleToolsFunc func(skill *entity.Skill) ([]entity.ToolSchema, error)
}

func (m *MockToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
	if m.AssembleToolsFunc != nil {
		return m.AssembleToolsFunc(skill)
	}
	return []entity.ToolSchema{}, nil
}

// MockToolExecutor - Mock for ToolExecutor interface
type MockToolExecutor struct {
	ExecuteFuncFunc func(function core.ToolCallFunction) (string, error)
}

func (m *MockToolExecutor) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	if m.ExecuteFuncFunc != nil {
		return m.ExecuteFuncFunc(function)
	}
	return "", nil
}

// MockSkillManager - Mock for backward compatibility
type MockSkillManager struct {
	SearchSkillsFunc func(keywords ...string) ([]string, error)
	ExecuteFuncFunc  func(function core.ToolCallFunction) (string, error)
}

func (m *MockSkillManager) SearchSkills(keywords ...string) ([]string, error) {
	if m.SearchSkillsFunc != nil {
		return m.SearchSkillsFunc(keywords...)
	}
	return []string{}, nil
}

func (m *MockSkillManager) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	if m.ExecuteFuncFunc != nil {
		return m.ExecuteFuncFunc(function)
	}
	return "", nil
}
