package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSchedulerStore_SaveLoad(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	entry := &ScheduleEntry{
		ID:       "test-job-1",
		Agent:    "test-agent",
		Content:  "run task",
		CronExpr: "*/5 * * * * *",
		Enabled:  true,
	}

	if err := store.Save(ctx, entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx, "test-job-1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != entry.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, entry.ID)
	}
	if loaded.Agent != entry.Agent {
		t.Errorf("Agent = %q, want %q", loaded.Agent, entry.Agent)
	}
	if loaded.Content != entry.Content {
		t.Errorf("Content = %q, want %q", loaded.Content, entry.Content)
	}
	if loaded.CronExpr != entry.CronExpr {
		t.Errorf("CronExpr = %q, want %q", loaded.CronExpr, entry.CronExpr)
	}
	if !loaded.Enabled {
		t.Error("Enabled should be true")
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestFileSchedulerStore_LoadNotFound(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	_, err = store.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Load nonexistent should return error")
	}
}

func TestFileSchedulerStore_Delete(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	entry := &ScheduleEntry{ID: "to-delete", Agent: "a", Content: "c", CronExpr: "* * * * * *"}
	if err := store.Save(ctx, entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 确认存在
	if _, err := store.Load(ctx, "to-delete"); err != nil {
		t.Fatal("Load before delete should succeed")
	}

	// 删除
	if err := store.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 确认已删除
	if _, err := store.Load(ctx, "to-delete"); err == nil {
		t.Fatal("Load after delete should return error")
	}

	// 重复删除应无错误
	if err := store.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete nonexistent should return nil, got: %v", err)
	}
}

func TestFileSchedulerStore_List(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	// 空目录
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List on empty = %d, want 0", len(list))
	}

	// 添加两个 entry
	store.Save(ctx, &ScheduleEntry{ID: "b-job", Agent: "a", Content: "c1", CronExpr: "* * * * * *"})
	store.Save(ctx, &ScheduleEntry{ID: "a-job", Agent: "b", Content: "c2", CronExpr: "* * * * * *"})

	list, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d items, want 2", len(list))
	}

	// 确认按 ID 排序
	if list[0].ID != "a-job" || list[1].ID != "b-job" {
		t.Errorf("List order: got %q, %q; want a-job, b-job", list[0].ID, list[1].ID)
	}
}

func TestFileSchedulerStore_UpdateLastRun(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	entry := &ScheduleEntry{ID: "job-1", Agent: "a", Content: "c", CronExpr: "* * * * * *"}
	if err := store.Save(ctx, entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.UpdateLastRun("job-1", "run-1", nil); err != nil {
		t.Fatalf("UpdateLastRun(success) failed: %v", err)
	}

	loaded, _ := store.Load(ctx, "job-1")
	if loaded.LastStatus != "success" {
		t.Errorf("LastStatus = %q, want %q", loaded.LastStatus, "success")
	}
	if loaded.LastRunID != "run-1" {
		t.Errorf("LastRunID = %q, want %q", loaded.LastRunID, "run-1")
	}
	if loaded.SuccessCnt != 1 {
		t.Errorf("SuccessCnt = %d, want 1", loaded.SuccessCnt)
	}

	if err := store.UpdateLastRun("job-1", "run-2", os.ErrPermission); err != nil {
		t.Fatalf("UpdateLastRun(failure) failed: %v", err)
	}

	loaded, _ = store.Load(ctx, "job-1")
	if loaded.LastStatus != "failed" {
		t.Errorf("LastStatus = %q, want %q", loaded.LastStatus, "failed")
	}
	if loaded.LastError != "permission denied" {
		t.Errorf("LastError = %q, want %q", loaded.LastError, "permission denied")
	}
	if loaded.FailureCnt != 1 {
		t.Errorf("FailureCnt = %d, want 1", loaded.FailureCnt)
	}
}

func TestFileSchedulerStore_LegacyMigrate(t *testing.T) {
	tmpDir := t.TempDir()

	// 模拟旧格式文件
	legacyJSON := `{
		"id": "legacy-job",
		"name": "old-job",
		"command": "run legacy",
		"cron_expr": "0 * * * * *",
		"agent": "legacy-agent",
		"enabled": true
	}`
	legacyPath := filepath.Join(tmpDir, "legacy-job.json")
	if err := os.WriteFile(legacyPath, []byte(legacyJSON), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	entry, err := store.Load(context.Background(), "legacy-job")
	if err != nil {
		t.Fatalf("Load legacy failed: %v", err)
	}

	if entry.Content != "run legacy" {
		t.Errorf("Content (migrated from Command) = %q, want %q", entry.Content, "run legacy")
	}
	if entry.Agent != "legacy-agent" {
		t.Errorf("Agent = %q, want %q", entry.Agent, "legacy-agent")
	}
	if !entry.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestNewFileSchedulerStore_CreateDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "deep", "nested", "schedules")

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore with deep dir failed: %v", err)
	}

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatal("NewFileSchedulerStore should create the directory")
	}

	// 确保 store 可用
	ctx := context.Background()
	entry := &ScheduleEntry{ID: "dir-test", Agent: "a", Content: "c", CronExpr: "* * * * * *"}
	if err := store.Save(ctx, entry); err != nil {
		t.Fatalf("Save in created dir failed: %v", err)
	}
}

func TestFileSchedulerStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewFileSchedulerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileSchedulerStore failed: %v", err)
	}

	// 并发的 Save 和 UpdateLastRun
	done := make(chan struct{})
	go func() {
		store.Save(ctx, &ScheduleEntry{ID: "concurrent-1", Agent: "a", Content: "c", CronExpr: "* * * * * *"})
		done <- struct{}{}
	}()
	go func() {
		store.Save(ctx, &ScheduleEntry{ID: "concurrent-2", Agent: "b", Content: "d", CronExpr: "* * * * * *"})
		done <- struct{}{}
	}()

	<-done
	<-done

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List after concurrent saves failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List returned %d items after concurrent saves, want 2", len(list))
	}
}
