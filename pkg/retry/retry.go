package retry

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// Config defines retry behavior.
type Config struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Retryable   func(error) bool
}

// DefaultConfig returns a sensible default for LLM calls:
// 3 retries, 1s → 2s → 4s exponential backoff.
func DefaultConfig() Config {
	return Config{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     4 * time.Second,
		Retryable:   DefaultRetryable,
	}
}

// DefaultRetryable returns true for network errors and 5xx-like errors.
// Returns false for context cancellation and 4xx-like errors.
func DefaultRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Never retry on context cancellation/deadline
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check error message for HTTP status indicators
	msg := err.Error()
	// 5xx errors are retryable
	if strings.Contains(msg, "status code: 5") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "529") {
		return true
	}

	// 4xx errors are NOT retryable
	if strings.Contains(msg, "status code: 4") {
		return false
	}

	// Connection-related errors are retryable
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "timeout") {
		return true
	}

	return false
}

// Do executes fn with retry logic according to cfg.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error
	wait := cfg.InitialWait

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt == cfg.MaxRetries {
			break
		}

		if cfg.Retryable != nil && !cfg.Retryable(lastErr) {
			return lastErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		wait *= 2
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}

	return lastErr
}

// DoWithResult executes fn with retry logic, returning both a result and error.
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error
	wait := cfg.InitialWait

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		if attempt == cfg.MaxRetries {
			break
		}

		if cfg.Retryable != nil && !cfg.Retryable(lastErr) {
			return result, lastErr
		}

		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(wait):
		}

		wait *= 2
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}

	return result, lastErr
}
