package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatchEntry represents a single directory being monitored for file changes.
// The TUI adds entries when a user opens a project directory.
type WatchEntry struct {
	Dir     string `json:"dir"`      // absolute path to the monitored directory
	Agent   string `json:"agent"`    // agent name this directory is associated with
	AddedAt int64  `json:"added_at"` // unix timestamp
}

// WatchListStore persists the list of directories to monitor.
// It is shared between TUI (add/remove) and Daemon (read/monitor).
//
// Storage: one JSON file at ~/.mindx/data/watchlist.json
type WatchListStore struct {
	path    string
	entries []WatchEntry
	mu      sync.RWMutex
}

// NewWatchListStore creates or loads a WatchListStore from the given directory.
// dir is typically app.Settings().DataDir() which is ~/.mindx/data/.
func NewWatchListStore(dir string) (*WatchListStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("watchlist: create dir: %w", err)
	}
	s := &WatchListStore{
		path: filepath.Join(dir, "watchlist.json"),
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("watchlist: load: %w", err)
	}
	return s, nil
}

// List returns a copy of all watch entries.
func (s *WatchListStore) List() []WatchEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]WatchEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

// Add appends a new directory to the watch list and persists.
// If the directory is already watched (same path+agent), it's a no-op.
func (s *WatchListStore) Add(dir, agent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("watchlist: resolve path: %w", err)
	}

	// Dedup by dir+agent
	for _, e := range s.entries {
		if e.Dir == absDir && e.Agent == agent {
			return nil // already exists
		}
	}

	s.entries = append(s.entries, WatchEntry{
		Dir:     absDir,
		Agent:   agent,
		AddedAt: time.Now().Unix(),
	})
	return s.save()
}

// Remove deletes a watch entry by dir+agent and persists.
func (s *WatchListStore) Remove(dir, agent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("watchlist: resolve path: %w", err)
	}

	filtered := s.entries[:0]
	for _, e := range s.entries {
		if e.Dir == absDir && e.Agent == agent {
			continue
		}
		filtered = append(filtered, e)
	}
	if len(filtered) == len(s.entries) {
		return nil // nothing removed
	}
	s.entries = filtered
	return s.save()
}

// RemoveByDir removes all entries for a directory and persists.
func (s *WatchListStore) RemoveByDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("watchlist: resolve path: %w", err)
	}

	filtered := s.entries[:0]
	for _, e := range s.entries {
		if e.Dir == absDir {
			continue
		}
		filtered = append(filtered, e)
	}
	if len(filtered) == len(s.entries) {
		return nil
	}
	s.entries = filtered
	return s.save()
}

// load reads entries from disk.
func (s *WatchListStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.entries)
}

// save atomically writes entries to disk.
func (s *WatchListStore) save() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
