package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestWatcher 构造只依赖目录的 watcher（不经过 core.App）。
// 这些用例只验证生命周期：目录事件触发重载需要 App，属于另一层的事。
func newTestWatcher(t *testing.T) *HotReloadWatcher {
	t.Helper()
	root := t.TempDir()
	agents := filepath.Join(root, "agents")
	skills := filepath.Join(root, "skills")
	teams := filepath.Join(root, "teams")
	for _, d := range []string{agents, skills, teams} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &HotReloadWatcher{agentsDir: agents, skillsDir: skills, teamsDir: teams}
}

// TestWatcherStopDoesNotBlock Start/Stop 必须能在毫秒级完成，不能挂住停机路径。
//
// 回归点（P1 §4.7）：旧实现 Start 丢弃调用方 ctx、isRunning 是无锁裸 bool，
// Stop 的 <-w.done 无超时——状态残留时 Stop 会永久阻塞。
func TestWatcherStopDoesNotBlock(t *testing.T) {
	w := newTestWatcher(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop 阻塞超过 5 秒")
	}

	// 幂等：重复 Stop 不 panic、不阻塞
	w.Stop()
	if w.IsRunning() {
		t.Fatal("Stop 后 IsRunning 应为 false")
	}
}

// TestWatcherHonorsCallerContext 调用方 ctx 取消后事件循环必须自行退出。
//
// 回归点（P1 §4.7）：旧实现用 context.WithCancel(context.Background())，
// 调用方取消对 watcher 完全不可见。
func TestWatcherHonorsCallerContext(t *testing.T) {
	w := newTestWatcher(t)
	ctx, cancel := context.WithCancel(context.Background())

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !w.IsRunning() {
		t.Fatal("Start 后应处于运行态")
	}

	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !w.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("调用方 ctx 取消后事件循环没有退出")
}

// TestWatcherStopWithoutStart 未启动时 Stop 必须立即返回（不能等到超时）。
func TestWatcherStopWithoutStart(t *testing.T) {
	w := newTestWatcher(t)
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("未启动时 Stop 不应阻塞")
	}
}

// TestWatcherRestart 停机后可重新启动（生命周期状态必须被复位）。
func TestWatcherRestart(t *testing.T) {
	w := newTestWatcher(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	w.Stop()
	if w.IsRunning() {
		t.Fatal("Stop 后应停止")
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("重启 Start: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !w.IsRunning() {
		time.Sleep(10 * time.Millisecond)
	}
	if !w.IsRunning() {
		t.Fatal("重启后应恢复运行态")
	}
	w.Stop()
}
