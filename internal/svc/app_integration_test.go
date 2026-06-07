package svc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

func TestIntegration_AllCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	server, err := svc.NewServer(":0", "/ws")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	server.RegisterBuiltinCommands()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Daemon().TestStart(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		server.Daemon().TestStop(ctx)
	}()

	wsURL := getWebSocketURLFromDaemon(server)
	conn := dialTestWS(t, wsURL)
	defer conn.Close()

	clientID := extractClientID(t, conn)

	t.Run("help command", func(t *testing.T) {
		callCommand(t, conn, clientID, "help", "")
	})

	t.Run("about command", func(t *testing.T) {
		callCommand(t, conn, clientID, "about", "")
	})

	t.Run("init command", func(t *testing.T) {
		callCommand(t, conn, clientID, "init", "")
	})

	t.Run("clear command", func(t *testing.T) {
		callCommand(t, conn, clientID, "clear", "")
	})

	t.Run("agents command", func(t *testing.T) {
		callCommand(t, conn, clientID, "agents", "")
	})

	t.Run("models command", func(t *testing.T) {
		callCommand(t, conn, clientID, "models", "")
	})

	t.Run("skills command", func(t *testing.T) {
		callCommand(t, conn, clientID, "skills", "")
	})
}

func TestIntegration_CommandMetas(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	_, err = svc.NewServer(":0", "/ws")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	metas := svc.GetCommandMetas()

	if len(metas) == 0 {
		t.Fatal("expected at least one command meta")
	}

	expectedCommands := []string{
		"help", "about", "init", "clear",
		"agents", "models", "skills",
	}

	for _, expected := range expectedCommands {
		found := false
		for _, meta := range metas {
			if meta.Name == expected {
				found = true
				if meta.Description == "" {
					t.Errorf("command %s has empty description", expected)
				}
				if meta.Category == "" {
					t.Errorf("command %s has empty category", expected)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected command %s not found in metas", expected)
		}
	}
}

func TestIntegration_ConcurrentCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	server, err := svc.NewServer(":0", "/ws")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	server.RegisterBuiltinCommands()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Daemon().TestStart(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		server.Daemon().TestStop(ctx)
	}()

	wsURL := getWebSocketURLFromDaemon(server)

	commands := []string{"help", "about", "agents", "models", "skills"}

	done := make(chan bool, len(commands))

	for _, cmd := range commands {
		go func(command string) {
			conn := dialTestWS(t, wsURL)
			defer conn.Close()

			clientID := extractClientID(t, conn)
			callCommand(t, conn, clientID, command, "")
			done <- true
		}(cmd)
	}

	for i := 0; i < len(commands); i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("test timed out")
		}
	}
}

func loadEnv() error {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	envFile := filepath.Join(projectRoot, ".env")

	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	os.Setenv("MINDX_WS_ADDR", ":2323")

	return nil
}

func getWebSocketURLFromDaemon(server *svc.Server) string {
	daemon := server.Daemon()
	addr := daemon.Addr()
	if addr == "" {
		addr = ":2323"
	}

	wsPath := daemon.WSPath()
	if wsPath == "" {
		wsPath = "/ws"
	}

	if addr[0] == ':' {
		return fmt.Sprintf("ws://localhost%s%s", addr, wsPath)
	}
	return fmt.Sprintf("ws://localhost:%s%s", addr, wsPath)
}

func dialTestWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	header := http.Header{"Origin": {url}}

	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })

	return conn
}

func extractClientID(t *testing.T, conn *websocket.Conn) string {
	t.Helper()

	return fmt.Sprintf("test-client-%d", time.Now().UnixNano())
}

func callCommand(t *testing.T, conn *websocket.Conn, clientID, command, args string) {
	t.Helper()

	req := gateway.Request{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("%s-%d", command, time.Now().UnixNano()),
		Method:  command,
	}

	if args != "" {
		params := map[string]string{"args": args}
		paramsBytes, _ := json.Marshal(params)
		req.Params = paramsBytes
	}

	reqBytes, _ := json.Marshal(req)
	conn.WriteMessage(websocket.TextMessage, reqBytes)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		_, respBytes, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read response for %s: %v", command, err)
		}

		lines := string(respBytes)
		for _, line := range splitLines(lines) {
			if line == "" {
				continue
			}

			var resp gateway.Response
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				continue
			}

			if resp.ID != nil {
				respIDStr, ok := resp.ID.(string)
				if ok && respIDStr == req.ID {
					if resp.Error != nil {
						t.Errorf("command %s returned error: %s", command, resp.Error.Message)
					}
					return
				}
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
