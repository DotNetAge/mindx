package indexing

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

// FileState is the index state of a single file.
type FileState int

const (
	FilePending    FileState = iota // in manifest, waiting for enqueue
	FileEnqueued                    // enqueued, waiting for worker
	FileProcessing                  // being indexed by worker
	FileIndexed                     // indexed successfully
	FileFailed                      // indexing failed
)

// FileMeta is the per-file state entity stored in boltDB.
type FileMeta struct {
	Path     string    `json:"path"`                // absolute path, unique key
	State    FileState `json:"state"`               // index state
	Error    string    `json:"error,omitempty"`     // failure reason
	Mtime    int64     `json:"mtime,omitempty"`     // modification time (nanoseconds)
	Size     int64     `json:"size,omitempty"`      // file size (bytes)
	ChunkIDs []string  `json:"chunk_ids,omitempty"` // indexed chunk IDs, for cleanup

	// Audit / billing fields
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CacheTokens  int     `json:"cache_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
	Chunks       int     `json:"chunks,omitempty"`     // number of chunks
	Nodes        int     `json:"nodes,omitempty"`      // number of graph nodes
	ElapsedMs    int64   `json:"elapsed_ms,omitempty"` // processing time in ms
	UpdatedAt    int64   `json:"updated_at"`           // unix timestamp of last update
}

// IndexerStatus is the runtime status returned by Status().
type IndexerStatus struct {
	ProjectDir   string `json:"project_dir"`
	Running      bool   `json:"running"`
	PendingCount int    `json:"pending_count"`
	Enqueued     int    `json:"enqueued"`
	Processing   string `json:"processing"` // current file being processed
	DoneCount    int    `json:"done_count"`
	ErrorCount   int    `json:"error_count"`
	TotalChunks  int    `json:"total_chunks"`
}

// IndexerCallbacks holds event hooks. Daemon uses them to broadcast
// JSON-RPC notifications to WebUI clients.
type IndexerCallbacks struct {
	// OnFileAdded is called when Add() discovers a new or changed file.
	OnFileAdded func(ctx interface{}, path string)

	// OnFileIndexStart is called when worker begins indexing a file.
	OnFileIndexStart func(ctx interface{}, path string)

	// OnFileIndexDone is called when a file has been indexed successfully.
	OnFileIndexDone func(ctx interface{}, path string)

	// OnFileIndexFail is called when indexing a file has failed.
	OnFileIndexFail func(ctx interface{}, path, errMsg string)

	// OnFileRemoved is called after a file chunk has been cleaned up.
	OnFileRemoved func(ctx interface{}, path string)

	// OnQueueEmpty is called when the queue becomes empty.
	OnQueueEmpty func(ctx interface{})
}

// IndexerOption configures the Indexer.
type IndexerOption func(*Indexer)
