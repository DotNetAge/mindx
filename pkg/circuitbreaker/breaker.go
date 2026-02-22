package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, reject calls
	StateHalfOpen              // Testing if service recovered
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

// Option configures a CircuitBreaker.
type Option func(*CircuitBreaker)

// WithMaxFailures sets the failure threshold before opening.
func WithMaxFailures(n int) Option {
	return func(cb *CircuitBreaker) { cb.maxFailures = n }
}

// WithResetTimeout sets how long to wait before half-open probe.
func WithResetTimeout(d time.Duration) Option {
	return func(cb *CircuitBreaker) { cb.resetTimeout = d }
}

// CircuitBreaker implements a three-state circuit breaker.
type CircuitBreaker struct {
	name         string
	mu           sync.Mutex
	state        State
	failures     int
	maxFailures  int
	resetTimeout time.Duration
	lastFailure  time.Time
}

// New creates a CircuitBreaker with the given name and options.
// Defaults: 5 max failures, 30s reset timeout.
func New(name string, opts ...Option) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:         name,
		state:        StateClosed,
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

// Execute runs fn through the circuit breaker.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			return cb.doHalfOpen(fn)
		}
		cb.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		cb.mu.Unlock()
		return cb.doHalfOpen(fn)

	default: // StateClosed
		cb.mu.Unlock()
		return cb.doClosed(fn)
	}
}

func (cb *CircuitBreaker) doClosed(fn func() error) error {
	err := fn()
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
		return err
	}

	cb.failures = 0
	return nil
}

func (cb *CircuitBreaker) doHalfOpen(fn func() error) error {
	err := fn()
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.state = StateOpen
		cb.lastFailure = time.Now()
		return err
	}

	cb.state = StateClosed
	cb.failures = 0
	return nil
}

// State returns the current state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Name returns the breaker name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}
