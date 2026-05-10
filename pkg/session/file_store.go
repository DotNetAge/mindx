package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func (s *FileSessionStore) sessionPath(agentName string, sessionID string) string {
	return filepath.Join(s.agentDir(agentName), sessionID+".yml")
}

func (s *FileSessionStore) Append(ctx context.Context, sessionID string, agentName string, msg core.Message) error {
	if msg.Role == "system" {
		return nil
	}

	dir := s.agentDir(agentName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agent dir %s: %w", dir, err)
	}

	path := s.sessionPath(agentName, sessionID)

	var ymlMsgs []yamlMessage
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &ymlMsgs); err != nil {
			return fmt.Errorf("parse existing session file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read session file %s: %w", path, err)
	}

	timestamp := msg.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}
	msg.Timestamp = timestamp
	ymlMsgs = append(ymlMsgs, newYamlMessage(msg))

	data, err := yaml.Marshal(ymlMsgs)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write session file %s: %w", path, err)
	}

	return nil
}

func (s *FileSessionStore) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	var msgs []core.Message

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".yml") || info.Name() != sessionID+".yml" {
			return nil
		}

		parsed, err := parseMessagesFromFile(path)
		if err != nil {
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

	filepath.Walk(s.rootDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".yml") || info.Name() != sessionID+".yml" {
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
	var walkErr error
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".yml") || info.Name() != sessionID+".yml" {
			return nil
		}
		if removeErr := os.Remove(path); removeErr != nil {
			walkErr = removeErr
			return filepath.SkipAll
		}
		return filepath.SkipAll
	})
	if walkErr != nil {
		return walkErr
	}
	return err
}

func (s *FileSessionStore) SetSlideHandler(handler core.SlideHandler) {
	s.slideMu.Lock()
	defer s.slideMu.Unlock()
	s.slideHandler = handler
}

func (s *FileSessionStore) Close() error {
	return nil
}

func (s *FileSessionStore) AppendTokenUsage(_ context.Context, sessionID string, usage core.TokenUsage) error {
	usageDir := filepath.Join(s.rootDir, ".usage")
	if err := os.MkdirAll(usageDir, 0755); err != nil {
		return fmt.Errorf("create usage dir: %w", err)
	}
	path := filepath.Join(usageDir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open usage file: %w", err)
	}
	defer f.Close()
	data, err := json.Marshal(&usage)
	if err != nil {
		return fmt.Errorf("marshal usage: %w", err)
	}
	_, err = f.WriteString(string(data) + "\n")
	return err
}

func (s *FileSessionStore) GetTokenUsages(_ context.Context, sessionID string) ([]core.TokenUsage, error) {
	usageDir := filepath.Join(s.rootDir, ".usage")
	path := filepath.Join(usageDir, sessionID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var usages []core.TokenUsage
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var u core.TokenUsage
		if err := json.Unmarshal([]byte(line), &u); err != nil {
			continue
		}
		usages = append(usages, u)
	}
	return usages, nil
}

func (s *FileSessionStore) GetByRole(ctx context.Context, agent string) (*core.SessionInfo, error) {
	var bestInfo *core.SessionInfo

	agentDir := s.agentDir(agent)
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return nil, core.ErrSessionNotFound
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".yml")
		path := filepath.Join(agentDir, entry.Name())
		info, err := statSessionInfo(agent, sessionID, path)
		if err != nil {
			continue
		}

		if bestInfo == nil || info.LastActivityAt.After(bestInfo.LastActivityAt) {
			bestInfo = info
		}
	}

	if bestInfo == nil {
		return nil, core.ErrSessionNotFound
	}

	messages, _ := s.Get(ctx, bestInfo.SessionID)
	bestInfo.Messages = messages
	return bestInfo, nil
}

func (s *FileSessionStore) ListSessions(ctx context.Context) ([]core.SessionInfo, error) {
	var infos []core.SessionInfo

	agentDirs, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, err
	}

	for _, agentEntry := range agentDirs {
		if !agentEntry.IsDir() {
			continue
		}
		agentName := agentEntry.Name()

		sessionFiles, err := os.ReadDir(filepath.Join(s.rootDir, agentName))
		if err != nil {
			continue
		}

		for _, file := range sessionFiles {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".yml") {
				continue
			}

			sessionID := strings.TrimSuffix(file.Name(), ".yml")
			path := s.sessionPath(agentName, sessionID)
			info, err := statSessionInfo(agentName, sessionID, path)
			if err != nil {
				continue
			}
			infos = append(infos, *info)
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].LastActivityAt.After(infos[j].LastActivityAt)
	})
	return infos, nil
}

func (s *FileSessionStore) findSessionByAgent(agentName string) (string, error) {
	agentDir := s.agentDir(agentName)
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return "", core.ErrSessionNotFound
	}

	var bestSession string
	var bestModTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}
		modTime := fileInfo.ModTime()
		if modTime.After(bestModTime) {
			bestModTime = modTime
			bestSession = strings.TrimSuffix(entry.Name(), ".yml")
		}
	}

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
