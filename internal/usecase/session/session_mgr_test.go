package session

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionMgr_NewSessionMgr(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")
	maxTokens := 1000

	sm := NewSessionMgr(maxTokens, storage, logger)

	if sm == nil {
		t.Fatal("SessionMgr 应该被创建")
	}

	if sm.maxTokens != maxTokens {
		t.Errorf("期望 maxTokens=%d, 实际=%d", maxTokens, sm.maxTokens)
	}
}

func TestSessionMgr_RestoreSession(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)

	// 测试首次恢复（应该创建新会话）
	err := sm.RestoreSession()
	if err != nil {
		t.Fatalf("RestoreSession 失败: %v", err)
	}

	// 验证创建了当前会话
	current, ok := sm.GetCurrentSession()
	if !ok || current == nil {
		t.Fatal("应该创建当前会话")
	}

	if current.TokensUsed != 0 {
		t.Errorf("新会话 TokensUsed 应该为 0, 实际=%d", current.TokensUsed)
	}
}

func TestSessionMgr_RecordMessage(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	msg1 := entity.Message{
		Role:    "user",
		Content: "你好",
		Time:    time.Now(),
	}

	err := sm.RecordMessage(msg1)
	if err != nil {
		t.Fatalf("RecordMessage 失败: %v", err)
	}

	// 验证消息已添加
	current, _ := sm.GetCurrentSession()
	if len(current.Messages) != 1 {
		t.Errorf("期望 1 条消息, 实际=%d", len(current.Messages))
	}

	if current.Messages[0].Content != "你好" {
		t.Errorf("消息内容不匹配")
	}

	// 添加第二条消息
	msg2 := entity.Message{
		Role:    "assistant",
		Content: "你好！很高兴认识你",
		Time:    time.Now(),
	}

	_ = sm.RecordMessage(msg2)
	current, _ = sm.GetCurrentSession()

	if len(current.Messages) != 2 {
		t.Errorf("期望 2 条消息, 实际=%d", len(current.Messages))
	}
}

func TestSessionMgr_SessionEnding(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(10, storage, logger) // 设置很小的 maxTokens
	_ = sm.RestoreSession()

	// 设置会话结束回调
	sessionEnded := false
	var endedSession entity.Session

	SetOnSessionEnd(sm, func(sess entity.Session) bool {
		sessionEnded = true
		endedSession = sess
		return true
	})

	// 添加多条消息直到会话结束
	messages := []string{
		"你好", "你好", "今天天气怎么样",
	}

	for _, msgContent := range messages {
		msg := entity.Message{
			Role:    "user",
			Content: msgContent,
			Time:    time.Now(),
		}

		_ = sm.RecordMessage(msg)
	}

	// 验证会话已结束
	if !sessionEnded {
		t.Error("会话应该已经结束")
	}

	if endedSession.ID == "" {
		t.Error("结束的会话应该有 ID")
	}

	if !endedSession.IsEnded {
		t.Error("会话应该被标记为已结束")
	}

	// 验证开启了新会话
	current, _ := sm.GetCurrentSession()
	if current.ID == endedSession.ID {
		t.Error("应该创建新会话")
	}

	if current.TokensUsed != 0 {
		t.Errorf("新会话 TokensUsed 应该为 0, 实际=%d", current.TokensUsed)
	}
}

func TestSessionMgr_GetHistory(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	// 初始应该为空
	history := sm.GetHistory()
	if len(history) != 0 {
		t.Errorf("初始历史应该为空, 实际长度=%d", len(history))
	}

	// 添加消息
	msg := entity.Message{
		Role:    "user",
		Content: "测试消息",
		Time:    time.Now(),
	}
	_ = sm.RecordMessage(msg)

	// 验证历史
	history = sm.GetHistory()
	if len(history) != 1 {
		t.Errorf("期望 1 条历史记录, 实际=%d", len(history))
	}

	if history[0].Content != "测试消息" {
		t.Errorf("历史记录内容不匹配")
	}
}

func TestSessionMgr_UpdateTokensFromModel(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	// 不通过 RecordMessage 添加消息，直接测试 UpdateTokensFromModel
	// 这样可以避免 RecordMessage 中的 token 计算影响测试

	// 从模型更新 Token
	sm.UpdateTokensFromModel(core.TokenUsage{
		PromptTokens:     50,
		CompletionTokens: 30,
		TotalTokens:      80,
	})

	// 验证 Token 已累积
	current, _ := sm.GetCurrentSession()
	if current.TokensUsed != 80 {
		t.Errorf("期望 TokensUsed=80, 实际=%d", current.TokensUsed)
	}
}

func TestSessionMgr_CleanupUnmemorizedSessions(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	// 先创建会话并保存到存储
	session1 := &entity.Session{
		ID:         "session_1",
		Messages:   []entity.Message{},
		TokensUsed: 100,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}
	session2 := &entity.Session{
		ID:         "session_2",
		Messages:   []entity.Message{},
		TokensUsed: 100,
		IsEnded:    true,
		CreatedAt:  time.Now(),
		EndedAt:    time.Now(),
	}

	_ = storage.Save(*session1)
	_ = storage.Save(*session2)

	// 然后创建 SessionMgr 并恢复会话
	sm := NewSessionMgr(100, storage, logger)
	_ = sm.RestoreSession()

	// 记录一个 CheckPoint
	sm.RecordCheckPoint("session_1")

	// 获取未记忆的会话
	unmemorized := sm.CleanupUnmemorizedSessions()

	// 应该只有 session_2 未记忆
	if len(unmemorized) != 1 {
		t.Errorf("期望 1 个未记忆的会话, 实际=%d", len(unmemorized))
	}

	if unmemorized[0].ID != "session_2" {
		t.Errorf("未记忆的会话 ID 应该是 session_2, 实际=%s", unmemorized[0].ID)
	}
}

func TestSessionMgr_SessionPersistence(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	// 添加消息
	msg := entity.Message{
		Role:    "user",
		Content: "测试持久化",
		Time:    time.Now(),
	}
	_ = sm.RecordMessage(msg)

	// 获取当前会话 ID
	sess1, _ := sm.GetCurrentSession()
	sessionID := sess1.ID

	// 手动保存会话到存储（模拟会话结束后）
	_ = storage.Save(*sess1)

	// 重新创建 SessionMgr（模拟重启）
	sm2 := NewSessionMgr(1000, storage, logger)
	err := sm2.RestoreSession()
	if err != nil {
		t.Fatalf("RestoreSession 失败: %v", err)
	}

	// 验证会话已恢复
	sess2, _ := sm2.GetCurrentSession()
	if sess2.ID != sessionID {
		t.Errorf("会话 ID 应该一致, 期望=%s, 实际=%s", sessionID, sess2.ID)
	}

	if len(sess2.Messages) != 1 {
		t.Errorf("期望恢复 1 条消息, 实际=%d", len(sess2.Messages))
	}

	if sess2.Messages[0].Content != "测试持久化" {
		t.Errorf("恢复的消息内容不匹配")
	}
}

func TestSessionMgr_GetAllSessions(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	// 手动添加会话到存储
	for i := 1; i <= 3; i++ {
		session := entity.Session{
			ID:         fmt.Sprintf("test_session_%d", i),
			Messages:   []entity.Message{},
			TokensUsed: 100 * i,
			IsEnded:    true,
			CreatedAt:  time.Now(),
			EndedAt:    time.Now(),
		}
		_ = storage.Save(session)
	}

	// 获取所有会话
	allSessions, err := sm.GetAllSessions()
	if err != nil {
		t.Fatalf("GetAllSessions 失败: %v", err)
	}

	// 验证会话数量（至少3个，加上当前会话可能更多）
	if len(allSessions) < 3 {
		t.Errorf("期望至少 3 个会话, 实际=%d", len(allSessions))
	}
}

func TestSessionMgr_RecordCheckPoint(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)
	logger := logging.GetSystemLogger().Named("test")

	sm := NewSessionMgr(1000, storage, logger)
	_ = sm.RestoreSession()

	sessionID := "test_session_123"

	// 记录 CheckPoint
	sm.RecordCheckPoint(sessionID)

	// 获取未记忆的会话（应该不包含这个 session）
	unmemorized := sm.CleanupUnmemorizedSessions()

	for _, sess := range unmemorized {
		if sess.ID == sessionID {
			t.Errorf("session %s 应该已经被记忆，不应该在未记忆列表中", sessionID)
		}
	}
}

func TestFileSessionStorage_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)

	session := entity.Session{
		ID:         "test_session",
		Messages: []entity.Message{
			{Role: "user", Content: "你好", Time: time.Now()},
			{Role: "assistant", Content: "你好！", Time: time.Now()},
		},
		TokensUsed: 100,
		IsEnded:    false,
		CreatedAt:  time.Now(),
		EndedAt:    time.Time{},
	}

	// 保存
	err := storage.Save(session)
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 验证文件已创建
	filePath := filepath.Join(tempDir, "test_session.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("会话文件应该被创建")
	}

	// 加载
	loaded, err := storage.Load("test_session")
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	// 验证内容
	if loaded.ID != session.ID {
		t.Errorf("ID 不匹配")
	}

	if len(loaded.Messages) != len(session.Messages) {
		t.Errorf("消息数量不匹配")
	}

	if loaded.Messages[0].Content != "你好" {
		t.Errorf("第一条消息内容不匹配")
	}
}

func TestFileSessionStorage_LoadAll(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileSessionStorage(tempDir)

	// 创建多个会话
	sessions := []entity.Session{
		{
			ID:         "session_1",
			Messages:   []entity.Message{{Role: "user", Content: "测试1", Time: time.Now()}},
			TokensUsed: 100,
			IsEnded:    true,
			CreatedAt:  time.Now(),
			EndedAt:    time.Now(),
		},
		{
			ID:         "session_2",
			Messages:   []entity.Message{{Role: "user", Content: "测试2", Time: time.Now()}},
			TokensUsed: 200,
			IsEnded:    true,
			CreatedAt:  time.Now(),
			EndedAt:    time.Now(),
		},
	}

	for _, sess := range sessions {
		_ = storage.Save(sess)
	}

	// 加载所有会话
	allSessions, err := storage.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll 失败: %v", err)
	}

	// 验证数量
	if len(allSessions) != len(sessions) {
		t.Errorf("期望 %d 个会话, 实际=%d", len(sessions), len(allSessions))
	}
}
