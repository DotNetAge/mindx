package svc

import (
	"context"
	"os"
	"testing"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// TestTerminalCwd 终端工作目录解析：真实会话返回其 project_dir；
// 不存在的会话 / 空会话 ID / project_dir 事后被删的会话均回退 daemon 进程 cwd。
func TestTerminalCwd(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	if sessDB == nil {
		t.Fatal("SessDB() is nil")
	}
	fallback, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	// 真实会话：project_dir 为存活的临时目录 → 返回它
	sessionID := mustCreateSession(t, sessDB, "agent-alpha")
	meta, err := sessDB.GetSessionMeta(sessionID)
	if err != nil {
		t.Fatalf("GetSessionMeta: %v", err)
	}
	if meta.ProjectDir == "" {
		t.Fatal("session meta has empty project_dir, test precondition broken")
	}
	if got := d.terminalCwd(sessionID); got != meta.ProjectDir {
		t.Fatalf("terminalCwd(session) = %q, want project_dir %q", got, meta.ProjectDir)
	}

	// 不存在的会话 / 空会话 ID → 回退进程 cwd
	if got := d.terminalCwd("no-such-session"); got != fallback {
		t.Fatalf("terminalCwd(unknown) = %q, want daemon cwd %q", got, fallback)
	}
	if got := d.terminalCwd(""); got != fallback {
		t.Fatalf("terminalCwd(empty) = %q, want daemon cwd %q", got, fallback)
	}

	// project_dir 事后被删（历史会话常见）→ 回退进程 cwd，不硬失败
	dir := t.TempDir()
	info, err := goharnesssession.CreateSession(context.Background(), sessDB, "agent-alpha",
		goharnesssession.WithProjectDirOption(dir))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove project dir: %v", err)
	}
	if got := d.terminalCwd(info.SessionID); got != fallback {
		t.Fatalf("terminalCwd(deleted dir) = %q, want daemon cwd %q", got, fallback)
	}
}
