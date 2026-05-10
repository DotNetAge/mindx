package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ChatSession 持久化 TUI 会话状态，确保客户端与服务端会话一致。
type ChatSession struct {
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id"`
}

// chatSessionManager 管理 chat.json 文件的读写，线程安全。
type chatSessionManager struct {
	mu      sync.RWMutex
	filePath string
	session *ChatSession
}

var (
	chatManager *chatSessionManager
	once        sync.Once
)

// GetChatSessionManager 返回全局单例的 chatSessionManager。
func GetChatSessionManager() *chatSessionManager {
	once.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		mindxDir := filepath.Join(homeDir, ".mindx")
		chatManager = &chatSessionManager{
			filePath: filepath.Join(mindxDir, "chat.json"),
			session:  &ChatSession{},
		}
	})
	return chatManager
}

// Exists 检查 chat.json 文件是否存在。
func (m *chatSessionManager) Exists() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, err := os.Stat(m.filePath)
	return !os.IsNotExist(err)
}

// Load 从 chat.json 加载会话状态。返回是否成功加载。
func (m *chatSessionManager) Load() (*ChatSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 chat.json 失败: %w", err)
	}

	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("解析 chat.json 失败: %w", err)
	}

	m.session = &session
	return &session, nil
}

// Save 将当前会话状态写入 chat.json。
func (m *chatSessionManager) Save(agentName, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.session = &ChatSession{
		AgentName: agentName,
		SessionID: sessionID,
	}

	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建 .mindx 目录失败: %w", err)
	}

	data, err := json.MarshalIndent(m.session, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入 chat.json 失败: %w", err)
	}

	return nil
}

// GetCurrentSession 获取当前缓存的会话信息（不加锁）。
func (m *chatSessionManager) GetCurrentSession() *ChatSession {
	return m.session
}

// Update 更新内存中的会话信息并立即持久化。
func (m *chatSessionManager) Update(agentName, sessionID string) error {
	return m.Save(agentName, sessionID)
}
