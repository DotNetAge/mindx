package gateway

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type session struct {
	id           string
	clientID     string
	messages     []*Message
	pendingFrames map[int][]byte
	frameTotal   int
	createdAt    time.Time
}

type SessionManager struct {
	mu           sync.RWMutex
	sessions     map[string]*session
	timeout      time.Duration
	done         chan struct{}
	once         sync.Once
	maxSessions  int
}

const defaultMaxSessions = 1000

func newSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions:     make(map[string]*session),
		timeout:      timeout,
		done:         make(chan struct{}),
		maxSessions:  defaultMaxSessions,
	}
	go sm.cleanupLoop()
	return sm
}

func (sm *SessionManager) create(clientID string) (*session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.sessions) >= sm.maxSessions {
		return nil, ErrSessionLimitReached
	}
	id := uuid.New().String()
	s := &session{
		id:            id,
		clientID:      clientID,
		messages:      make([]*Message, 0),
		pendingFrames: make(map[int][]byte),
		createdAt:     time.Now(),
	}
	sm.sessions[id] = s
	return s, nil
}

func (sm *SessionManager) createWithID(clientID, id string) (*session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, ErrSessionNotFound
	}

	if len(sm.sessions) >= sm.maxSessions {
		return nil, ErrSessionLimitReached
	}

	s := &session{
		id:            id,
		clientID:      clientID,
		messages:      make([]*Message, 0),
		pendingFrames: make(map[int][]byte),
		createdAt:     time.Now(),
	}
	sm.sessions[id] = s
	return s, nil
}

func (sm *SessionManager) get(id string) (*session, bool) {
	sm.mu.RLock()
	s, ok := sm.sessions[id]
	sm.mu.RUnlock()
	return s, ok
}

func (sm *SessionManager) addMessage(id string, msg *Message) error {
	sm.mu.Lock()
	s, ok := sm.sessions[id]
	if !ok {
		sm.mu.Unlock()
		return ErrSessionNotFound
	}
	s.messages = append(s.messages, msg)
	sm.mu.Unlock()
	return nil
}

func (sm *SessionManager) addFrame(id string, index, total int, data []byte) error {
	sm.mu.Lock()
	s, ok := sm.sessions[id]
	if !ok {
		sm.mu.Unlock()
		return ErrSessionNotFound
	}
	s.pendingFrames[index] = data
	s.frameTotal = total
	sm.mu.Unlock()
	return nil
}

func (sm *SessionManager) assembleAndRemove(id string) (*session, bool) {
	sm.mu.Lock()
	s, ok := sm.sessions[id]
	if ok {
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()
	if !ok {
		return nil, false
	}

	complete := false
	if s.frameTotal > 0 && len(s.pendingFrames) == s.frameTotal {
		var assembled []byte
		for i := 0; i < s.frameTotal; i++ {
			if data, ok := s.pendingFrames[i]; ok {
				assembled = append(assembled, data...)
			} else {
				slog.Warn("incomplete frame session", "id", id, "expected", s.frameTotal, "got", len(s.pendingFrames))
				return s, false
			}
		}
		msg := &Message{
			ID:        uuid.New().String(),
			SessionID: id,
			ClientID:  s.clientID,
			Direction: DirectionOutbound,
			Data:      assembled,
			Timestamp: time.Now(),
		}
		s.messages = append(s.messages, msg)
		complete = true
	}
	return s, complete
}

func (sm *SessionManager) remove(id string) {
	sm.mu.Lock()
	delete(sm.sessions, id)
	sm.mu.Unlock()
}

func (sm *SessionManager) cleanupClient(clientID string) {
	var ids []string
	sm.mu.RLock()
	for id, s := range sm.sessions {
		if s.clientID == clientID {
			ids = append(ids, id)
		}
	}
	sm.mu.RUnlock()
	for _, id := range ids {
		sm.remove(id)
	}
}

func (sm *SessionManager) Close() { sm.once.Do(func() { close(sm.done) }) }

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var expired []string
			sm.mu.RLock()
			for id, s := range sm.sessions {
				if now.Sub(s.createdAt) > sm.timeout {
					expired = append(expired, id)
				}
			}
			sm.mu.RUnlock()
			for _, id := range expired {
				sm.remove(id)
				slog.Warn("session expired", "id", id)
			}
		case <-sm.done:
			return
		}
	}
}