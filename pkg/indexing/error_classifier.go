package indexing

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// classifyError categorises an indexing error into a high-level type string
// for structured logging and frontend display.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	errMsg := err.Error()
	// Network errors (connection reset, DNS failure, TLS error, dial timeout, read timeout)
	if strings.Contains(errMsg, "network_error") ||
		strings.Contains(errMsg, "read tcp") ||
		strings.Contains(errMsg, "dial tcp") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "tls") ||
		strings.Contains(errMsg, "handshake") {
		return "network"
	}
	// Rate limit / quota errors
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "rate_limit") ||
		strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "quota") {
		return "rate_limit"
	}
	// Auth errors
	if strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "401") ||
		strings.Contains(errMsg, "403") ||
		strings.Contains(errMsg, "api key") ||
		strings.Contains(errMsg, "invalid key") {
		return "auth"
	}
	// File-related errors
	if strings.Contains(errMsg, "no such file") ||
		strings.Contains(errMsg, "permission denied") ||
		strings.Contains(errMsg, "is a directory") ||
		strings.Contains(errMsg, "file") {
		return "file_error"
	}
	return "unknown"
}

// classifyErrorString parses an already-formatted error string (e.g. from
// FileIndexError.Error which contains "[type] message") and returns the
// canonical error type.
func classifyErrorString(s string) string {
	// Format: "path: [type] message"
	if idx := strings.Index(s, "["); idx >= 0 {
		if end := strings.Index(s[idx:], "]"); end >= 0 {
			return s[idx+1 : idx+end]
		}
	}
	return classifyError(fmt.Errorf("%s", s))
}
