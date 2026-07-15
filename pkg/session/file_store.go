package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"gopkg.in/yaml.v3"
)

// encodeMsg base64-encodes content fields for safe YAML storage.
func encodeMsg(msg goharnesssession.Message) goharnesssession.Message {
	encoded := msg
	encoded.Content = base64.StdEncoding.EncodeToString([]byte(msg.Content))
	encoded.ReasoningContent = base64.StdEncoding.EncodeToString([]byte(msg.ReasoningContent))
	if msg.Compacted != "" {
		encoded.Compacted = base64.StdEncoding.EncodeToString([]byte(msg.Compacted))
	}
	return encoded
}

// decodeMsg base64-decodes content fields, falling back to raw string on error.
func decodeMsg(msg goharnesssession.Message) goharnesssession.Message {
	decoded := msg
	if d, err := base64.StdEncoding.DecodeString(msg.Content); err == nil {
		decoded.Content = string(d)
	} else {
		log.Printf("[WARN] session: failed to base64-decode content for role=%q: %v", msg.Role, err)
	}
	if d, err := base64.StdEncoding.DecodeString(msg.ReasoningContent); err == nil && msg.ReasoningContent != "" {
		decoded.ReasoningContent = string(d)
	} else if msg.ReasoningContent != "" {
		log.Printf("[WARN] session: failed to base64-decode reasoning_content for role=%q: %v", msg.Role, err)
	}
	if msg.Compacted != "" {
		if d, err := base64.StdEncoding.DecodeString(msg.Compacted); err == nil {
			decoded.Compacted = string(d)
		} else {
			log.Printf("[WARN] session: failed to base64-decode compacted for role=%q: %v", msg.Role, err)
		}
	}
	return decoded
}

var _ goharnesssession.SessionStore = (*FileSessionStore)(nil)

// GetCursor retrieves the compaction cursor position from the session's metadata.
func (s *FileSessionStore) GetCursor(_ context.Context, sessionID string) (int, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return 0, nil
	}
	info, err := LoadSessionMeta(dirPath)
	if err != nil {
		return 0, nil
	}
	return info.Cursor, nil
}

// SetCursor persists the compaction cursor position to the session's metadata.
func (s *FileSessionStore) SetCursor(_ context.Context, sessionID string, cursor int) error {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return fmt.Errorf("session %q not found", sessionID)
	}
	info, err := LoadSessionMeta(dirPath)
	if err != nil {
		info = &goharnesssession.SessionInfo{
			SessionID: sessionID,
			Cursor:    cursor,
		}
	} else {
		info.Cursor = cursor
	}
	return SaveSessionMeta(dirPath, info)
}

type FileSessionStore struct {
	rootDir        string
	slideMu        sync.RWMutex
	slideHandler   goharnesssession.SlideHandler
	tokenEstimator TokenEstimator
	ioMu           sync.Mutex // 保护所有文件 I/O 操作，防止并发读写导致数据损坏
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
		slideHandler:   goharnesssession.NoopSlideHandler,
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

func (s *FileSessionStore) findSessionDir(sessionID string) string {
	var result string
	_ = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || info.Name() != sessionID {
			return nil
		}
		sessionFile := filepath.Join(path, "meta.json")
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

func (s *FileSessionStore) Append(ctx context.Context, sessionID string, agentName string, sponsor string, msg goharnesssession.Message) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

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

	var msgs []goharnesssession.Message
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &msgs); err != nil {
			return fmt.Errorf("parse existing session file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read session file %s: %w", path, err)
	}

	msgs = append(msgs, encodeMsg(msg))

	data, err := yaml.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write session file %s: %w", path, err)
	}

	// 补录 title：首条 user 消息内容 → session 标题（仅首次，不覆盖）
	if msg.Role == "user" && msg.Content != "" {
		if existingMeta, err := LoadSessionMeta(dir); err == nil && existingMeta.Title == "" {
			title := strings.TrimSpace(msg.Content)
			title = strings.ReplaceAll(title, "\n", " ")
			if len(title) > 80 {
				title = title[:77] + "..."
			}
			existingMeta.Title = title
			if saveErr := SaveSessionMeta(dir, existingMeta); saveErr != nil {
				log.Printf("[WARN] session: failed to save title meta for session dir %s: %v", dir, saveErr)
			}
		}
	}

	s.updateSessionMeta(dir, agentName, sponsor)

	return nil
}

func (s *FileSessionStore) Get(ctx context.Context, sessionID string) ([]goharnesssession.Message, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	var msgs []goharnesssession.Message

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
			log.Printf("[WARN] session: failed to parse session file %s: %v", path, parseErr)
			return nil
		}
		msgs = parsed
		return filepath.SkipAll
	})

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Timestamp < msgs[j].Timestamp })
	return msgs, err
}

func (s *FileSessionStore) CurrentContext(ctx context.Context, agentName string, maxTokens int64) ([]goharnesssession.Message, error) {
	sessionID, err := s.findSessionByAgent(agentName)
	if err != nil {
		return nil, err
	}

	allMsgs, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var result []goharnesssession.Message
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
			handler(ctx, goharnesssession.SlideEvent{
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
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

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

	var filtered []goharnesssession.Message
	for _, m := range msgs {
		if m.Timestamp != timestamp {
			filtered = append(filtered, m)
		}
	}

	return writeMessagesToFile(path, filtered)
}

func (s *FileSessionStore) Clear(ctx context.Context, sessionID string) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	var dirPath string
	_ = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "session.yml" {
			return nil
		}
		parentDir := filepath.Base(filepath.Dir(path))
		if parentDir == sessionID {
			dirPath = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if dirPath == "" {
		return nil
	}
	return os.RemoveAll(dirPath)
}

// DeleteSession removes the entire session directory and all its contents.
// This permanently deletes the session and cannot be undone.
func (s *FileSessionStore) DeleteSession(_ context.Context, sessionID string) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return goharnesssession.ErrSessionNotFound
	}

	if rmErr := os.RemoveAll(dirPath); rmErr != nil {
		log.Printf("[WARN] session: failed to remove session directory %s: %v", dirPath, rmErr)
		return fmt.Errorf("delete session %q: %w", sessionID, rmErr)
	}
	return nil
}

func (s *FileSessionStore) SetSlideHandler(handler goharnesssession.SlideHandler) {
	s.slideMu.Lock()
	defer s.slideMu.Unlock()
	s.slideHandler = handler
}

func (s *FileSessionStore) Close() error {
	return nil
}

func (s *FileSessionStore) ListSessions(ctx context.Context) ([]goharnesssession.SessionInfo, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	var infos []goharnesssession.SessionInfo

	_ = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "meta.json" {
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
			log.Printf("[WARN] session: failed to stat session info for agent=%q id=%q dir=%s: %v", agentName, sessionID, sessionDirPath, statErr)
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
		if err != nil || info.IsDir() || info.Name() != "meta.json" {
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
		return "", goharnesssession.ErrSessionNotFound
	}
	return bestSession, nil
}

func parseMessagesFromFile(path string) ([]goharnesssession.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var msgs []goharnesssession.Message
	if err := yaml.Unmarshal(data, &msgs); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	for i := range msgs {
		msgs[i] = decodeMsg(msgs[i])
	}

	return msgs, nil
}

func writeMessagesToFile(path string, msgs []goharnesssession.Message) error {
	encoded := make([]goharnesssession.Message, len(msgs))
	for i, msg := range msgs {
		encoded[i] = encodeMsg(msg)
	}

	data, err := yaml.Marshal(encoded)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func statSessionInfo(agentName, sessionID, sessionDirPath string) (*goharnesssession.SessionInfo, error) {
	info, err := os.Stat(sessionDirPath)
	if err != nil {
		return nil, err
	}

	si := &goharnesssession.SessionInfo{
		SessionID:      sessionID,
		AgentName:      agentName,
		LastActivityAt: info.ModTime(),
		CreatedAt:      info.ModTime(),
	}

	meta, metaErr := LoadSessionMeta(sessionDirPath)
	if metaErr == nil {
		si.Sponsor = meta.Sponsor
		si.ProjectDir = meta.ProjectDir
		si.Title = meta.Title
		si.CreatedAt = meta.CreatedAt
		si.LastActivityAt = meta.UpdatedAt
		si.UpdatedAt = meta.UpdatedAt
		si.MessageCount = meta.MessageCount
		si.Cursor = meta.Cursor
		if si.LastActivityAt.IsZero() {
			si.LastActivityAt = info.ModTime()
		}
	} else {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			si.ProjectDir = cwd
		}
		defaultMeta := &goharnesssession.SessionInfo{
			SessionID:      sessionID,
			AgentName:      agentName,
			Sponsor:        si.Sponsor,
			ProjectDir:     si.ProjectDir,
			CreatedAt:      info.ModTime(),
			LastActivityAt: info.ModTime(),
		}
		_ = SaveSessionMeta(sessionDirPath, defaultMeta)
	}

	// 加载已追踪的修改文件列表
	if mfData, mfErr := os.ReadFile(filepath.Join(sessionDirPath, "modify_files.yml")); mfErr == nil {
		var files []string
		if yaml.Unmarshal(mfData, &files) == nil {
			si.ModifyFiles = files
		}
	}

	return si, nil
}

// GetSessionMeta loads session metadata for the given session ID.
// It searches all agent directories to find the session.
func (s *FileSessionStore) GetSessionMeta(sessionID string) (*goharnesssession.SessionInfo, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, goharnesssession.ErrSessionNotFound
	}
	return LoadSessionMeta(dirPath)
}

// updateSessionMeta updates the UpdatedAt and LastActivityAt timestamps in meta.json.
// If meta.json does not exist (first append), it creates one with available session info.
// This is called after each message append to keep metadata current.
func (s *FileSessionStore) updateSessionMeta(sessionDir, agentName, sponsor string) {
	info, err := LoadSessionMeta(sessionDir)
	if err != nil {
		// First append — create meta.json with available info
		sessionID := filepath.Base(sessionDir)
		info = &goharnesssession.SessionInfo{
			SessionID:      sessionID,
			AgentName:      agentName,
			Sponsor:        sponsor,
			ProjectDir:     "",
			CreatedAt:      time.Now(),
			LastActivityAt: time.Now(),
		}
	}
	info.UpdatedAt = time.Now()
	info.LastActivityAt = time.Now()
	info.MessageCount++
	if saveErr := SaveSessionMeta(sessionDir, info); saveErr != nil {
		log.Printf("[WARN] session: failed to update session meta for %s: %v", sessionDir, saveErr)
	}
}

// === Session Lifecycle Management (Framework-level directory control) ===

// Create creates a new session with the given agent name and options.
// This centralizes session creation logic:
//  1. Generates a unique session ID (format: sess_<timestamp>)
//  2. Captures ProjectDir from options or os.Getwd()
//  3. Creates session directory structure: <root>/<agent>/<session_id>/tmp
//  4. Persists session metadata (for future GetMeta calls)
//  5. Returns complete SessionInfo with directory context
func (s *FileSessionStore) Create(_ context.Context, agentName string, opts ...goharnesssession.SessionOption) (*goharnesssession.SessionInfo, error) {
	sessionID := generateSessionID()

	sessionInfo := &goharnesssession.SessionInfo{
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

	meta := &goharnesssession.SessionInfo{
		SessionID:      sessionID,
		AgentName:      agentName,
		Sponsor:        sessionInfo.Sponsor,
		ProjectDir:     sessionInfo.ProjectDir,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
		MessageCount:   0,
	}

	if err := SaveSessionMeta(sessionDirPath, meta); err != nil {
		return nil, fmt.Errorf("save session meta: %w", err)
	}

	return sessionInfo, nil
}

// GetMeta returns complete session metadata including directory information.
func (s *FileSessionStore) GetMeta(_ context.Context, sessionID string) (*goharnesssession.SessionInfo, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, goharnesssession.ErrSessionNotFound
	}

	return LoadSessionMeta(dirPath)
}

// ResolveSessionDir returns the filesystem path for the session's sandbox directory.
// This is the canonical way for tools and components to locate session-specific files.
func (s *FileSessionStore) ResolveSessionDir(sessionID string) (string, error) {
	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return "", goharnesssession.ErrSessionNotFound
	}
	return dirPath, nil
}

// modifyFilesPath returns the path to the modify_files.yml for a session.
func (s *FileSessionStore) modifyFilesPath(dirPath string) string {
	return filepath.Join(dirPath, "modify_files.yml")
}

// SaveModifyFiles persists the tracked modified file paths list to disk.
// Files are stored as a YAML string array in the session directory.
func (s *FileSessionStore) SaveModifyFiles(sessionID string, files []string) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return fmt.Errorf("session %q not found", sessionID)
	}

	path := s.modifyFilesPath(dirPath)

	if files == nil {
		// nil 表示清除，删除文件
		if _, err := os.Stat(path); err == nil {
			return os.Remove(path)
		}
		return nil
	}

	data, err := yaml.Marshal(files)
	if err != nil {
		return fmt.Errorf("marshal modify_files: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// GetModifyFiles loads the tracked modified file paths list from disk.
// Returns nil if no file exists (session has no tracked modifications).
func (s *FileSessionStore) GetModifyFiles(sessionID string) ([]string, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil, nil
	}

	path := s.modifyFilesPath(dirPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read modify_files: %w", err)
	}

	var files []string
	if err := yaml.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("parse modify_files: %w", err)
	}

	return files, nil
}

// Truncate removes messages from the session file on disk, keeping only
// the first keepCount messages. Remaining messages are written back to
// the session.yml file in their original order.
func (s *FileSessionStore) Truncate(_ context.Context, sessionID string, keepCount int) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return nil
	}

	path := filepath.Join(dirPath, "session.yml")
	msgs, err := parseMessagesFromFile(path)
	if err != nil {
		return err
	}

	if keepCount >= len(msgs) {
		return nil
	}

	msgs = msgs[:keepCount]
	return writeMessagesToFile(path, msgs)
}

// UpdateMessages replaces all messages in the session file with the given messages.
// This writes the complete message list to session.yml, overwriting any existing content.
// Also persists the compaction cursor position.
func (s *FileSessionStore) UpdateMessages(_ context.Context, sessionID string, cursor int, msgs []goharnesssession.Message) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	dirPath := s.findSessionDir(sessionID)
	if dirPath == "" {
		return goharnesssession.ErrSessionNotFound
	}

	path := filepath.Join(dirPath, "session.yml")
	if err := writeMessagesToFile(path, msgs); err != nil {
		return err
	}

	// Persist cursor alongside messages
	info, err := LoadSessionMeta(dirPath)
	if err != nil {
		info = &goharnesssession.SessionInfo{SessionID: sessionID, Cursor: cursor}
	} else {
		info.Cursor = cursor
	}
	return SaveSessionMeta(dirPath, info)
}
