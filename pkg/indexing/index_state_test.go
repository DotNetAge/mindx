package indexing

import (
	"testing"
)

func TestIndexStateStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	// Initially empty
	all := store.All()
	if len(all) != 0 {
		t.Errorf("expected empty store, got %d states", len(all))
	}

	// Set pending
	store.SetPending("/tmp/project")
	st := store.Get("/tmp/project")
	if st == nil {
		t.Fatal("expected non-nil state after SetPending")
	}
	if st.State != "pending" {
		t.Errorf("expected state 'pending', got '%s'", st.State)
	}
	if st.Dir != "/tmp/project" {
		t.Errorf("expected dir /tmp/project, got %s", st.Dir)
	}

	// Set indexing
	store.SetIndexing("/tmp/project", 100)
	st = store.Get("/tmp/project")
	if st.State != "indexing" {
		t.Errorf("expected state 'indexing', got '%s'", st.State)
	}
	if st.TotalFiles != 100 {
		t.Errorf("expected TotalFiles 100, got %d", st.TotalFiles)
	}

	// Increment
	store.IncrementIndexedFiles("/tmp/project")
	st = store.Get("/tmp/project")
	if st.IndexedFiles != 1 {
		t.Errorf("expected IndexedFiles 1, got %d", st.IndexedFiles)
	}

	// Set completed with stats
	store.SetCompletedWithStats("/tmp/project", 50, 50, 10, 5, 1000, nil)
	st = store.Get("/tmp/project")
	if st.State != "completed" {
		t.Errorf("expected state 'completed', got '%s'", st.State)
	}
	if st.IndexedFiles != 100 {
		t.Errorf("expected IndexedFiles 100, got %d", st.IndexedFiles)
	}
	if st.EntitiesCreated != 10 || st.RelsCreated != 5 {
		t.Errorf("expected entities=10 rels=5, got entities=%d rels=%d",
			st.EntitiesCreated, st.RelsCreated)
	}
	if st.TotalElapsedMs != 1000 {
		t.Errorf("expected TotalElapsedMs 1000, got %d", st.TotalElapsedMs)
	}
	if st.CompletedAt == 0 {
		t.Error("expected CompletedAt to be set")
	}
}

func TestIndexStateStoreSetFailed(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/project")
	store.SetFailed("/tmp/project", "something went wrong")

	st := store.Get("/tmp/project")
	if st.State != "failed" {
		t.Errorf("expected state 'failed', got '%s'", st.State)
	}
	if st.Error != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got '%s'", st.Error)
	}
}

func TestIndexStateStoreGetNonExistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	if st := store.Get("/nonexistent"); st != nil {
		t.Error("expected nil for non-existent dir")
	}
}

func TestIndexStateStoreRemove(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/project")
	store.Remove("/tmp/project")

	if st := store.Get("/tmp/project"); st != nil {
		t.Error("expected nil after Remove")
	}
}

func TestIndexStateStoreSetCompletedWithFailedFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/project")
	store.SetCompletedWithFailedFiles("/tmp/project", 40, 50, 8, 3, 2000,
		[]CompletedFileRecord{
			{Path: "ok.go", Chunks: 2, ElapsedMs: 100, Timestamp: 1000},
		},
		[]FailedFileRecord{
			{Path: "bad.go", Error: "timeout", Timestamp: 1001, ElapsedMs: 500},
		},
	)

	st := store.Get("/tmp/project")
	if st.State != "completed" {
		t.Errorf("expected 'completed', got '%s'", st.State)
	}
	if st.IndexedFiles != 90 {
		t.Errorf("expected IndexedFiles 90 (40+50), got %d", st.IndexedFiles)
	}
	if st.TotalFiles != 91 {
		t.Errorf("expected TotalFiles 91 (40+50+1), got %d", st.TotalFiles)
	}
	if len(st.CompletedFiles) != 1 || st.CompletedFiles[0].Path != "ok.go" {
		t.Errorf("unexpected completed files: %v", st.CompletedFiles)
	}
	if len(st.FailedFiles) != 1 || st.FailedFiles[0].Path != "bad.go" {
		t.Errorf("unexpected failed files: %v", st.FailedFiles)
	}
}

func TestIndexStateStoreAll(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/a")
	store.SetPending("/tmp/b")

	all := store.All()
	if len(all) != 2 {
		t.Errorf("expected 2 states, got %d", len(all))
	}
	if all["/tmp/a"] == nil || all["/tmp/b"] == nil {
		t.Error("expected both /tmp/a and /tmp/b in All()")
	}
}

func TestIndexStateStoreIgnoreFailedFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/project")
	store.SetCompletedWithFailedFiles("/tmp/project", 0, 0, 0, 0, 0, nil,
		[]FailedFileRecord{
			{Path: "bad.go", Error: "timeout"},
			{Path: "ugly.go", Error: "parse error"},
		},
	)

	store.IgnoreFailedFiles("/tmp/project", []string{"bad.go"})
	if !store.IsFileIgnored("/tmp/project", "bad.go") {
		t.Error("expected bad.go to be ignored")
	}
	if store.IsFileIgnored("/tmp/project", "ugly.go") {
		t.Error("expected ugly.go to NOT be ignored")
	}

	store.RemoveFailedFiles("/tmp/project", []string{"bad.go", "ugly.go"})
	st := store.Get("/tmp/project")
	if len(st.FailedFiles) != 0 {
		t.Errorf("expected 0 failed files after RemoveFailedFiles, got %d", len(st.FailedFiles))
	}
}

func TestIndexStateStoreSetCompletedOnNonExistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetCompletedWithStats("/tmp/new", 0, 0, 0, 0, 0, nil)
	st := store.Get("/tmp/new")
	if st == nil || st.State != "completed" {
		t.Errorf("expected completed state, got %v", st)
	}
}

func TestIndexStateStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore failed: %v", err)
	}

	store.SetPending("/tmp/project")
	store.SetCompletedWithStats("/tmp/project", 10, 5, 3, 1, 500, nil)

	// Reload from disk
	store2, err := NewIndexStateStore(dir)
	if err != nil {
		t.Fatalf("NewIndexStateStore reload failed: %v", err)
	}

	st := store2.Get("/tmp/project")
	if st == nil {
		t.Fatal("expected state after reload")
	}
	if st.State != "completed" || st.IndexedFiles != 15 {
		t.Errorf("after reload: state=%s indexed=%d, want completed 15",
			st.State, st.IndexedFiles)
	}
}
