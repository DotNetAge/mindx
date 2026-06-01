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

	goreactsession "github.com/DotNetAge/goreact/session"
	"gopkg.in/yaml.v3"
)

type yamlMessage struct {
	Role       string         `yaml:"role"`
	Content    string         `yaml:"content"`
	Timestamp  int64          `yaml:"timestamp"`
	ToolCallID string         `yaml:"tool_call_id,omitempty"`
	ToolCalls  []yamlToolCall `yaml:"tool_calls,omitempty"`
}

// GetCursor retrieves the compaction cursor position from the session's metadata.
// Returns 0 if no cursor has been set (meaning no compaction has occurred).
//
// This method is used internally by Session.ensureLoaded() to restore compaction state
// across Session object lifecycles. External code should never call this directly.
func (s *FileSessionStore) GetCursor(_ context.Context, sessionID string) (int, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return 0, nil // New session, no cursor yet
	}

	meta, err := LoadSessionMeta(dirPath)
	if err != nil {
		return 0, nil // No meta file yet, return default
	}

	return meta.Cursor, nil
}

// SetCursor persists the compaction cursor position to the session's metadata.
// This is called after each compaction event to ensure the cursor survives
// across Session object lifecycles (critical for lazy-loading).
//
// This method is used internally by Session.tryCompact(). External code should
// never call this directly - use session.Compact() instead.
func (s *FileSessionStore) SetCursor(_ context.Context, sessionID string, cursor int) error {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return fmt.Errorf("session %q not found", sessionID)
	}

	meta, err := LoadSessionMeta(dirPath)
	if err != nil {
		// Meta file doesn't exist yet - this can happen if no session meta was created
		// Create a minimal meta with just the cursor
		meta = &SessionMeta{
			SessionID: sessionID,
			Cursor:   cursor,
		}
	} else {
		meta.Cursor = cursor
	}

	return meta.Save(dirPath)
}

type yamlToolCall struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Arguments string `yaml:"arguments"`
}

func newYamlMessage(msg goreactsession.Message) yamlMessage {
	var ymlTCs []yamlToolCall
	for _, tc := range msg.ToolCalls {
		ymlTCs = append(ymlTCs, yamlToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		})
	}
	return yamlMessage{
		Role:       msg.Role,
		Content:    base64.StdEncoding.EncodeToString([]byte(msg.Content)),
		Timestamp:  msg.Timestamp,
		ToolCallID: msg.ToolCallID,
		ToolCalls:  ymlTCs,
	}
}

func (ym yamlMessage) toCoreMessage() goreactsession.Message {
	decoded, _ := base64.StdEncoding.DecodeString(ym.Content)
	var tcs []goreactsession.ToolCall
	for _, ytc := range ym.ToolCalls {
		tcs = append(tcs, goreactsession.ToolCall{
			ID:        ytc.ID,
			Name:      ytc.Name,
			Arguments: ytc.Arguments,
		})
	}
	return goreactsession.Message{
		Role:       ym.Role,
		Content:    string(decoded),
		Timestamp:  ym.Timestamp,
		ToolCallID: ym.ToolCallID,
		ToolCalls:  tcs,
	}
}

var _ goreactsession.SessionStore = (*FileSessionStore)(nil)

type FileSessionStore struct {
	rootDir        string
	slideMu        sync.RWMutex
	slideHandler   goreactsession.SlideHandler
	tokenEstimator TokenEstimator
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
		slideHandler:   goreactsession.NoopSlideHandler,
		tokenEstimator: NewTokenEstimator(),
	}, nil
}

func (s *FileSessionStore) SetTokenEstimator(est TokenEstimator) {
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

func (s *FileSessionStore) Append(ctx context.Context, sessionID string, agentName string, msg goreactsession.Message) error {
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

func (s *FileSessionStore) Get(ctx context.Context, sessionID string) ([]goreactsession.Message, error) {
	var msgs []goreactsession.Message

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

func (s *FileSessionStore) CurrentContext(ctx context.Context, agentName string, maxTokens int64) ([]goreactsession.Message, error) {
	sessionID, err := s.findSessionByAgent(agentName)
	if err != nil {
		return nil, err
	}

	allMsgs, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var result []goreactsession.Message
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
		if handler != nil {
			handler(ctx, goreactsession.SlideEvent{
				SessionID: sessionID,
				Slided:    evicted,
				Remaining: len(result),
				Timestamp: time.Now().UnixMilli(),
			})
		}
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

	var filtered []goreactsession.Message
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

// DeleteSession removes the entire session directory and all its contents.
// This permanently deletes the session and cannot be undone.
func (s *FileSessionStore) DeleteSession(_ context.Context, sessionID string) error {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return goreactsession.ErrSessionNotFound
	}

	_ = os.RemoveAll(dirPath)
	return nil
}

func (s *FileSessionStore) SetSlideHandler(handler goreactsession.SlideHandler) {
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

func toYamlUsage(u goreactsession.TokenUsage) yamlUsage {
	return yamlUsage{
		Timestamp:    u.Timestamp,
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		RemainTokens: u.RemainTokens,
	}
}

func fromYamlUsage(yu yamlUsage) goreactsession.TokenUsage {
	return goreactsession.TokenUsage{
		Timestamp:    yu.Timestamp,
		InputTokens:  yu.InputTokens,
		OutputTokens: yu.OutputTokens,
		RemainTokens: yu.RemainTokens,
	}
}

func (s *FileSessionStore) AppendTokenUsage(_ context.Context, sessionID string, usage goreactsession.TokenUsage) error {
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

func (s *FileSessionStore) GetTokenUsages(_ context.Context, sessionID string) ([]goreactsession.TokenUsage, error) {
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

	usages := make([]goreactsession.TokenUsage, len(yamlUsages))
	for i, yu := range yamlUsages {
		usages[i] = fromYamlUsage(yu)
	}

	return usages, nil
}

func (s *FileSessionStore) GetByRole(ctx context.Context, agent string) (*goreactsession.SessionInfo, error) {
	var bestInfo *goreactsession.SessionInfo

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
		return nil, goreactsession.ErrSessionNotFound
	}

	messages, _ := s.Get(ctx, bestInfo.SessionID)
	bestInfo.Messages = messages
	return bestInfo, nil
}

func (s *FileSessionStore) ListSessions(ctx context.Context) ([]goreactsession.SessionInfo, error) {
	var infos []goreactsession.SessionInfo

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
		sessionDirPath := filepath.Join(s.rootDir, agentName, sessionID)

		si, statErr := statSessionInfo(agentName, sessionID, sessionDirPath)
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
		return "", goreactsession.ErrSessionNotFound
	}
	return bestSession, nil
}

func parseMessagesFromFile(path string) ([]goreactsession.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ymlMsgs []yamlMessage
	if err := yaml.Unmarshal(data, &ymlMsgs); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	msgs := make([]goreactsession.Message, len(ymlMsgs))
	for i, ym := range ymlMsgs {
		msgs[i] = ym.toCoreMessage()
	}

	return msgs, nil
}

func writeMessagesToFile(path string, msgs []goreactsession.Message) error {
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

func statSessionInfo(agentName, sessionID, sessionDirPath string) (*goreactsession.SessionInfo, error) {
	info, err := os.Stat(sessionDirPath)
	if err != nil {
		return nil, err
	}

	si := &goreactsession.SessionInfo{
		SessionID:      sessionID,
		AgentName:      agentName,
		LastActivityAt: info.ModTime(),
		CreatedAt:      info.ModTime(),
	}

	meta, metaErr := LoadSessionMeta(sessionDirPath)
	if metaErr == nil {
		si.ProjectDir = meta.ProjectWorkingDir
		si.CreatedAt = meta.CreatedAt
		si.LastActivityAt = meta.UpdatedAt
		if si.LastActivityAt.IsZero() {
			si.LastActivityAt = info.ModTime()
		}
	} else {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			si.ProjectDir = cwd
		}
		defaultMeta := &SessionMeta{
			SessionID:         sessionID,
			AgentName:         agentName,
			ProjectWorkingDir: si.ProjectDir,
			CreatedAt:         info.ModTime(),
			LastActivityAt:    info.ModTime(),
		}
		_ = defaultMeta.Save(sessionDirPath)
	}

	return si, nil
}

// GetSessionMeta loads session metadata for the given session ID.
// It searches all agent directories to find the session.
func (s *FileSessionStore) GetSessionMeta(sessionID string) (*SessionMeta, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, goreactsession.ErrSessionNotFound
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

// === Session Lifecycle Management (Framework-level directory control) ===

// Create creates a new session with the given agent name and options.
// This centralizes session creation logic:
//  1. Generates a unique session ID (format: sess_<timestamp>)
//  2. Captures ProjectDir from options or os.Getwd()
//  3. Creates session directory structure: <root>/<agent>/<session_id>/tmp
//  4. Persists session metadata (for future GetMeta calls)
//  5. Returns complete SessionInfo with directory context
func (s *FileSessionStore) Create(_ context.Context, agentName string, opts ...goreactsession.SessionOption) (*goreactsession.SessionInfo, error) {
	sessionID := generateSessionID()

	sessionInfo := &goreactsession.SessionInfo{
		SessionID:      sessionID,
		AgentName:      agentName,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	for _, opt := range opts {
		opt(sessionInfo)
	}

	if sessionInfo.ProjectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		sessionInfo.ProjectDir = cwd
	}

	sessionDirPath := s.sessionDir(agentName, sessionID)
	tmpDir := filepath.Join(sessionDirPath, "tmp")

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create session directory %s: %w", sessionDirPath, err)
	}

	sessionInfo.SessionDir = sessionDirPath

	meta := &SessionMeta{
		SessionID:         sessionID,
		AgentName:         agentName,
		ProjectWorkingDir: sessionInfo.ProjectDir,
		CreatedAt:         time.Now(),
		LastActivityAt:    time.Now(),
		MessageCount:      0,
	}

	if err := meta.Save(sessionDirPath); err != nil {
		return nil, fmt.Errorf("save session meta: %w", err)
	}

	return sessionInfo, nil
}

// GetMeta returns complete session metadata including directory information.
// It loads both GoReact SessionInfo and extended metadata from disk.
func (s *FileSessionStore) GetMeta(_ context.Context, sessionID string) (*goreactsession.SessionInfo, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, goreactsession.ErrSessionNotFound
	}

	meta, err := LoadSessionMeta(dirPath)
	if err != nil {
		return nil, fmt.Errorf("load session meta: %w", err)
	}

	return &goreactsession.SessionInfo{
		SessionID:      sessionID,
		AgentName:      meta.AgentName,
		ProjectDir:     meta.ProjectWorkingDir,
		SessionDir:     dirPath,
		LastActivityAt: meta.LastActivityAt,
		CreatedAt:      meta.CreatedAt,
	}, nil
}

// ResolveSessionDir returns the filesystem path for the session's sandbox directory.
// This is the canonical way for tools and components to locate session-specific files.
func (s *FileSessionStore) ResolveSessionDir(sessionID string) (string, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return "", goreactsession.ErrSessionNotFound
	}
	return dirPath, nil
}
