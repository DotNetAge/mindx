package persistence

import (
	"encoding/json"
	"testing"

	"mindx/internal/entity"
)

func TestMemoryStore_PutAndGet(t *testing.T) {
	store := NewMemoryStore(nil)

	vec := []float64{1.0, 0.0, 0.0}
	meta := map[string]string{"type": "test"}
	if err := store.Put("key1", vec, meta); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	entry, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.Key != "key1" {
		t.Fatalf("expected key 'key1', got %q", entry.Key)
	}
	if len(entry.Vector) != 3 || entry.Vector[0] != 1.0 {
		t.Fatalf("unexpected vector: %v", entry.Vector)
	}
}

func TestMemoryStore_GetNotFound(t *testing.T) {
	store := NewMemoryStore(nil)
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(nil)
	store.Put("key1", []float64{1, 0}, nil)

	if err := store.Delete("key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := store.Get("key1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMemoryStore_BatchPut(t *testing.T) {
	store := NewMemoryStore(nil)

	entries := []entity.VectorEntry{
		{Key: "a", Vector: []float64{1, 0, 0}},
		{Key: "b", Vector: []float64{0, 1, 0}},
		{Key: "c", Vector: []float64{0, 0, 1}},
	}
	if err := store.BatchPut(entries); err != nil {
		t.Fatalf("BatchPut failed: %v", err)
	}
	if store.Size() != 3 {
		t.Fatalf("expected size 3, got %d", store.Size())
	}
	for _, key := range []string{"a", "b", "c"} {
		if _, err := store.Get(key); err != nil {
			t.Fatalf("Get(%q) failed after BatchPut: %v", key, err)
		}
	}
}

func TestMemoryStore_BatchPutEmpty(t *testing.T) {
	store := NewMemoryStore(nil)
	if err := store.BatchPut(nil); err != nil {
		t.Fatalf("BatchPut(nil) should not error: %v", err)
	}
}

func TestMemoryStore_Search(t *testing.T) {
	store := NewMemoryStore(nil)
	store.Put("x", []float64{1, 0, 0}, nil)
	store.Put("y", []float64{0.9, 0.1, 0}, nil)
	store.Put("z", []float64{0, 0, 1}, nil)

	results, err := store.Search([]float64{1, 0, 0}, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// The most similar to [1,0,0] should be "x"
	if results[0].Key != "x" {
		t.Fatalf("expected first result key 'x', got %q", results[0].Key)
	}
}

func TestMemoryStore_SearchWithThreshold(t *testing.T) {
	store := NewMemoryStore(nil)
	store.Put("close", []float64{1, 0, 0}, nil)
	store.Put("far", []float64{0, 0, 1}, nil)

	results, err := store.SearchWithThreshold([]float64{1, 0, 0}, 10, 0.9)
	if err != nil {
		t.Fatalf("SearchWithThreshold failed: %v", err)
	}
	// Only "close" should pass the 0.9 threshold
	for _, r := range results {
		if r.Key == "far" {
			t.Fatal("'far' should have been filtered by threshold")
		}
	}
}

func TestMemoryStore_SearchEmpty(t *testing.T) {
	store := NewMemoryStore(nil)
	results, err := store.Search([]float64{1, 0}, 5)
	if err != nil {
		t.Fatalf("Search on empty store failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestMemoryStore_Scan(t *testing.T) {
	store := NewMemoryStore(nil)
	store.Put("user:1", []float64{1, 0}, nil)
	store.Put("user:2", []float64{0, 1}, nil)
	store.Put("item:1", []float64{1, 1}, nil)

	results, err := store.Scan("user:")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for prefix 'user:', got %d", len(results))
	}
}

func TestMemoryStore_ScanEmptyPrefix(t *testing.T) {
	store := NewMemoryStore(nil)
	store.Put("a", []float64{1}, nil)
	store.Put("b", []float64{2}, nil)

	results, err := store.Scan("")
	if err != nil {
		t.Fatalf("Scan('') failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for empty prefix, got %d", len(results))
	}
}

func TestMemoryStore_PutWithMetadata(t *testing.T) {
	store := NewMemoryStore(nil)
	meta := map[string]interface{}{"name": "test", "score": 42}
	store.Put("k", []float64{1}, meta)

	entry, _ := store.Get("k")
	var decoded map[string]interface{}
	if err := json.Unmarshal(entry.Metadata, &decoded); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if decoded["name"] != "test" {
		t.Fatalf("expected name=test, got %v", decoded["name"])
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore(nil)
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
