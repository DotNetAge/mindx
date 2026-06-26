package indexing

import (
	"testing"
)

func TestFileCacheCRUD(t *testing.T) {
	cache := NewProjectFileCache()

	// Get non-existent
	if entry := cache.Get("unknown.go"); entry != nil {
		t.Error("expected nil for non-existent entry")
	}

	// Set and Get
	entry := &projectFileEntry{
		Path:   "test.go",
		Mtime:  1000,
		Size:   500,
		Chunks: []chunkInfo{{ID: "chunk-1"}},
	}
	cache.Set(entry)

	got := cache.Get("test.go")
	if got == nil {
		t.Fatal("expected non-nil entry after Set")
	}
	if got.Path != "test.go" || got.Mtime != 1000 || got.Size != 500 {
		t.Errorf("got path=%s mtime=%d size=%d, want test.go 1000 500",
			got.Path, got.Mtime, got.Size)
	}
	if len(got.Chunks) != 1 || got.Chunks[0].ID != "chunk-1" {
		t.Errorf("unexpected chunks: %v", got.Chunks)
	}

	// Delete
	cache.Delete("test.go")
	if entry := cache.Get("test.go"); entry != nil {
		t.Error("expected nil after Delete")
	}
}

func TestFileCacheOverwrite(t *testing.T) {
	cache := NewProjectFileCache()

	cache.Set(&projectFileEntry{Path: "file.go", Mtime: 100, Size: 200})
	cache.Set(&projectFileEntry{Path: "file.go", Mtime: 300, Size: 400})

	entry := cache.Get("file.go")
	if entry.Mtime != 300 || entry.Size != 400 {
		t.Errorf("overwritten entry has mtime=%d size=%d, want 300 400",
			entry.Mtime, entry.Size)
	}
}

func TestFileCacheLoadSaveRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	cache := NewProjectFileCache()

	cache.Set(&projectFileEntry{Path: "a.go", Mtime: 100, Size: 200,
		Chunks: []chunkInfo{{ID: "c1"}, {ID: "c2"}}})
	cache.Set(&projectFileEntry{Path: "b.go", Mtime: 300, Size: 400})

	if err := cache.SaveToFile(baseDir); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loaded := NewProjectFileCache()
	if err := loaded.LoadFromFile(baseDir); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Check a.go
	a := loaded.Get("a.go")
	if a == nil {
		t.Fatal("expected a.go in loaded cache")
	}
	if a.Mtime != 100 || a.Size != 200 || len(a.Chunks) != 2 {
		t.Errorf("a.go: mtime=%d size=%d chunks=%d, want 100 200 2",
			a.Mtime, a.Size, len(a.Chunks))
	}

	// Check b.go
	b := loaded.Get("b.go")
	if b == nil {
		t.Fatal("expected b.go in loaded cache")
	}
	if b.Mtime != 300 || b.Size != 400 {
		t.Errorf("b.go: mtime=%d size=%d, want 300 400", b.Mtime, b.Size)
	}
}

func TestFileCacheLoadNonExistent(t *testing.T) {
	cache := NewProjectFileCache()
	err := cache.LoadFromFile(t.TempDir()) // no cache file exists
	if err != nil {
		t.Fatalf("LoadFromFile on empty dir should not error, got: %v", err)
	}
}
