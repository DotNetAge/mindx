package kbwatch

import "time"

// MaxFileSize is the maximum file size (in bytes) allowed for indexing.
// Files larger than this are skipped with a warning.
const MaxFileSize = 2202010 // ~2.1MB

// DefaultConcurrency is the default number of files indexed concurrently
// during a full directory scan. This balances throughput against LLM API
// rate limits and resource consumption.
const DefaultConcurrency = 3

// DefaultIgnoredDirs lists directories excluded by default from project indexing.
var DefaultIgnoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".mindx":       true,
}

// FileIndexError records a single file indexing failure with diagnostics.
type FileIndexError struct {
	Path      string        `json:"path"`      // relative path within the project
	Error     string        `json:"error"`     // error message with classification
	Elapsed   time.Duration `json:"elapsed"`   // wall-clock time spent on this file
	Timestamp time.Time     `json:"timestamp"` // when the failure occurred
}

// ErrorType returns the top-level classification of this error.
func (e FileIndexError) ErrorType() string { return classifyErrorString(e.Error) }

// CompletedFileInfo records a successfully indexed file with timing info.
type CompletedFileInfo struct {
	Path      string        `json:"path"`
	Chunks    int           `json:"chunks"`
	Elapsed   time.Duration `json:"elapsed"`
	Timestamp time.Time     `json:"timestamp"`
}

// ProjectSyncResult summarizes a Sync operation.
type ProjectSyncResult struct {
	Indexed int      // files newly indexed
	Updated int      // files re-indexed due to change
	Skipped int      // files unchanged (cache hit)
	Removed int      // chunks cleaned up from deleted files
	Errors  []string // non-fatal error messages grouped by file
	Err     error    // fatal error (operation aborted)
	Elapsed time.Duration

	// FailedFiles records per-file indexing failures with timestamps
	// and error classification. The frontend can surface this to the user
	// rather than silently skipping failed files.
	FailedFiles []FileIndexError

	// CompletedFiles records successfully indexed files with timing info.
	CompletedFiles []CompletedFileInfo
}

// FileState describes the indexing status of a single file.
type FileState string

const (
	FileStateIndexed FileState = "indexed" // exists on disk and matches cache
	FileStateChanged FileState = "changed" // exists on disk but mtime/size differs from cache
	FileStateNew     FileState = "new"     // exists on disk but not in cache
	FileStateRemoved FileState = "removed" // in cache but no longer on disk
	FileStateSkipped FileState = "skipped" // excluded by ignore rules, size limit, or content check
)

// FileStateInfo holds per-file scanning result from ScanFileStates.
type FileStateInfo struct {
	Path        string    `json:"path"`
	State       FileState `json:"state"`
	Size        int64     `json:"size,omitempty"`
	Mtime       int64     `json:"mtime,omitempty"`
	CachedSize  int64     `json:"cached_size,omitempty"`
	CachedMtime int64     `json:"cached_mtime,omitempty"`
	Error       string    `json:"error,omitempty"`
}
