package indexing

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileRecord represents a single file's state in the session's index manifest.
type FileRecord struct {
	Path         string  `json:"path"`                    // relative to project dir
	State        string  `json:"state"`                   // pending | processing | done | error
	Error        string  `json:"error,omitempty"`         // failure reason
	InputTokens  int     `json:"input_tokens,omitempty"`  // input token count
	OutputTokens int     `json:"output_tokens,omitempty"` // output token count
	CacheTokens  int     `json:"cache_tokens,omitempty"`  // cached token count
	Cost         float64 `json:"cost,omitempty"`          // computed cost (USD)
	Chunks       int     `json:"chunks,omitempty"`        // number of chunks indexed
	Nodes        int     `json:"nodes,omitempty"`         // number of graph nodes extracted
	ElapsedMs    int64   `json:"elapsed_ms,omitempty"`    // processing time in milliseconds
	UpdatedAt    int64   `json:"updated_at"`              // unix timestamp
}

// Manifest is the per-project index manifest that stores file states and a
// FIFO queue. Each project directory has one manifest file, named by SHA256
// of the project path to avoid filesystem conflicts.
type Manifest struct {
	ProjectDir string                 `json:"project_dir"`
	Files      map[string]*FileRecord `json:"files"` // key = relative file path
	Queue      []string               `json:"queue"` // FIFO order of pending file paths
	Processing bool                   `json:"processing"`
}

// NewManifest creates an empty manifest for the given project directory.
func NewManifest(projectDir string) *Manifest {
	return &Manifest{
		ProjectDir: projectDir,
		Files:      make(map[string]*FileRecord),
		Queue:      make([]string, 0),
		Processing: false,
	}
}

// AddFiles appends files to the manifest with state "pending".
// Already-existing files are skipped. Returns the number of files added.
func (m *Manifest) AddFiles(relPaths []string) int {
	added := 0
	now := time.Now().Unix()
	for _, p := range relPaths {
		if p == "" {
			continue
		}
		if _, exists := m.Files[p]; exists {
			continue // skip duplicates
		}
		m.Files[p] = &FileRecord{
			Path:      p,
			State:     "pending",
			UpdatedAt: now,
		}
		m.Queue = append(m.Queue, p)
		added++
	}
	return added
}

// RemoveFiles removes files from the manifest entirely.
// Returns the number of files removed.
func (m *Manifest) RemoveFiles(relPaths []string) int {
	removed := 0
	del := make(map[string]bool, len(relPaths))
	for _, p := range relPaths {
		if _, exists := m.Files[p]; exists {
			delete(m.Files, p)
			del[p] = true
			removed++
		}
	}
	if removed > 0 {
		filtered := make([]string, 0, len(m.Queue))
		for _, p := range m.Queue {
			if !del[p] {
				filtered = append(filtered, p)
			}
		}
		m.Queue = filtered
	}
	return removed
}

// GetState returns the state of a file, or "unindexed" if not in the manifest.
func (m *Manifest) GetState(relPath string) string {
	rec, ok := m.Files[relPath]
	if !ok {
		return "unindexed"
	}
	return rec.State
}

// PeekNext returns the path of the next pending file without removing it.
// Returns "" if the queue is empty.
func (m *Manifest) PeekNext() string {
	for _, p := range m.Queue {
		rec, ok := m.Files[p]
		if ok && rec.State == "pending" {
			return p
		}
	}
	return ""
}

// DequeueNext removes the next pending file from the queue and sets its state
// to "processing". Returns the file path, or "" if nothing is pending.
func (m *Manifest) DequeueNext() string {
	// Find the first file in Queue that is still pending
	for i, p := range m.Queue {
		rec, ok := m.Files[p]
		if ok && rec.State == "pending" {
			rec.State = "processing"
			rec.UpdatedAt = time.Now().Unix()
			// Remove from queue
			m.Queue = append(m.Queue[:i], m.Queue[i+1:]...)
			return p
		}
	}
	return ""
}

// SetDone marks a file as successfully indexed.
func (m *Manifest) SetDone(relPath string, tokens *TokenUsage, elapsedMs int64, chunks int) {
	rec, ok := m.Files[relPath]
	if !ok {
		return
	}
	rec.State = "done"
	rec.Error = ""
	rec.Chunks = chunks
	rec.ElapsedMs = elapsedMs
	rec.UpdatedAt = time.Now().Unix()
	if tokens != nil {
		rec.InputTokens = tokens.InputTokens
		rec.OutputTokens = tokens.OutputTokens
		rec.CacheTokens = tokens.CacheTokens
		rec.Cost = tokens.Cost
	}
}

// SetError marks a file as failed.
func (m *Manifest) SetError(relPath string, errMsg string) {
	rec, ok := m.Files[relPath]
	if !ok {
		return
	}
	rec.State = "error"
	rec.Error = errMsg
	rec.UpdatedAt = time.Now().Unix()
}

// Stats returns summary counts for the manifest.
func (m *Manifest) Stats() (total, pending, processing, done, failed int) {
	total = len(m.Files)
	for _, rec := range m.Files {
		switch rec.State {
		case "pending":
			pending++
		case "processing":
			processing++
		case "done":
			done++
		case "error":
			failed++
		}
	}
	return
}

// PendingCount returns the number of files with state "pending".
func (m *Manifest) PendingCount() int {
	count := 0
	for _, rec := range m.Files {
		if rec.State == "pending" {
			count++
		}
	}
	return count
}

// --- Persistence ---

// ManifestStore manages loading, caching, and saving manifests per project.
type ManifestStore struct {
	dataDir string
	mu      sync.RWMutex
	cache   map[string]*Manifest // key = absolute project dir
}

// NewManifestStore creates a ManifestStore that stores JSON files in dataDir.
func NewManifestStore(dataDir string) *ManifestStore {
	return &ManifestStore{
		dataDir: dataDir,
		cache:   make(map[string]*Manifest),
	}
}

// manifestPath returns the filesystem path for a project's manifest file.
func (s *ManifestStore) manifestPath(projectDir string) string {
	h := sha256.Sum256([]byte(projectDir))
	name := fmt.Sprintf("manifest_%x.json", h)
	return filepath.Join(s.dataDir, name)
}

// Get returns the cached manifest for a project dir, or nil if not loaded.
func (s *ManifestStore) Get(projectDir string) *Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[projectDir]
}

// LoadOrCreate returns the manifest for a project dir, loading from disk or
// creating a new one if it doesn't exist.
func (s *ManifestStore) LoadOrCreate(projectDir string) *Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check cache
	if m, ok := s.cache[projectDir]; ok {
		return m
	}

	// Try loading from disk
	mp := s.manifestPath(projectDir)
	data, err := os.ReadFile(mp)
	if err == nil {
		var m Manifest
		if json.Unmarshal(data, &m) == nil {
			m.ProjectDir = projectDir // ensure consistency
			s.cache[projectDir] = &m
			return &m
		}
	}

	// Create new
	m := NewManifest(projectDir)
	s.cache[projectDir] = m
	return m
}

// Save persists the project's manifest to disk.
func (s *ManifestStore) Save(projectDir string) error {
	s.mu.RLock()
	m, ok := s.cache[projectDir]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("manifest not loaded for %s", projectDir)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest marshal: %w", err)
	}

	mp := s.manifestPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(mp), 0755); err != nil {
		return fmt.Errorf("manifest mkdir: %w", err)
	}
	if err := os.WriteFile(mp, data, 0644); err != nil {
		return fmt.Errorf("manifest write: %w", err)
	}
	return nil
}

// SaveAll persists all cached manifests to disk.
func (s *ManifestStore) SaveAll() {
	s.mu.RLock()
	dirs := make([]string, 0, len(s.cache))
	for dir := range s.cache {
		dirs = append(dirs, dir)
	}
	s.mu.RUnlock()

	sort.Strings(dirs)
	for _, dir := range dirs {
		if err := s.Save(dir); err != nil {
			log.Printf("[manifest] save error for %s: %v", dir, err)
		}
	}
}

// RefreshFromDisk reloads a project's manifest from disk, discarding cache.
func (s *ManifestStore) RefreshFromDisk(projectDir string) *Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()

	mp := s.manifestPath(projectDir)
	data, err := os.ReadFile(mp)
	if err != nil {
		// File doesn't exist — create new
		m := NewManifest(projectDir)
		s.cache[projectDir] = m
		return m
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupt file — create new
		log.Printf("[manifest] corrupt file for %s: %v, creating new", projectDir, err)
		m = Manifest{}
	}
	m.ProjectDir = projectDir
	s.cache[projectDir] = &m
	return &m
}

// All returns a copy of all cached manifests.
func (s *ManifestStore) All() map[string]*Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Manifest, len(s.cache))
	for k, v := range s.cache {
		result[k] = v
	}
	return result
}

// FindForPath returns the manifest whose project directory contains absPath,
// or nil if no matching manifest is found. It checks the cache first, then
// scans the data directory on disk. This ensures the file browser can
// discover manifests without the KB dialog having loaded them first.
func (s *ManifestStore) FindForPath(absPath string) *Manifest {
	s.mu.RLock()
	// Check cache first
	for _, m := range s.cache {
		if strings.HasPrefix(absPath, m.ProjectDir+string(filepath.Separator)) || absPath == m.ProjectDir {
			s.mu.RUnlock()
			return m
		}
	}
	s.mu.RUnlock()

	// Not in cache — scan disk for all manifest files
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock (in case another goroutine loaded it)
	for _, m := range s.cache {
		if strings.HasPrefix(absPath, m.ProjectDir+string(filepath.Separator)) || absPath == m.ProjectDir {
			return m
		}
	}

	pattern := filepath.Join(s.dataDir, "manifest_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	for _, mp := range matches {
		data, err := os.ReadFile(mp)
		if err != nil {
			continue
		}
		var m Manifest
		if json.Unmarshal(data, &m) != nil || m.ProjectDir == "" {
			continue
		}
		// Cache it
		stored := &Manifest{
			ProjectDir: m.ProjectDir,
			Files:      m.Files,
			Queue:      m.Queue,
			Processing: m.Processing,
		}
		s.cache[m.ProjectDir] = stored
		if strings.HasPrefix(absPath, m.ProjectDir+string(filepath.Separator)) || absPath == m.ProjectDir {
			return stored
		}
	}
	return nil
}

// TokenUsage holds token and cost data for a completed indexing operation.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	Cost         float64
	ElapsedMs    int64
}
