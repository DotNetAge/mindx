package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DirIndexState tracks the full-scan state of a watched directory.
type DirIndexState struct {
	Dir          string `json:"dir"`
	State        string `json:"state"`         // pending | indexing | completed | failed
	TotalFiles   int    `json:"total_files"`   // total files discovered during walk
	IndexedFiles int    `json:"indexed_files"` // files processed so far
	Error        string `json:"error,omitempty"`
	StartedAt    int64  `json:"started_at"`
	CompletedAt  int64  `json:"completed_at,omitempty"`
}

// IndexStateStore persists per-directory index states to disk.
//
// Storage: ~/.mindx/data/index_state.json
type IndexStateStore struct {
	path   string
	states map[string]*DirIndexState // key = absolute dir path
	mu     sync.RWMutex
}

// NewIndexStateStore creates or loads an IndexStateStore from the given directory.
func NewIndexStateStore(dir string) (*IndexStateStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("index-state: create dir: %w", err)
	}
	s := &IndexStateStore{
		path:   filepath.Join(dir, "index_state.json"),
		states: make(map[string]*DirIndexState),
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("index-state: load: %w", err)
	}
	return s, nil
}

// Get returns the state for a directory, or nil if not tracked.
func (s *IndexStateStore) Get(dir string) *DirIndexState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[dir]
}

// SetPending creates or resets a directory's state to pending.
func (s *IndexStateStore) SetPending(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[dir] = &DirIndexState{
		Dir:   dir,
		State: "pending",
	}
	_ = s.save()
}

// SetIndexing marks a directory as being indexed, with total file count.
func (s *IndexStateStore) SetIndexing(dir string, totalFiles int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[dir] = &DirIndexState{
		Dir:          dir,
		State:        "indexing",
		TotalFiles:   totalFiles,
		IndexedFiles: 0,
		StartedAt:    time.Now().Unix(),
	}
	_ = s.save()
}

// SetCompletedWithStats marks a directory's indexing as completed and records
// the actual number of files indexed (from the Sync result).
func (s *IndexStateStore) SetCompletedWithStats(dir string, indexedFiles, skippedFiles int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		st = &DirIndexState{Dir: dir}
	}
	st.State = "completed"
	st.IndexedFiles = indexedFiles
	st.TotalFiles = indexedFiles + skippedFiles
	st.CompletedAt = time.Now().Unix()
	s.states[dir] = st
	_ = s.save()
}

// SetCompleted marks a directory's indexing as completed.
func (s *IndexStateStore) SetCompleted(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		st = &DirIndexState{Dir: dir}
	}
	st.State = "completed"
	st.CompletedAt = time.Now().Unix()
	s.states[dir] = st
	_ = s.save()
}

// SetFailed marks a directory's indexing as failed.
func (s *IndexStateStore) SetFailed(dir string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		st = &DirIndexState{Dir: dir}
	}
	st.State = "failed"
	st.Error = errMsg
	st.CompletedAt = time.Now().Unix()
	s.states[dir] = st
	_ = s.save()
}

// Remove deletes a directory's state entry.
func (s *IndexStateStore) Remove(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, dir)
	_ = s.save()
}

// All returns a copy of all states.
func (s *IndexStateStore) All() map[string]*DirIndexState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*DirIndexState, len(s.states))
	for k, v := range s.states {
		result[k] = v
	}
	return result
}

func (s *IndexStateStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var list []*DirIndexState
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	s.states = make(map[string]*DirIndexState, len(list))
	for _, st := range list {
		s.states[st.Dir] = st
	}
	return nil
}

func (s *IndexStateStore) save() error {
	list := make([]*DirIndexState, 0, len(s.states))
	for _, st := range s.states {
		list = append(list, st)
	}
	data, err := json.MarshalIndent(list, "", "  ")
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
