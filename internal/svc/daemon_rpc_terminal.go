package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/creack/pty"
)

// terminalSession 管理一个 PTY 终端会话
type terminalSession struct {
	id        string
	cmd       *exec.Cmd
	pty       *os.File
	clientID  string
	daemon    *Daemon
	done      chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
}

type terminalManager struct {
	mu       sync.Mutex
	sessions map[string]*terminalSession
	counter  int64
}

var termMgr = &terminalManager{
	sessions: make(map[string]*terminalSession),
}

func (tm *terminalManager) nextID() string {
	id := atomic.AddInt64(&tm.counter, 1)
	return fmt.Sprintf("term_%d", id)
}

func (tm *terminalManager) add(s *terminalSession) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.sessions[s.id] = s
}

func (tm *terminalManager) get(id string) *terminalSession {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.sessions[id]
}

func (tm *terminalManager) remove(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.sessions, id)
}

// cleanupClient 断开客户端时清理该客户端的所有终端会话
func (tm *terminalManager) cleanupClient(clientID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for id, ts := range tm.sessions {
		if ts.clientID == clientID {
			ts.pty.Close()
			ts.cmd.Process.Kill()
			delete(tm.sessions, id)
		}
	}
}

// handleTerminalStart 启动一个新的终端会话
func (d *Daemon) handleTerminalStart(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Cwd string `json:"cwd"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" {
		return nil, fmt.Errorf("client_id required")
	}

	cmd := exec.Command("zsh")
	cmd.Dir = req.Cwd

	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	sessionID := termMgr.nextID()

	ts := &terminalSession{
		id:       sessionID,
		cmd:      cmd,
		pty:      f,
		clientID: clientID,
		daemon:   d,
		done:     make(chan struct{}),
	}

	termMgr.add(ts)

	// PTY 输出 → WebSocket 推送
	go func() {
		defer ts.close()
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			if err != nil {
				if err != io.EOF {
					d.logger.Error("terminal read error", err,
						"session_id", sessionID)
				}
				return
			}
			if n > 0 {
				d.gw.SendResponse(clientID, gateway.ResponseType("terminal.output"),
					"", string(buf[:n]),
					gateway.WithSessionID(sessionID))
			}
		}
	}()

	// 进程退出通知
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		ts.close()
		d.gw.SendResponse(clientID, gateway.ResponseType("terminal.exit"),
			"", "",
			gateway.WithSessionID(sessionID),
			gateway.WithResponseMeta(map[string]interface{}{
				"exit_code": exitCode,
			}))
	}()

	d.logger.Info("terminal started", "session_id", sessionID, "cwd", req.Cwd)
	return map[string]any{"session_id": sessionID}, nil
}

// handleTerminalInput 将键盘输入发送到 PTY
func (d *Daemon) handleTerminalInput(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SessionID string `json:"session_id"`
		Data      string `json:"data"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	ts := termMgr.get(req.SessionID)
	if ts == nil {
		return nil, fmt.Errorf("terminal session not found: %s", req.SessionID)
	}

	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" || clientID != ts.clientID {
		return nil, fmt.Errorf("permission denied: not your terminal")
	}

	if _, err := ts.pty.Write([]byte(req.Data)); err != nil {
		return nil, fmt.Errorf("failed to write to pty: %w", err)
	}

	return nil, nil
}

// handleTerminalResize 调整 PTY 窗口大小
func (d *Daemon) handleTerminalResize(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SessionID string `json:"session_id"`
		Rows      uint16 `json:"rows"`
		Cols      uint16 `json:"cols"`
		X         uint16 `json:"x"`
		Y         uint16 `json:"y"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	ts := termMgr.get(req.SessionID)
	if ts == nil {
		return nil, fmt.Errorf("terminal session not found: %s", req.SessionID)
	}

	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" || clientID != ts.clientID {
		return nil, fmt.Errorf("permission denied: not your terminal")
	}

	if err := pty.Setsize(ts.pty, &pty.Winsize{
		Rows: req.Rows,
		Cols: req.Cols,
		X:    req.X,
		Y:    req.Y,
	}); err != nil {
		return nil, fmt.Errorf("failed to resize pty: %w", err)
	}

	return nil, nil
}

// handleTerminalKill 强制终止终端会话
func (d *Daemon) handleTerminalKill(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	ts := termMgr.get(req.SessionID)
	if ts == nil {
		return nil, fmt.Errorf("terminal session not found: %s", req.SessionID)
	}

	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" || clientID != ts.clientID {
		return nil, fmt.Errorf("permission denied: not your terminal")
	}

	ts.close()
	return map[string]any{"status": "closed"}, nil
}

// handleTerminalList 列出当前客户端的所有活跃终端会话
func (d *Daemon) handleTerminalList(ctx context.Context, params json.RawMessage) (any, error) {
	clientID := gateway.ClientIDFromContext(ctx)

	termMgr.mu.Lock()
	defer termMgr.mu.Unlock()

	var sessions []map[string]any
	for id, ts := range termMgr.sessions {
		if ts.clientID == clientID {
			sessions = append(sessions, map[string]any{
				"session_id": id,
			})
		}
	}
	return map[string]any{"sessions": sessions}, nil
}

func (ts *terminalSession) close() {
	ts.closeOnce.Do(func() {
		ts.closed.Store(true)
		termMgr.remove(ts.id)
		ts.pty.Close()
		ts.cmd.Process.Kill()
		close(ts.done)
	})
}
