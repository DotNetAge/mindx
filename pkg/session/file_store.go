package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/goreact/core"
	"gopkg.in/yaml.v3"
)

type yamlMessage struct {
	Role      string `yaml:"role"`
	Content   string `yaml:"content"`
	Timestamp int64  `yaml:"timestamp"`
}

func newYamlMessage(msg core.Message) yamlMessage {
	return yamlMessage{
		Role:      msg.Role,
		Content:   base64.StdEncoding.EncodeToString([]byte(msg.Content)),
		Timestamp: msg.Timestamp,
	}
}

func (ym yamlMessage) toCoreMessage() core.Message {
	decoded, _ := base64.StdEncoding.DecodeString(ym.Content)
	return core.Message{
		Role:      ym.Role,
		Content:   string(decoded),
		Timestamp: ym.Timestamp,
	}
}

var _ core.SessionStore = (*FileSessionStore)(nil)

type FileSessionStore struct {
	rootDir        string
	slideMu        sync.RWMutex
	slideHandler   core.SlideHandler
	tokenEstimator core.TokenEstimator
}

func NewFileSessionStore(rootDir string) (*FileSessionStore, error) {
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("create session store directory %s: %w", absPath, err)
	}

	return &FileSessionStore{
		rootDir:        absPath,
		slideHandler:   core.NoopSlideHandler,
		tokenEstimator: core.NewTokenEstimator(),
	}, nil
}

func (s *FileSessionStore) SetTokenEstimator(est core.TokenEstimator) {
	if est != nil {
		s.tokenEstimator = est
	}
}

func (s *FileSessionStore) agentDir(agentName string) string {
	return filepath.Join(s.rootDir, agentName)
}

func (s *FileSessionStore) sessionDir(agentName, sessionID string) string {
	return filepath.Join(s.agentDir(agentName), sessionID)
}

func (s *FileSessionStore) sessionFilePath(agentName, sessionID string) string {
	return filepath.Join(s.sessionDir(agentName, sessionID), "session.yml")
}

func (s *FileSessionStore) usageFilePath(agentName, sessionID string) string {
	return filepath.Join(s.sessionDir(agentName, sessionID), "usages.yml")
}

func (s *FileSessionStore) findSessionDir(sessionID string) string {
	var result string
	_ = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || info.Name() != sessionID {
			return nil
		}
		sessionFile := filepath.Join(path, "session.yml")
		if info.IsDir() {
			if _, statErr := os.Stat(sessionFile); statErr == nil {
				result = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return result
}

func (s *FileSessionStore) Append(ctx context.Context, sessionID string, agentName string, msg core.Message) error {
	if msg.Role == "system" {
		return nil
	}

	timestamp := msg.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}
	msg.Timestamp = timestamp

	dir := s.sessionDir(agentName, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir %s: %w", dir, err)
	}

	path := s.sessionFilePath(agentName, sessionID)

	var ymlMsgs []yamlMessage
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &ymlMsgs); err != nil {
			return fmt.Errorf("parse existing session file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read session file %s: %w", path, err)
	}

	ymlMsgs = append(ymlMsgs, newYamlMessage(msg))

	data, err := yaml.Marshal(ymlMsgs)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write session file %s: %w", path, err)
	}

	s.updateSessionMeta(dir)

	return nil
}

func (s *FileSessionStore) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	var msgs []core.Message

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}
		parentDir := filepath.Base(filepath.Dir(path))
		if parentDir != sessionID {
			return nil
		}

		parsed, parseErr := parseMessagesFromFile(path)
		if parseErr != nil {
			return nil
		}
		msgs = parsed
		return filepath.SkipAll
	})

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Timestamp < msgs[j].Timestamp })
	return msgs, err
}

func (s *FileSessionStore) CurrentContext(ctx context.Context, agentName string, maxTokens int64) ([]core.Message, error) {
	sessionID, err := s.findSessionByAgent(agentName)
	if err != nil {
		return nil, err
	}

	allMsgs, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var result []core.Message
	var totalTokens int64
	for i := len(allMsgs) - 1; i >= 0; i-- {
		msg := allMsgs[i]
		tokens := int64(s.tokenEstimator.Estimate(msg.Content))
		if totalTokens+tokens > maxTokens && len(result) > 0 {
			break
		}
		totalTokens += tokens
		result = append(result, msg)
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	if len(allMsgs) > len(result) && len(result) > 0 {
		evicted := allMsgs[:len(allMsgs)-len(result)]
		s.slideMu.RLock()
		handler := s.slideHandler
		s.slideMu.RUnlock()
		core.EmitSlideEvent(handler, ctx, core.SlideEvent{
			SessionID: sessionID,
			Slided:    evicted,
			Remaining: len(result),
			Timestamp: time.Now().UnixMilli(),
		})
	}

	return result, nil
}

func (s *FileSessionStore) Delete(ctx context.Context, timestamp int64, sessionID string) error {
	path := ""

	_ = filepath.Walk(s.rootDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}
		parentDir := filepath.Base(filepath.Dir(p))
		if parentDir != sessionID {
			return nil
		}
		path = p
		return filepath.SkipAll
	})

	if path == "" {
		return nil
	}

	msgs, err := parseMessagesFromFile(path)
	if err != nil {
		return err
	}

	var filtered []core.Message
	for _, m := range msgs {
		if m.Timestamp != timestamp {
			filtered = append(filtered, m)
		}
	}

	return writeMessagesToFile(path, filtered)
}

func (s *FileSessionStore) Clear(ctx context.Context, sessionID string) error {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil
	}

	_ = os.RemoveAll(dirPath)
	return nil
}

func (s *FileSessionStore) SetSlideHandler(handler core.SlideHandler) {
	s.slideMu.Lock()
	defer s.slideMu.Unlock()
	s.slideHandler = handler
}

func (s *FileSessionStore) Close() error {
	return nil
}

type yamlUsage struct {
	Timestamp    time.Time `yaml:"timestamp"`
	InputTokens  int       `yaml:"input_tokens"`
	OutputTokens int       `yaml:"output_tokens"`
	RemainTokens int       `yaml:"remain_tokens"`
}

func toYamlUsage(u core.TokenUsage) yamlUsage {
	return yamlUsage{
		Timestamp:    u.Timestamp,
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		RemainTokens: u.RemainTokens,
	}
}

func fromYamlUsage(yu yamlUsage) core.TokenUsage {
	return core.TokenUsage{
		Timestamp:    yu.Timestamp,
		InputTokens:  yu.InputTokens,
		OutputTokens: yu.OutputTokens,
		RemainTokens: yu.RemainTokens,
	}
}

func (s *FileSessionStore) AppendTokenUsage(_ context.Context, sessionID string, usage core.TokenUsage) error {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return fmt.Errorf("session %q not found", sessionID)
	}

	path := filepath.Join(dirPath, "usages.yml")

	var usages []yamlUsage
	if data, err := os.ReadFile(path); err == nil {
		if unmarshalErr := yaml.Unmarshal(data, &usages); unmarshalErr != nil {
			return fmt.Errorf("parse usages file %s: %w", path, unmarshalErr)
		}
	}

	usages = append(usages, toYamlUsage(usage))

	data, err := yaml.Marshal(usages)
	if err != nil {
		return fmt.Errorf("marshal usages: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (s *FileSessionStore) GetTokenUsages(_ context.Context, sessionID string) ([]core.TokenUsage, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, nil
	}

	path := filepath.Join(dirPath, "usages.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var yamlUsages []yamlUsage
	if err := yaml.Unmarshal(data, &yamlUsages); err != nil {
		return nil, fmt.Errorf("parse usages file: %w", err)
	}

	usages := make([]core.TokenUsage, len(yamlUsages))
	for i, yu := range yamlUsages {
		usages[i] = fromYamlUsage(yu)
	}

	return usages, nil
}

func (s *FileSessionStore) GetByRole(ctx context.Context, agent string) (*core.SessionInfo, error) {
	var bestInfo *core.SessionInfo

	agentDir := s.agentDir(agent)

	_ = filepath.Walk(agentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}

		sessionID := filepath.Base(filepath.Dir(path))
		si, statErr := statSessionInfo(agent, sessionID, path)
		if statErr != nil {
			return nil
		}

		if bestInfo == nil || si.LastActivityAt.After(bestInfo.LastActivityAt) {
			bestInfo = si
		}
		return nil
	})

	if bestInfo == nil {
		return nil, core.ErrSessionNotFound
	}

	messages, _ := s.Get(ctx, bestInfo.SessionID)
	bestInfo.Messages = messages
	return bestInfo, nil
}

func (s *FileSessionStore) ListSessions(ctx context.Context) ([]core.SessionInfo, error) {
	var infos []core.SessionInfo

	_ = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}

		relPath, relErr := filepath.Rel(s.rootDir, path)
		if relErr != nil {
			return nil
		}

		parts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(parts) < 3 {
			return nil
		}
		agentName := parts[0]
		sessionID := parts[1]

		si, statErr := statSessionInfo(agentName, sessionID, path)
		if statErr != nil {
			return nil
		}
		infos = append(infos, *si)
		return nil
	})

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].LastActivityAt.After(infos[j].LastActivityAt)
	})
	return infos, nil
}

func (s *FileSessionStore) findSessionByAgent(agentName string) (string, error) {
	agentDir := s.agentDir(agentName)

	var bestSession string
	var bestModTime time.Time

	_ = filepath.Walk(agentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}
		modTime := info.ModTime()
		if modTime.After(bestModTime) {
			bestModTime = modTime
			bestSession = filepath.Base(filepath.Dir(path))
		}
		return nil
	})

	if bestSession == "" {
		return "", core.ErrSessionNotFound
	}
	return bestSession, nil
}

func parseMessagesFromFile(path string) ([]core.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ymlMsgs []yamlMessage
	if err := yaml.Unmarshal(data, &ymlMsgs); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	msgs := make([]core.Message, len(ymlMsgs))
	for i, ym := range ymlMsgs {
		msgs[i] = ym.toCoreMessage()
	}

	return msgs, nil
}

func writeMessagesToFile(path string, msgs []core.Message) error {
	ymlMsgs := make([]yamlMessage, len(msgs))
	for i, msg := range msgs {
		ymlMsgs[i] = newYamlMessage(msg)
	}

	data, err := yaml.Marshal(ymlMsgs)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func statSessionInfo(agentName, sessionID, path string) (*core.SessionInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &core.SessionInfo{
		SessionID:      sessionID,
		AgentName:      agentName,
		LastActivityAt: info.ModTime(),
		CreatedAt:      info.ModTime(),
	}, nil
}

// GetSessionMeta loads session metadata for the given session ID.
// It searches all agent directories to find the session.
func (s *FileSessionStore) GetSessionMeta(sessionID string) (*SessionMeta, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, core.ErrSessionNotFound
	}
	return LoadSessionMeta(dirPath)
}

// updateSessionMeta updates the UpdatedAt and LastActivityAt timestamps in meta.json.
// This is called after each message append to keep metadata current.
func (s *FileSessionStore) updateSessionMeta(sessionDir string) {
	meta, err := LoadSessionMeta(sessionDir)
	if err != nil {
		return
	}
	meta.UpdatedAt = time.Now()
	meta.LastActivityAt = time.Now()
	meta.MessageCount++
	_ = meta.Save(sessionDir)
}
