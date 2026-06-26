package kbwatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestIndexServiceSyncWithNilIndexer verifies that Sync returns immediately
// without error when the indexer is nil (no embedder configured).
func TestIndexServiceSyncWithNilIndexer(t *testing.T) {
	svc := NewIndexService(nil, t.TempDir(), nil)
	result := svc.Sync(context.Background(), t.TempDir())
	if result.Err != nil {
		t.Fatalf("Sync with nil indexer should not return error, got: %v", result.Err)
	}
	if result.Indexed != 0 || result.Updated != 0 || result.Skipped != 0 {
		t.Errorf("Sync with nil indexer should produce zero counts, got indexed=%d updated=%d skipped=%d",
			result.Indexed, result.Updated, result.Skipped)
	}
}

// TestIndexServiceSyncFilesWithNilIndexer verifies that SyncFiles returns
// immediately without error when the indexer is nil.
func TestIndexServiceSyncFilesWithNilIndexer(t *testing.T) {
	svc := NewIndexService(nil, t.TempDir(), nil)
	result := svc.SyncFiles(context.Background(), t.TempDir(), []string{"file.go"}, false)
	if result.Err != nil {
		t.Fatalf("SyncFiles with nil indexer should not return error, got: %v", result.Err)
	}
	if result.Skipped != 1 {
		t.Errorf("SyncFiles with nil indexer should have 1 skipped, got %d", result.Skipped)
	}
}

// TestIndexServiceSyncFilesDeletedWithNilIndexer verifies SyncFiles with
// deleted=true does not panic when indexer is nil.
func TestIndexServiceSyncFilesDeletedWithNilIndexer(t *testing.T) {
	svc := NewIndexService(nil, t.TempDir(), nil)
	result := svc.SyncFiles(context.Background(), t.TempDir(), []string{"deleted.go"}, true)
	if result.Err != nil {
		t.Fatalf("SyncFiles (deleted) with nil indexer should not return error, got: %v", result.Err)
	}
}

// TestIndexServiceSyncWithNilIndexerWritesNoCache verifies that after Sync
// with nil indexer, the cache file is NOT created (files unprocessed).
func TestIndexServiceSyncWithNilIndexerWritesNoCache(t *testing.T) {
	cacheDir := t.TempDir()
	svc := NewIndexService(nil, cacheDir, nil)

	// Create a test file in a temp project dir
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "test.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := svc.Sync(context.Background(), projectDir)
	if result.Err != nil {
		t.Fatalf("Sync failed: %v", result.Err)
	}

	// Cache should not exist on disk (nothing was indexed)
	cachePath := filepath.Join(cacheDir, "index_cache.json")
	if _, err := os.Stat(cachePath); err == nil {
		t.Error("cache file should not exist after Sync with nil indexer")
	}
}

// TestIndexServiceClearCacheEntry verifies that ClearCacheEntry works on a
// fresh IndexService with nil indexer (no panic).
func TestIndexServiceClearCacheEntry(t *testing.T) {
	svc := NewIndexService(nil, t.TempDir(), nil)
	svc.ClearCacheEntry("some/file.go")
	// No panic = success
}

// TestIsValidFileContentForScan checks the quick validation path.
func TestIsValidFileContentForScan(t *testing.T) {
	dir := t.TempDir()

	// Valid text file
	textPath := "valid.txt"
	if err := os.WriteFile(filepath.Join(dir, textPath), []byte("hello world this is a text file with enough content"), 0644); err != nil {
		t.Fatal(err)
	}
	if !isValidFileContentForScan(dir, textPath) {
		t.Error("expected valid text file to pass scan check")
	}

	// Binary file with null byte
	binPath := "binary.bin"
	if err := os.WriteFile(filepath.Join(dir, binPath), []byte{0x00, 0x01, 0x02}, 0644); err != nil {
		t.Fatal(err)
	}
	if isValidFileContentForScan(dir, binPath) {
		t.Error("expected binary file to fail scan check")
	}

	// Empty file
	emptyPath := "empty.txt"
	if err := os.WriteFile(filepath.Join(dir, emptyPath), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if isValidFileContentForScan(dir, emptyPath) {
		t.Error("expected empty file to fail scan check")
	}
}
