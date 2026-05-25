package svc

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
)

var ulidEntropy = ulid.Monotonic(cryptorand.Reader, 0)

func generateSessionID() string {
	uid := ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy)
	return uid.String()
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

// unmarshalParams unmarshals JSON params into the given target and returns a
// formatted error on failure. Accepts nil params (no-op).
func unmarshalParams(params json.RawMessage, target any) error {
	if params == nil {
		return nil
	}
	if err := json.Unmarshal(params, target); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}
