package kbwatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// projectFileEntry tracks a single indexed file's metadata.
type projectFileEntry struct {
	Path   string      `json:"path"`
	Mtime  int64       `json:"mtime"`
	Size   int64       `json:"size"`
	Chunks []chunkInfo `json:"chunks"`
}

// chunkInfo holds the identifier of an indexed chunk.
type chunkInfo struct {
	ID string `json:"id"`
}

// fileCache persists file indexing metadata to disk.
// It is safe for concurrent use.
type fileCache struct {
	Files map[string]*projectFileEntry `json:"files"`
	mu    sync.Mutex
}

// NewProjectFileCache creates an empty cache.
func NewProjectFileCache() *fileCache {
	return &fileCache{
		Files: make(map[string]*projectFileEntry),
	}
}

// Get returns the cached entry for path, or nil.
func (c *fileCache) Get(path string) *projectFileEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Files[path]
}

// Set stores or updates a cached entry.
func (c *fileCache) Set(entry *projectFileEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Files[entry.Path] = entry
}

// Delete removes a cached entry by path.
func (c *fileCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Files, path)
}

// cacheFilePath returns the cache file path within the given base directory.
func cacheFilePath(baseDir string) string {
	return filepath.Join(baseDir, "index_cache.json")
}

// LoadFromFile reads the cache from disk. Returns nil if the file does not exist.
func (c *fileCache) LoadFromFile(baseDir string) error {
	path := cacheFilePath(baseDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, c)
}

// SaveToFile writes the cache atomically to disk (via temp file + rename).
func (c *fileCache) SaveToFile(baseDir string) error {
	path := cacheFilePath(baseDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
