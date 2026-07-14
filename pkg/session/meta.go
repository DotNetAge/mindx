package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/oklog/ulid/v2"
)

// ulidEntropy is a shared entropy source for ULID generation (monotonic safe).
var ulidEntropy = ulid.Monotonic(rand.Reader, 0)

// generateSessionID generates a unique session identifier using ULID.
// ULID provides: sortability by time + cryptographic randomness + collision resistance.
func generateSessionID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy).String()
}

// LoadSessionMeta loads metadata from meta.json in the given session directory.
// Returns a SessionInfo with all persisted fields.
// Handles backward compat: reads both "project_dir" and "project_working_dir" from JSON.
func LoadSessionMeta(sessionDirPath string) (*goharnesssession.SessionInfo, error) {
	metaPath := filepath.Join(sessionDirPath, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read session meta: %w", err)
	}

	// Unmarshal into raw map first for backward compat migration.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal session meta: %w", err)
	}

	// Migrate project_working_dir → project_dir (backward compat with old meta.json).
	if _, exists := raw["project_dir"]; !exists {
		if v, ok := raw["project_working_dir"]; ok {
			raw["project_dir"] = v
		}
	}

	// Remove deprecated fields that no longer exist on SessionInfo.
	delete(raw, "project_working_dir")
	delete(raw, "home_dir")

	// Re-marshal cleaned map into SessionInfo.
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal session meta: %w", err)
	}

	var info goharnesssession.SessionInfo
	if err := json.Unmarshal(cleaned, &info); err != nil {
		return nil, fmt.Errorf("unmarshal session info: %w", err)
	}

	return &info, nil
}

// SaveSessionMeta persists session metadata to meta.json in the given session directory.
func SaveSessionMeta(sessionDirPath string, info *goharnesssession.SessionInfo) error {
	info.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session meta: %w", err)
	}

	metaPath := filepath.Join(sessionDirPath, "meta.json")
	if err := os.MkdirAll(sessionDirPath, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	return os.WriteFile(metaPath, data, 0600)
}
