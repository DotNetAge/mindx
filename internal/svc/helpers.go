package svc

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func generateSessionID() string {
	return fmt.Sprintf("sess_%s", uuid.New().String()[:8])
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(100 * time.Millisecond).String()
}

func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}
