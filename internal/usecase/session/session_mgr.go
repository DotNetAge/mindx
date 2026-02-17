package session

import (
	"encoding/json"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SessionMgr struct {
	sessions       map[string]*entity.Session
	mutex          sync.RWMutex
	currentSession *entity.Session
	maxTokens      int
	checkPoints    map[string]bool
	storage        SessionStorage
	onSessionEnd   OnSessionEndFunc
	logger         logging.Logger
	cleaner        *HistoryCleaner
	splitter       *MessageSplitter
}

type OnSessionEndFunc func(session entity.Session) bool

type SessionStorage interface {
	Save(session entity.Session) error
	Load(id string) (*entity.Session, error)
	LoadAll() ([]entity.Session, error)
	Delete(id string) error
}

type FileSessionStorage struct {
	saveDir string
}

func NewFileSessionStorage(saveDir string) *FileSessionStorage {
	return &FileSessionStorage{
		saveDir: saveDir,
	}
}

func (fs *FileSessionStorage) Save(session entity.Session) error {
	filePath := filepath.Join(fs.saveDir, session.ID+".json")

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.TWithData("session.marshal_failed", map[string]interface{}{"Error": err.Error()}))
	}

	return os.WriteFile(filePath, data, 0644)
}

func (fs *FileSessionStorage) Load(id string) (*entity.Session, error) {
	filePath := filepath.Join(fs.saveDir, id+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var session entity.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (fs *FileSessionStorage) LoadAll() ([]entity.Session, error) {
	entries, err := os.ReadDir(fs.saveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []entity.Session{}, nil
		}
		return nil, err
	}

	sessions := make([]entity.Session, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			id := entry.Name()[:len(entry.Name())-5]
			session, err := fs.Load(id)
			if err == nil && session != nil {
				sessions = append(sessions, *session)
			}
		}
	}

	return sessions, nil
}

func (fs *FileSessionStorage) Delete(id string) error {
	filePath := filepath.Join(fs.saveDir, id+".json")
	return os.Remove(filePath)
}

func SetOnSessionEnd(sm *SessionMgr, onSessionEnd OnSessionEndFunc) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.onSessionEnd = onSessionEnd
}

func NewSessionMgr(
	maxTokens int,
	storage SessionStorage,
	logger logging.Logger,
) *SessionMgr {
	if storage == nil {
		storage = NewFileSessionStorage(filepath.Join("data", "sessions"))
	}

	return &SessionMgr{
		sessions:     make(map[string]*entity.Session),
		maxTokens:    maxTokens,
		checkPoints:  make(map[string]bool),
		storage:      storage,
		onSessionEnd: nil,
		logger:       logger.Named("session_mgr"),
		cleaner:      NewHistoryCleaner(),
		splitter:     NewMessageSplitter(2000),
	}
}

func (sm *SessionMgr) RestoreSession() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	allSessions, err := sm.storage.LoadAll()
	if err != nil {
		return fmt.Errorf(i18n.TWithData("session.load_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var lastActive *entity.Session
	for _, session := range allSessions {
		sessionCopy := session
		sm.sessions[session.ID] = &sessionCopy
		if !session.IsEnded {
			if lastActive == nil || session.CreatedAt.After(lastActive.CreatedAt) {
				lastActive = &sessionCopy
			}
		}
	}

	if lastActive != nil {
		sm.currentSession = lastActive
		sm.logger.Info(i18n.T("session.restore_session"),
			logging.String(i18n.T("session.session_id"), lastActive.ID),
			logging.Int(i18n.T("session.messages"), len(lastActive.Messages)),
			logging.Int(i18n.T("session.tokens"), lastActive.TokensUsed),
		)
	} else {
		return sm.startNewSessionLocked()
	}

	return nil
}

func (sm *SessionMgr) RecordMessage(msg entity.Message) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.currentSession == nil {
		return sm.startNewSessionLocked()
	}

	messages := sm.splitter.Split(msg)

	for _, m := range messages {
		sm.currentSession.Messages = append(sm.currentSession.Messages, m)

		msgTokens := sm.calculateTokens(m.Content)
		sm.currentSession.TokensUsed += msgTokens

		sm.logger.Debug(i18n.T("session.record_message"),
			logging.String(i18n.T("session.role"), m.Role),
			logging.Int(i18n.T("session.tokens"), msgTokens),
			logging.Int(i18n.T("session.total_tokens"), sm.currentSession.TokensUsed),
			logging.Int(i18n.T("session.max_tokens"), sm.maxTokens),
			logging.Int("content_length", len(m.Content)),
		)
	}

	if err := sm.storage.Save(*sm.currentSession); err != nil {
		sm.logger.Error(i18n.T("session.persist_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
		)
	}

	if sm.currentSession.TokensUsed >= sm.maxTokens {
		return sm.endSessionLocked()
	}

	return nil
}

func (sm *SessionMgr) UpdateTokensFromModel(usage core.TokenUsage) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.currentSession != nil {
		sm.currentSession.TokensUsed += usage.TotalTokens
		sm.logger.Debug(i18n.T("session.update_tokens"),
			logging.Int(i18n.T("session.tokens"), usage.TotalTokens),
			logging.Int(i18n.T("session.total_tokens"), sm.currentSession.TokensUsed),
		)

		if err := sm.storage.Save(*sm.currentSession); err != nil {
			sm.logger.Error(i18n.T("session.persist_failed"),
				logging.Err(err),
				logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
			)
		}
	}
}

func (sm *SessionMgr) GetHistory() []entity.Message {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if sm.currentSession == nil {
		return []entity.Message{}
	}

	return sm.cleaner.Clean(sm.currentSession.Messages)
}

func (sm *SessionMgr) GetCurrentSession() (*entity.Session, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if sm.currentSession == nil {
		return nil, false
	}
	return sm.currentSession, true
}

func (sm *SessionMgr) GetAllSessions() ([]entity.Session, error) {
	return sm.storage.LoadAll()
}

func (sm *SessionMgr) endSessionLocked() error {
	if sm.currentSession == nil {
		return nil
	}

	sm.logger.Info(i18n.T("session.session_end_limit"),
		logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
		logging.Int(i18n.T("session.messages"), len(sm.currentSession.Messages)),
		logging.Int(i18n.T("session.tokens"), sm.currentSession.TokensUsed),
	)

	sm.currentSession.IsEnded = true
	sm.currentSession.EndedAt = time.Now()

	var success bool
	if sm.onSessionEnd != nil {
		success = sm.onSessionEnd(*sm.currentSession)
		if success {
			sm.logger.Info(i18n.T("session.memory_extract_success"),
				logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
			)
		} else {
			sm.logger.Warn(i18n.T("session.memory_extract_failed"),
				logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
			)
		}
	} else {
		success = true
	}

	if success {
		sm.checkPoints[sm.currentSession.ID] = true
	}

	if err := sm.storage.Save(*sm.currentSession); err != nil {
		sm.logger.Error(i18n.T("session.persist_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
		)
	}

	return sm.startNewSessionLocked()
}

func (sm *SessionMgr) startNewSessionLocked() error {
	newSession := &entity.Session{
		ID:         sm.generateSessionID(),
		Messages:   []entity.Message{},
		TokensUsed: 0,
		IsEnded:    false,
		CreatedAt:  time.Now(),
	}

	sm.sessions[newSession.ID] = newSession
	sm.currentSession = newSession

	sm.logger.Info(i18n.T("session.start_new_session"),
		logging.String(i18n.T("session.session_id"), newSession.ID),
		logging.Int(i18n.T("session.max_tokens"), sm.maxTokens),
	)

	if err := sm.storage.Save(*newSession); err != nil {
		sm.logger.Error(i18n.T("session.persist_new_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), newSession.ID),
		)
	}

	return nil
}

func (sm *SessionMgr) RecordCheckPoint(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.checkPoints[sessionID] = true
	sm.logger.Debug(i18n.T("session.record_checkpoint"), logging.String(i18n.T("session.session_id"), sessionID))
}

func (sm *SessionMgr) CleanupUnmemorizedSessions() []entity.Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var unmemorized []entity.Session
	for id, session := range sm.sessions {
		if session.IsEnded && !sm.checkPoints[id] {
			unmemorized = append(unmemorized, *session)
		}
	}

	return unmemorized
}

func (sm *SessionMgr) generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

func (sm *SessionMgr) calculateTokens(content string) int {
	return calculateTokens(content)
}

func (sm *SessionMgr) DeleteSession(id string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.currentSession != nil && sm.currentSession.ID == id {
		if err := sm.startNewSessionLocked(); err != nil {
			return err
		}
	}

	delete(sm.sessions, id)
	delete(sm.checkPoints, id)

	if err := sm.storage.Delete(id); err != nil {
		sm.logger.Error(i18n.T("session.delete_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), id),
		)
		return err
	}

	sm.logger.Info(i18n.T("session.delete_success"),
		logging.String(i18n.T("session.session_id"), id),
	)

	return nil
}

func (sm *SessionMgr) SwitchSession(id string) (*entity.Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, err := sm.storage.Load(id)
	if err != nil {
		sm.logger.Error(i18n.T("session.switch_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), id),
		)
		return nil, fmt.Errorf(i18n.TWithData("session.switch_failed", map[string]interface{}{"Error": err.Error()}))
	}

	session.IsEnded = false

	sm.sessions[session.ID] = session
	sm.currentSession = session

	if err := sm.storage.Save(*session); err != nil {
		sm.logger.Error(i18n.T("session.persist_failed"),
			logging.Err(err),
			logging.String(i18n.T("session.session_id"), session.ID),
		)
	}

	sm.logger.Info(i18n.T("session.switch_success"),
		logging.String(i18n.T("session.session_id"), session.ID),
		logging.Int(i18n.T("session.messages"), len(session.Messages)),
	)

	return session, nil
}

func (sm *SessionMgr) CreateNewSession() (*entity.Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.currentSession != nil && len(sm.currentSession.Messages) > 0 {
		sm.currentSession.IsEnded = true
		sm.currentSession.EndedAt = time.Now()
		if err := sm.storage.Save(*sm.currentSession); err != nil {
			sm.logger.Error(i18n.T("session.persist_failed"),
				logging.Err(err),
				logging.String(i18n.T("session.session_id"), sm.currentSession.ID),
			)
		}
	}

	if err := sm.startNewSessionLocked(); err != nil {
		return nil, err
	}

	return sm.currentSession, nil
}
