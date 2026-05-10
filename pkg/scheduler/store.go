package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ScheduleEntry struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Content   string    `json:"content"`
	CronExpr  string    `json:"cron_expr"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastRunAt time.Time `json:"last_run_at,omitempty"`
	LastRunID string    `json:"last_run_id,omitempty"`
	LastStatus string   `json:"last_status,omitempty"`
	LastError  string   `json:"last_error,omitempty"`
	SuccessCnt int      `json:"success_count"`
	FailureCnt int      `json:"failure_count"`
}

type FileSchedulerStore struct {
	dataDir string
	mu      sync.RWMutex
}

func NewFileSchedulerStore(dataDir string) (*FileSchedulerStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create scheduler data dir: %w", err)
	}
	return &FileSchedulerStore{dataDir: dataDir}, nil
}

func (s *FileSchedulerStore) filePath(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

func (s *FileSchedulerStore) Save(ctx context.Context, entry *ScheduleEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("schedule entry ID is required")
	}

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedule entry: %w", err)
	}

	path := s.filePath(entry.ID)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write schedule entry: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename schedule entry: %w", err)
	}
	return nil
}

func (s *FileSchedulerStore) Load(ctx context.Context, id string) (*ScheduleEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schedule entry %q not found: %w", id, err)
		}
		return nil, fmt.Errorf("failed to read schedule entry: %w", err)
	}

	entry, err := unmarshalEntry(data)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *FileSchedulerStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete schedule entry: %w", err)
	}
	return nil
}

func (s *FileSchedulerStore) List(ctx context.Context) ([]ScheduleEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := filepath.Glob(filepath.Join(s.dataDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list schedule entries: %w", err)
	}

	var result []ScheduleEntry
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entry, err := unmarshalEntry(data)
		if err != nil {
			continue
		}
		result = append(result, *entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (s *FileSchedulerStore) UpdateLastRun(id string, runID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("failed to read entry for last run update: %w", readErr)
	}

	var entry ScheduleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	entry.LastRunAt = time.Now()
	entry.LastRunID = runID

	if err != nil {
		entry.LastStatus = "failed"
		entry.LastError = err.Error()
		entry.FailureCnt++
	} else {
		entry.LastStatus = "success"
		entry.LastError = ""
		entry.SuccessCnt++
	}
	entry.UpdatedAt = time.Now()

	updated, marshalErr := json.MarshalIndent(&entry, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal updated entry: %w", marshalErr)
	}

	tmpPath := path + ".tmp"
	if writeErr := os.WriteFile(tmpPath, updated, 0600); writeErr != nil {
		return fmt.Errorf("failed to write updated entry: %w", writeErr)
	}
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename updated entry: %w", renameErr)
	}
	return nil
}

type legacyEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr"`
	Command  string `json:"command"`
	Args     string `json:"args,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Enabled  bool   `json:"enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	LastRunID  string    `json:"last_run_id,omitempty"`
	LastStatus string    `json:"last_status,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	SuccessCnt int       `json:"success_count"`
	FailureCnt int       `json:"failure_count"`
}

func unmarshalEntry(data []byte) (*ScheduleEntry, error) {
	var entry ScheduleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schedule entry: %w", err)
	}

	if entry.Content == "" {
		var legacy legacyEntry
		if err := json.Unmarshal(data, &legacy); err == nil && legacy.Command != "" {
			entry.Content = legacy.Command
			if entry.Agent == "" && legacy.Agent != "" {
				entry.Agent = legacy.Agent
			}
			if !entry.Enabled && legacy.Enabled {
				entry.Enabled = legacy.Enabled
			}
		}
	}

	return &entry, nil
}
