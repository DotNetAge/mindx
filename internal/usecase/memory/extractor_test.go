package memory

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"
	"time"
)

// MockThinking Mock 的 Thinking 接口实现
type MockThinking struct {
	thinkFunc func(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error)
	stream    any
}

func (m *MockThinking) Think(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
	if m.thinkFunc != nil {
		return m.thinkFunc(question, history, references, jsonResult)
	}
	return nil, nil
}

func (m *MockThinking) ThinkWithTools(question string, history []*core.DialogueMessage, tools []*core.ToolSchema) (*core.ToolCallResult, error) {
	return nil, nil
}

func (m *MockThinking) ReturnFuncResult(toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (string, error) {
	return "", nil
}

func (m *MockThinking) CalculateMaxHistoryCount() int {
	return 10
}

func (m *MockThinking) SetStream(stream any) {
	m.stream = stream
}

func (m *MockThinking) SetEventChan(ch chan<- core.ThinkingEvent) {}

// MockMemory Mock 的 Memory 接口实现
type MockMemory struct {
	recordFunc func(mem core.MemoryPoint) error
	memories   []core.MemoryPoint
}

func (m *MockMemory) Record(mem core.MemoryPoint) error {
	if m.recordFunc != nil {
		return m.recordFunc(mem)
	}
	m.memories = append(m.memories, mem)
	return nil
}

func (m *MockMemory) RecordBatch(mems []core.MemoryPoint) error {
	for _, mem := range mems {
		if err := m.Record(mem); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockMemory) Search(query string) ([]core.MemoryPoint, error) {
	// 简单的关键词匹配
	var result []core.MemoryPoint
	for _, mem := range m.memories {
		if containsKeyword(query, mem.Keywords) {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *MockMemory) Get(id int) (*core.MemoryPoint, error) {
	for _, mem := range m.memories {
		if mem.ID == id {
			return &mem, nil
		}
	}
	return nil, nil
}

func (m *MockMemory) GetAll() ([]core.MemoryPoint, error) {
	return m.memories, nil
}

func (m *MockMemory) Update(mem core.MemoryPoint) error {
	return nil
}

func (m *MockMemory) Delete(id int) error {
	return nil
}

func (m *MockMemory) Optimize() error {
	return nil
}

func (m *MockMemory) CleanupExpired() error {
	return nil
}

func (m *MockMemory) AdjustWeight(id int, timeWeight, repeatWeight, emphasisWeight float64) error {
	return nil
}

func (m *MockMemory) ClusterConversations(conversations []entity.ConversationLog) error {
	return nil
}

func (m *MockMemory) Close() error {
	return nil
}

func containsKeyword(query string, keywords []string) bool {
	for _, kw := range keywords {
		if contains(query, kw) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLLMExtractor_NewLLMExtractor(t *testing.T) {
	mockBrain := &MockThinking{}
	mockMemory := &MockMemory{}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	if extractor == nil {
		t.Fatal("Extractor 应该被创建")
	}

	if extractor.brain != mockBrain {
		t.Error("brain 应该被设置")
	}

	if extractor.memory != mockMemory {
		t.Error("memory 应该被设置")
	}
}

func TestLLMExtractor_Extract_Success(t *testing.T) {
	mockBrain := &MockThinking{
		thinkFunc: func(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 返回有效的 JSON 响应
			jsonResponse := `{"memories": [{"topic": "编程", "keywords": ["编程", "Go"], "summary": "用户喜欢编程", "content": "用户喜欢使用 Go 语言进行编程"}]}`
			return &core.ThinkingResult{
				Answer: jsonResponse,
			}, nil
		},
	}

	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID: "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "我喜欢编程", Time: time.Now()},
			{Role: "assistant", Content: "太棒了", Time: time.Now()},
		},
		TokensUsed: 100,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.Extract(session)

	if !result {
		t.Error("Extract 应该返回 true")
	}

	// 验证记忆被记录
	if len(mockMemory.memories) != 1 {
		t.Errorf("期望记录 1 条记忆, 实际=%d", len(mockMemory.memories))
	}

	mem := mockMemory.memories[0]
	if mem.Summary != "用户喜欢编程" {
		t.Errorf("摘要不匹配, 期望=用户喜欢编程, 实际=%s", mem.Summary)
	}

	if len(mem.Keywords) != 2 {
		t.Errorf("关键词数量不匹配, 期望=2, 实际=%d", len(mem.Keywords))
	}
}

func TestLLMExtractor_Extract_EmptySession(t *testing.T) {
	mockBrain := &MockThinking{}
	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID:         "test_session",
		Messages:   []entity.Message{},
		TokensUsed: 0,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.Extract(session)

	if !result {
		t.Error("空会话应该返回 true")
	}

	if len(mockMemory.memories) != 0 {
		t.Errorf("空会话不应该记录记忆, 实际=%d", len(mockMemory.memories))
	}
}

func TestLLMExtractor_Extract_InvalidJSON(t *testing.T) {
	mockBrain := &MockThinking{
		thinkFunc: func(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 返回无效的 JSON
			return &core.ThinkingResult{
				Answer: "这不是 JSON 响应",
			}, nil
		},
	}

	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID: "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "测试", Time: time.Now()},
		},
		TokensUsed: 50,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.Extract(session)

	// 应该使用 fallback 创建记忆
	if !result {
		t.Error("应该创建 fallback 记忆")
	}

	if len(mockMemory.memories) != 1 {
		t.Errorf("期望记录 1 条 fallback 记忆, 实际=%d", len(mockMemory.memories))
	}

	mem := mockMemory.memories[0]
	if len(mem.Keywords) != 1 || mem.Keywords[0] != "对话" {
		t.Errorf("fallback 记忆关键词不正确, 期望=[对话], 实际=%v", mem.Keywords)
	}
}

func TestLLMExtractor_Extract_BrainError(t *testing.T) {
	mockBrain := &MockThinking{
		thinkFunc: func(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, nil // 返回 nil 表示错误
		},
	}

	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID: "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "测试", Time: time.Now()},
		},
		TokensUsed: 50,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.Extract(session)

	if result {
		t.Error("Brain 错误应该返回 false")
	}

	if len(mockMemory.memories) != 0 {
		t.Errorf("Brain 错误不应该记录记忆, 实际=%d", len(mockMemory.memories))
	}
}

func TestLLMExtractor_Extract_MultipleMemories(t *testing.T) {
	mockBrain := &MockThinking{
		thinkFunc: func(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 返回多个记忆点
			jsonResponse := `{
				"memories": [
					{"topic": "编程", "keywords": ["编程", "Go"], "summary": "用户喜欢编程", "content": "用户喜欢使用 Go 语言进行编程"},
					{"topic": "居住", "keywords": ["上海", "浦东"], "summary": "用户住在上海", "content": "用户住在上海市浦东新区"}
				]
			}`
			return &core.ThinkingResult{
				Answer: jsonResponse,
			}, nil
		},
	}

	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}

	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID: "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "我住在上海浦东，喜欢用 Go 编程", Time: time.Now()},
		},
		TokensUsed: 100,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.Extract(session)

	if !result {
		t.Error("Extract 应该返回 true")
	}

	// 验证记录了 2 条记忆
	if len(mockMemory.memories) != 2 {
		t.Errorf("期望记录 2 条记忆, 实际=%d", len(mockMemory.memories))
	}

	// 验证第一段记忆
	if mockMemory.memories[0].Summary != "用户喜欢编程" {
		t.Errorf("第一段记忆摘要不匹配")
	}

	// 验证第二段记忆
	if mockMemory.memories[1].Summary != "用户住在上海" {
		t.Errorf("第二段记忆摘要不匹配")
	}
}

func TestLLMExtractor_formatConversation(t *testing.T) {
	mockBrain := &MockThinking{}
	mockMemory := &MockMemory{}
	extractor := NewLLMExtractor(mockBrain, mockMemory)

	messages := []entity.Message{
		{Role: "user", Content: "你好", Time: time.Now()},
		{Role: "assistant", Content: "你好！", Time: time.Now()},
		{Role: "user", Content: "我叫张三", Time: time.Now()},
	}

	result := extractor.formatConversation(messages)

	// 验证格式
	if !contains(result, "[user] 你好") {
		t.Error("应该包含 user 消息")
	}

	if !contains(result, "[assistant] 你好！") {
		t.Error("应该包含 assistant 消息")
	}

	if !contains(result, "[user] 我叫张三") {
		t.Error("应该包含第二条 user 消息")
	}
}

func TestLLMExtractor_buildPrompt(t *testing.T) {
	mockBrain := &MockThinking{}
	mockMemory := &MockMemory{}
	extractor := NewLLMExtractor(mockBrain, mockMemory)

	prompt := extractor.buildPrompt("测试对话内容")

	// 验证 prompt 包含关键指令
	expectedPhrases := []string{
		"智能记忆提取助手",
		"话题分类",
		"关键词",
		"摘要",
		"主要内容",
		"JSON",
	}

	for _, phrase := range expectedPhrases {
		if !contains(prompt, phrase) {
			t.Errorf("prompt 应该包含 '%s'", phrase)
		}
	}

	// 验证 prompt 包含对话内容
	if !contains(prompt, "测试对话内容") {
		t.Error("prompt 应该包含对话内容")
	}
}

func TestLLMExtractor_createFallbackMemory(t *testing.T) {
	mockBrain := &MockThinking{}
	mockMemory := &MockMemory{
		memories: []core.MemoryPoint{},
	}
	extractor := NewLLMExtractor(mockBrain, mockMemory)

	session := entity.Session{
		ID: "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "消息1", Time: time.Now()},
			{Role: "assistant", Content: "回复1", Time: time.Now()},
			{Role: "user", Content: "消息2", Time: time.Now()},
		},
		TokensUsed: 100,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	result := extractor.createFallbackMemory(session)

	if !result {
		t.Error("createFallbackMemory 应该返回 true")
	}

	if len(mockMemory.memories) != 1 {
		t.Errorf("期望记录 1 条 fallback 记忆, 实际=%d", len(mockMemory.memories))
	}

	mem := mockMemory.memories[0]

	// 验证关键词
	if len(mem.Keywords) != 1 || mem.Keywords[0] != "对话" {
		t.Errorf("fallback 记忆关键词不正确, 实际=%v", mem.Keywords)
	}

	// 验证摘要
	if !contains(mem.Summary, "包含") || !contains(mem.Summary, "消息") {
		t.Errorf("fallback 记忆摘要不正确, 实际=%s", mem.Summary)
	}

	// 验证内容包含所有消息
	content := mem.Content
	if !contains(content, "user: 消息1") {
		t.Error("内容应该包含第一条消息")
	}

	if !contains(content, "assistant: 回复1") {
		t.Error("内容应该包含第一条回复")
	}

	if !contains(content, "user: 消息2") {
		t.Error("内容应该包含第二条消息")
	}
}
