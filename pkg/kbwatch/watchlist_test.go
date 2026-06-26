package kbwatch

import (
	"testing"
)

func TestWatchListStoreAddAndList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	entries := store.List()
	if len(entries) != 0 {
		t.Fatalf("expected empty store, got %d entries", len(entries))
	}

	if err := store.Add("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries = store.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Agent != "agent-a" {
		t.Errorf("expected agent-a, got %s", entries[0].Agent)
	}
}

func TestWatchListStoreDedup(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("first Add failed: %v", err)
	}
	if err := store.Add("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", len(store.List()))
	}

	if err := store.Add("/tmp/test-dir", "agent-b"); err != nil {
		t.Fatalf("Add with different agent failed: %v", err)
	}
	if len(store.List()) != 2 {
		t.Errorf("expected 2 entries, got %d", len(store.List()))
	}
}

func TestWatchListStoreRemove(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Add("/tmp/test-dir", "agent-b"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if err := store.Remove("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 entry after remove, got %d", len(store.List()))
	}
	if store.List()[0].Agent != "agent-b" {
		t.Errorf("expected remaining agent-b, got %s", store.List()[0].Agent)
	}
}

func TestWatchListStoreRemoveByDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/test-dir", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Add("/tmp/test-dir", "agent-b"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if err := store.RemoveByDir("/tmp/test-dir"); err != nil {
		t.Fatalf("RemoveByDir failed: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries after RemoveByDir, got %d", len(store.List()))
	}
}

func TestWatchListStoreCoveredByAncestor(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/projects", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	ancestor, ok := store.CoveredByAncestor("/tmp/projects/sub")
	if !ok {
		t.Error("expected CoveredByAncestor to return true")
	}
	if ancestor != "/tmp/projects" {
		t.Errorf("expected ancestor /tmp/projects, got %s", ancestor)
	}

	_, ok = store.CoveredByAncestor("/other/path")
	if ok {
		t.Error("expected CoveredByAncestor to return false for unrelated path")
	}
}

func TestWatchListStoreRemoveDescendants(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/projects/sub", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Add("/another/path", "agent-b"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	removed := store.RemoveDescendants("/tmp/projects")
	if len(removed) != 1 {
		t.Errorf("expected 1 removed descendant, got %d: %v", len(removed), removed)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 entry after RemoveDescendants, got %d", len(store.List()))
	}
	if store.List()[0].Agent != "agent-b" {
		t.Errorf("expected remaining agent-b, got %s", store.List()[0].Agent)
	}
}

func TestWatchListStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore failed: %v", err)
	}

	if err := store.Add("/tmp/persist-dir", "agent-a"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	store2, err := NewWatchListStore(dir)
	if err != nil {
		t.Fatalf("NewWatchListStore reload failed: %v", err)
	}
	if len(store2.List()) != 1 {
		t.Errorf("expected 1 entry after reload, got %d", len(store2.List()))
	}
}

func TestSanitizeDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/me/project", "_Users_me_project"},
		{"/tmp/test-dir", "_tmp_test-dir"},
	}

	for _, tt := range tests {
		got := SanitizeDirName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeDirName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}

	long := ""
	for i := 0; i < 50; i++ {
		long += "/very-long-path-component"
	}
	got := SanitizeDirName(long)
	if len(got) > 200 {
		t.Errorf("SanitizeDirName produced name > 200 chars: %d", len(got))
	}
}
