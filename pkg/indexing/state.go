package indexing

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// logSaveErr calls s.save() and logs any error it returns.
func (s *IndexStateStore) logSaveErr() {
	if err := s.save(); err != nil {
		log.Printf("[index-state] save error: %v", err)
	}
}

// saveInterval is the minimum interval between disk writes for incremental
// IndexedFiles updates. Rapid per-file increments are batched to avoid
// excessive I/O during a large index scan.
const saveInterval = 500 * time.Millisecond

// DirIndexState tracks the full-scan state of a watched directory.
type DirIndexState struct {
	Dir          string `json:"dir"`
	State        string `json:"state"`         // pending | indexing | completed | failed
	TotalFiles   int    `json:"total_files"`   // total files discovered during walk
	IndexedFiles int    `json:"indexed_files"` // files processed so far
	Error        string `json:"error,omitempty"`
	StartedAt    int64  `json:"started_at"`
	CompletedAt  int64  `json:"completed_at,omitempty"`

	// EntitiesCreated counts entities extracted and written to graphDB
	// for this directory (populated after a successful sync).
	EntitiesCreated int `json:"entities_created,omitempty"`

	// RelsCreated counts relationships written to graphDB for this directory.
	RelsCreated int `json:"rels_created,omitempty"`

	// TotalElapsedMs is the wall-clock time (ms) spent indexing this directory,
	// from Sync start to completion. Useful for post-hoc performance analysis.
	TotalElapsedMs int64 `json:"total_elapsed_ms,omitempty"`

	// FailedFiles lists individual file indexing failures for display
	// in the frontend's progress panel.
	FailedFiles []FailedFileRecord `json:"failed_files,omitempty"`

	// CompletedFiles lists successfully indexed files with timing info.
	CompletedFiles []CompletedFileRecord `json:"completed_files,omitempty"`

	// IgnoredFiles lists file paths (relative) that the user has chosen to
	// permanently skip. These are excluded from the failed files display
	// and will not be re-indexed.
	IgnoredFiles []string `json:"ignored_files,omitempty"`

	// RegionTitle is the directory-level summary title generated after
	// all files in this directory have been indexed. Populated by
	// RegionIndexer.IndexRegion after Sync completes.
	RegionTitle string `json:"region_title,omitempty"`

	// RegionSummary is the LLM-aggregated summary of this directory's
	// content, generated after all files are indexed.
	RegionSummary string `json:"region_summary,omitempty"`

	// RegionTags are merged, deduplicated tags from all chunks in this
	// directory, collected during region indexing.
	RegionTags []string `json:"region_tags,omitempty"`

	// Processing indicates whether the FIFO queue worker is actively consuming
	// the pending list. When false, the worker is paused (current file finishes,
	// then waits). Set via kb.index.start/stop RPC.
	Processing bool `json:"processing"`

	// CurrentFile is the relative path of the file currently being indexed
	// by the FIFO worker. Empty when idle.
	CurrentFile string `json:"current_file,omitempty"`

	// PendingFiles is the FIFO queue of files awaiting indexing.
	// The worker pops from the front; users add at the back.
	PendingFiles []string `json:"pending_files,omitempty"`
}

// FailedFileRecord records a single failed file during indexing.
type FailedFileRecord struct {
	Path      string `json:"path"`
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"` // unix timestamp
	ElapsedMs int64  `json:"elapsed_ms"`
}

// CompletedFileRecord records a successfully indexed file with timing info
// and LLM token consumption.
type CompletedFileRecord struct {
	Path      string `json:"path"`
	Chunks    int    `json:"chunks"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Timestamp int64  `json:"timestamp"` // unix timestamp

	// Token usage from the last LLM call during indexing of this file.
	// Units: actual token count (not thousands).
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheTokens  int     `json:"cache_tokens"`
	Cost         float64 `json:"cost"` // calculated cost in USD
}

// IndexStateStore persists per-directory index states to disk.
//
// Storage: ~/.mindx/data/index_state.json
type IndexStateStore struct {
	path     string
	states   map[string]*DirIndexState // key = absolute dir path
	mu       sync.RWMutex
	lastSave time.Time // last disk write time (for IncrementIndexedFiles debounce)
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
	s.logSaveErr()
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
	s.logSaveErr()
}

// IncrementIndexedFiles atomically increments IndexedFiles for a directory
// that is currently being indexed. Disk writes are debounced to avoid
// excessive I/O during large scans.
func (s *IndexStateStore) IncrementIndexedFiles(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok || st.State != "indexing" {
		return // not in an active indexing state
	}
	st.IndexedFiles++

	// Debounce disk writes: only save if saveInterval has elapsed since the
	// last disk write.
	if time.Since(s.lastSave) < saveInterval {
		return
	}
	s.lastSave = time.Now()
	s.logSaveErr()
}

// SetCompletedWithStats marks a directory's indexing as completed and records
// the actual number of files indexed (from the Sync result).
// skippedFiles are files that were already in cache (previously indexed),
// so they count as both indexed and total.
func (s *IndexStateStore) SetCompletedWithStats(dir string, indexedFiles, skippedFiles int, entitiesCreated, relsCreated int, totalElapsedMs int64, completedFiles []CompletedFileRecord) {
	s.SetCompletedWithFailedFiles(dir, indexedFiles, skippedFiles, entitiesCreated, relsCreated, totalElapsedMs, completedFiles, nil)
}

// SetCompletedWithFailedFiles marks a directory's indexing as completed,
// recording both the common stats and the list of individual files that
// failed. The frontend uses failed_files to show per-file error details.
// skippedFiles are previously indexed (cached) files — they count as indexed.
func (s *IndexStateStore) SetCompletedWithFailedFiles(dir string, indexedFiles, skippedFiles int, entitiesCreated, relsCreated int, totalElapsedMs int64, completedFiles []CompletedFileRecord, failedFiles []FailedFileRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		st = &DirIndexState{Dir: dir}
	}
	st.State = "completed"
	// Skipped files are already in cache — they count as indexed.
	st.IndexedFiles = indexedFiles + skippedFiles
	st.TotalFiles = indexedFiles + skippedFiles + len(failedFiles)
	st.EntitiesCreated = entitiesCreated
	st.RelsCreated = relsCreated
	st.TotalElapsedMs = totalElapsedMs
	st.CompletedAt = time.Now().Unix()
	if len(completedFiles) > 0 {
		st.CompletedFiles = completedFiles
	}
	if len(failedFiles) > 0 {
		st.FailedFiles = failedFiles
	}
	s.states[dir] = st
	s.logSaveErr()
}

// IgnoreFailedFiles adds the given file paths to the ignored list for
// the specified directory. Ignored files are excluded from failed_files
// in the status response.
func (s *IndexStateStore) IgnoreFailedFiles(dir string, filePaths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	existing := make(map[string]bool, len(st.IgnoredFiles))
	for _, f := range st.IgnoredFiles {
		existing[f] = true
	}
	for _, f := range filePaths {
		if !existing[f] {
			st.IgnoredFiles = append(st.IgnoredFiles, f)
			existing[f] = true
		}
	}
	s.logSaveErr()
}

// IsFileIgnored checks whether a file path is in the ignored list.
func (s *IndexStateStore) IsFileIgnored(dir, relPath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.states[dir]
	if st == nil {
		return false
	}
	for _, f := range st.IgnoredFiles {
		if f == relPath {
			return true
		}
	}
	return false
}

// RemoveFailedFiles removes entries from the failed list.
func (s *IndexStateStore) RemoveFailedFiles(dir string, filePaths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	skip := make(map[string]bool, len(filePaths))
	for _, f := range filePaths {
		skip[f] = true
	}
	filtered := make([]FailedFileRecord, 0, len(st.FailedFiles))
	for _, rec := range st.FailedFiles {
		if !skip[rec.Path] {
			filtered = append(filtered, rec)
		}
	}
	st.FailedFiles = filtered
	s.logSaveErr()
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
	s.logSaveErr()
}

// SetRegion records the Region summary for a directory after region indexing.
// This is called by FileWatchService.SyncDir after all files have been indexed
// and the RegionIndexer has generated the aggregate summary.
func (s *IndexStateStore) SetRegion(dir, title, summary string, tags []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		return
	}
	st.RegionTitle = title
	st.RegionSummary = summary
	st.RegionTags = tags
	s.logSaveErr()
}

// Remove deletes a directory's state entry.
func (s *IndexStateStore) Remove(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, dir)
	s.logSaveErr()
}

// AddPendingFiles adds file paths to the pending list for a directory.
// If the directory has no state yet, a new pending entry is created.
// Files already in the list are silently skipped.
func (s *IndexStateStore) AddPendingFiles(dir string, relPaths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[dir]
	if !ok {
		st = &DirIndexState{
			Dir:   dir,
			State: "pending",
		}
		s.states[dir] = st
	}
	existing := make(map[string]bool, len(st.PendingFiles))
	for _, f := range st.PendingFiles {
		existing[f] = true
	}
	for _, f := range relPaths {
		if !existing[f] {
			st.PendingFiles = append(st.PendingFiles, f)
			existing[f] = true
		}
	}
	s.logSaveErr()
}

// RemovePendingFiles removes file paths from the pending list.
func (s *IndexStateStore) RemovePendingFiles(dir string, relPaths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	skip := make(map[string]bool, len(relPaths))
	for _, f := range relPaths {
		skip[f] = true
	}
	filtered := make([]string, 0, len(st.PendingFiles))
	for _, f := range st.PendingFiles {
		if !skip[f] {
			filtered = append(filtered, f)
		}
	}
	st.PendingFiles = filtered
	s.logSaveErr()
}

// SetProcessing marks whether the FIFO queue worker for a directory is
// actively consuming pending files (false = paused).
func (s *IndexStateStore) SetProcessing(dir string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	st.Processing = active
	s.logSaveErr()
}

// HasPending returns true if the directory has pending files or a current
// file being processed (i.e. the queue has work to do).
func (s *IndexStateStore) HasPending(dir string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.states[dir]
	if st == nil {
		return false
	}
	return len(st.PendingFiles) > 0 || st.CurrentFile != ""
}

// PeekNextPending returns the first pending file without removing it.
// Returns empty string if the queue is empty.
func (s *IndexStateStore) PeekNextPending(dir string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.states[dir]
	if st == nil || len(st.PendingFiles) == 0 {
		return ""
	}
	return st.PendingFiles[0]
}

// PopNextPending removes and returns the first pending file (FIFO) from
// the queue. Returns empty string if the queue is empty.
func (s *IndexStateStore) PopNextPending(dir string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil || len(st.PendingFiles) == 0 {
		return ""
	}
	file := st.PendingFiles[0]
	st.PendingFiles = st.PendingFiles[1:]
	s.logSaveErr()
	return file
}

// SetCurrentFile records which file the FIFO worker is currently indexing.
func (s *IndexStateStore) SetCurrentFile(dir, relPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	st.CurrentFile = relPath
	s.logSaveErr()
}

// ClearCurrentFile clears the current-file field (called when the worker
// finishes processing a file).
func (s *IndexStateStore) ClearCurrentFile(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	st.CurrentFile = ""
	s.logSaveErr()
}

// AddFailedFile appends a failed file record to the directory's state.
func (s *IndexStateStore) AddFailedFile(dir string, rec FailedFileRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	st.FailedFiles = append(st.FailedFiles, rec)
	if st.TotalFiles == 0 {
		st.TotalFiles = 1
	} else {
		st.TotalFiles++
	}
	s.logSaveErr()
}

// AddCompletedFile appends a completed file record and increments the indexed count.
func (s *IndexStateStore) AddCompletedFile(dir string, rec CompletedFileRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[dir]
	if st == nil {
		return
	}
	st.CompletedFiles = append(st.CompletedFiles, rec)
	st.IndexedFiles++
	if st.TotalFiles < st.IndexedFiles+len(st.FailedFiles) {
		st.TotalFiles = st.IndexedFiles + len(st.FailedFiles)
	}
	s.logSaveErr()
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
