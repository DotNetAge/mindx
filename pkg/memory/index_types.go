package memory

import "time"

// MaxFileSize is the maximum file size (in bytes) allowed for indexing.
// Files larger than this are skipped with a warning.
const MaxFileSize = 1 << 20 // 1MB

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

// ProjectSyncResult summarizes a Sync operation.
type ProjectSyncResult struct {
	Indexed int      // files newly indexed
	Updated int      // files re-indexed due to change
	Skipped int      // files unchanged (cache hit)
	Removed int      // chunks cleaned up from deleted files
	Errors  []string // non-fatal errors grouped by file
	Err     error    // fatal error (operation aborted)
	Elapsed time.Duration
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
