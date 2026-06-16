package core

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/mindx/internal/client/data"
)

// ChangeJournal provides file-based IPC for file change events.
// Daemon writes NDJSON lines to a shared file; TUI reads from it.
type ChangeJournal struct {
	path string
	mu   sync.Mutex
}

// NewChangeJournal creates a journal at <projectDir>/.mindx/changes.ndjson.
func NewChangeJournal(projectDir string) *ChangeJournal {
	return &ChangeJournal{
		path: filepath.Join(projectDir, ".mindx", "changes.ndjson"),
	}
}

// Append writes one or more file change events to the journal.
func (j *ChangeJournal) Append(changes ...data.FileChange) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Ensure parent directory exists
	dir := filepath.Dir(j.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, c := range changes {
		if err := enc.Encode(c); err != nil {
			return err
		}
	}
	return nil
}

// ReadAll returns all recorded changes.
func (j *ChangeJournal) ReadAll() ([]data.FileChange, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	f, err := os.Open(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var changes []data.FileChange
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var fc data.FileChange
		if err := json.Unmarshal(scanner.Bytes(), &fc); err != nil {
			continue // skip malformed lines
		}
		changes = append(changes, fc)
	}
	return changes, scanner.Err()
}

// Clear empties the journal.
func (j *ChangeJournal) Clear() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return os.Remove(j.path)
}
