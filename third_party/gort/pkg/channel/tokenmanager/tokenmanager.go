// Package tokenmanager provides a generic token management solution for channel adapters.
package tokenmanager

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Token represents an access token with expiration time.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// IsExpired returns true if the token is expired.
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// RefreshFunc is a function that refreshes the token.
type RefreshFunc func(ctx context.Context) (string, time.Duration, error)

// Manager manages token refresh lifecycle.
type Manager struct {
	mu         sync.RWMutex
	token      *Token
	refreshFn  RefreshFunc
	refreshInterval time.Duration
	stopCh     chan struct{}
	stopped    bool
}

// Config contains configuration for the token manager.
type Config struct {
	RefreshFunc     RefreshFunc
	RefreshInterval time.Duration
	InitialToken    string
	InitialExpiry   time.Time
}

// NewManager creates a new token manager.
func NewManager(config Config) *Manager {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 90 * time.Minute
	}

	var token *Token
	if config.InitialToken != "" {
		token = &Token{
			Value:     config.InitialToken,
			ExpiresAt: config.InitialExpiry,
		}
	}

	return &Manager{
		token:           token,
		refreshFn:       config.RefreshFunc,
		refreshInterval: config.RefreshInterval,
		stopCh:          make(chan struct{}),
	}
}

// GetToken returns the current token value.
func (m *Manager) GetToken() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.token == nil || m.token.IsExpired() {
		return "", ErrTokenExpired
	}

	return m.token.Value, nil
}

// Refresh manually refreshes the token.
func (m *Manager) Refresh(ctx context.Context) error {
	if m.refreshFn == nil {
		return ErrNoRefreshFunc
	}

	tokenValue, expiry, err := m.refreshFn(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.token = &Token{
		Value:     tokenValue,
		ExpiresAt: time.Now().Add(expiry),
	}
	m.mu.Unlock()

	return nil
}

// Start starts the automatic token refresh loop.
func (m *Manager) Start(ctx context.Context) error {
	// Initial refresh if no token
	if _, err := m.GetToken(); err != nil {
		if err := m.Refresh(ctx); err != nil {
			return err
		}
	}

	go m.refreshLoop(ctx)
	return nil
}

// Stop stops the automatic token refresh loop.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.stopped {
		close(m.stopCh)
		m.stopped = true
	}
	m.mu.Unlock()
}

func (m *Manager) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = m.Refresh(refreshCtx)
			cancel()
		}
	}
}

var (
	ErrTokenExpired  = errors.New("token expired or not available")
	ErrNoRefreshFunc = errors.New("no refresh function configured")
)
