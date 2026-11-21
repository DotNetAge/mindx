package gateway

import (
	"log/slog"
	"sync"
	"time"
)

type ConnectionState int

const (
	StateDisconnected ConnectionState = 0
	StateConnecting    ConnectionState = 1
	StateConnected     ConnectionState = 2
	StateReconnecting  ConnectionState = 3
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

type HeartbeatConfig struct {
	Interval       time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	PingPeriod     time.Duration
	MaxMissedPings int
}

func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval:       30 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		PingPeriod:     27 * time.Second,
		MaxMissedPings: 3,
	}
}

type HeartbeatStats struct {
	TotalPingsSent     uint64
	TotalPongsReceived uint64
	MissedPings        uint64
	LastPingAt         time.Time
	LastPongAt         time.Time
	ConnectionCount    int
}

type HeartbeatMonitor struct {
	config      *HeartbeatConfig
	stats       HeartbeatStats
	mu          sync.RWMutex
	cbMu        sync.RWMutex
	onStateChange func(clientID, oldState, newState string)
	onTimeout    func(clientID string)
	onReconnect  func(clientID string, attempt int)
}

func newHeartbeatMonitor(cfg *HeartbeatConfig) *HeartbeatMonitor {
	if cfg == nil {
		cfg = DefaultHeartbeatConfig()
	}
	return &HeartbeatMonitor{
		config: cfg,
	}
}

func (h *HeartbeatMonitor) Config() *HeartbeatConfig { return h.config }

func (h *HeartbeatMonitor) Stats() HeartbeatStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stats
}

func (h *HeartbeatMonitor) recordPing() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.TotalPingsSent++
	h.stats.LastPingAt = time.Now()
}

func (h *HeartbeatMonitor) recordPong() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.TotalPongsReceived++
	h.stats.LastPongAt = time.Now()
}

func (h *HeartbeatMonitor) recordMiss() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.MissedPings++
}

func (h *HeartbeatMonitor) setConnectionCount(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.ConnectionCount = n
}

func (h *HeartbeatMonitor) OnStateChange(fn func(clientID, oldState, newState string)) {
	h.cbMu.Lock()
	defer h.cbMu.Unlock()
	h.onStateChange = fn
}

func (h *HeartbeatMonitor) OnTimeout(fn func(clientID string)) {
	h.cbMu.Lock()
	defer h.cbMu.Unlock()
	h.onTimeout = fn
}

func (h *HeartbeatMonitor) OnReconnect(fn func(clientID string, attempt int)) {
	h.cbMu.Lock()
	defer h.cbMu.Unlock()
	h.onReconnect = fn
}

func (h *HeartbeatMonitor) notifyStateChange(clientID, oldState, newState string) {
	h.cbMu.RLock()
	fn := h.onStateChange
	h.cbMu.RUnlock()
	if fn != nil {
		go fn(clientID, oldState, newState)
		slog.Info("client state changed", "id", clientID, "from", oldState, "to", newState)
	}
}

func (h *HeartbeatMonitor) notifyTimeout(clientID string) {
	h.cbMu.RLock()
	fn := h.onTimeout
	h.cbMu.RUnlock()
	if fn != nil {
		go fn(clientID)
		slog.Warn("client heartbeat timeout", "id", clientID)
	}
}

func (h *HeartbeatMonitor) notifyReconnect(clientID string, attempt int) {
	h.cbMu.RLock()
	fn := h.onReconnect
	h.cbMu.RUnlock()
	if fn != nil {
		go fn(clientID, attempt)
		slog.Info("client reconnect attempt", "id", clientID, "attempt", attempt)
	}
}

type ReconnectConfig struct {
	Enabled          bool
	MaxRetries       int
	InitialInterval  time.Duration
	MaxInterval      time.Duration
	Multiplier       float64
	Jitter           float64
	Strategy         string
	OnReconnect      func(attempt int) error
	OnReconnectFail  func(err error)
	OnConnected      func()
}

func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Enabled:          true,
		MaxRetries:       10,
		InitialInterval:  1 * time.Second,
		MaxInterval:      60 * time.Second,
		Multiplier:       2.0,
		Jitter:           0.2,
	}
}

func (r *ReconnectConfig) calcDelay(attempt int) time.Duration {
	delay := r.InitialInterval
	for i := 0; i < attempt && delay < r.MaxInterval; i++ {
		delay = time.Duration(float64(delay) * r.Multiplier)
	}
	if delay > r.MaxInterval {
		delay = r.MaxInterval
	}
	jitter := time.Duration(float64(delay) * r.Jitter)
	delay += jitter
	return delay
}
