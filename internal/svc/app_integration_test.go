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

	app, err := svc.DefaultApp()
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	app.RegisterBuiltinCommands()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.TestStart(ctx); err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer func() {
		app.TestStop(ctx)
	}()

	wsURL := getWebSocketURL(app)
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

	t.Run("job-add command", func(t *testing.T) {
		callCommand(t, conn, clientID, "job-add", `@personal-assistant 每日晨会提醒 expr="0 0 9 * * *"`)
	})

	t.Run("job-list command", func(t *testing.T) {
		callCommand(t, conn, clientID, "job-list", "")
	})

	t.Run("job-del command", func(t *testing.T) {
		entries, err := app.SchedulerDB().List(context.Background())
		if err != nil {
			t.Fatalf("failed to list scheduler entries: %v", err)
		}
		if len(entries) == 0 {
			t.Skip("no scheduler entries to delete")
		}
		callCommand(t, conn, clientID, "job-del", fmt.Sprintf("id=%s", entries[0].ID))
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

	app, err := svc.DefaultApp()
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	app.RegisterBuiltinCommands()

	metas := svc.GetCommandMetas()

	if len(metas) == 0 {
		t.Fatal("expected at least one command meta")
	}

	expectedCommands := []string{
		"help", "about", "init", "clear",
		"agents", "models", "skills",
		"job-add", "job-list", "job-del",
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

func TestIntegration_JobLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	app, err := svc.DefaultApp()
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	app.RegisterBuiltinCommands()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.TestStart(ctx); err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer func() {
		app.TestStop(ctx)
	}()

	wsURL := getWebSocketURL(app)
	conn := dialTestWS(t, wsURL)
	defer conn.Close()

	clientID := extractClientID(t, conn)

	callCommand(t, conn, clientID, "job-add", `@personal-assistant daily standup reminder expr="0 0 9 * * *"`)

	entries, err := app.SchedulerDB().List(context.Background())
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one scheduler entry")
	}

	entry := entries[0]
	if entry.Agent != "personal-assistant" {
		t.Errorf("expected agent 'personal-assistant', got '%s'", entry.Agent)
	}
	if entry.CronExpr != "0 0 9 * * *" {
		t.Errorf("expected cron '0 0 9 * * *', got '%s'", entry.CronExpr)
	}

	callCommand(t, conn, clientID, "job-list", "")

	callCommand(t, conn, clientID, "job-del", fmt.Sprintf("id=%s", entry.ID))

	entries, err = app.SchedulerDB().List(context.Background())
	if err != nil {
		t.Fatalf("failed to list entries after delete: %v", err)
	}

	for _, e := range entries {
		if e.ID == entry.ID {
			t.Error("entry should have been deleted")
			break
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

	app, err := svc.DefaultApp()
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	app.RegisterBuiltinCommands()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.TestStart(ctx); err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer func() {
		app.TestStop(ctx)
	}()

	wsURL := getWebSocketURL(app)

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

type testEnv struct {
	app     *svc.App
	gw      *gateway.Server
	addr    string
	cleanup func()
}

func loadEnv() error {
	if os.Getenv("MINDX_WORKSPACE") == "" {
		_, filename, _, _ := runtime.Caller(0)
		projectRoot := filepath.Join(filepath.Dir(filename), "../..")
		envFile := filepath.Join(projectRoot, ".env")

		if _, err := os.Stat(envFile); err == nil {
			if err := godotenv.Load(envFile); err != nil {
				return fmt.Errorf("failed to load .env file: %w", err)
			}
		}

		if os.Getenv("MINDX_WORKSPACE") == "" {
			return fmt.Errorf("MINDX_WORKSPACE not set")
		}
	}

	os.Setenv("MINDX_WS_ADDR", ":2323")

	return nil
}

func getWebSocketURL(app *svc.App) string {
	settings := app.Settings()
	addr := settings.Addr
	if addr == "" {
		addr = ":2323"
	}

	wsPath := settings.WSPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	if addr[0] == ':' {
		return fmt.Sprintf("ws://localhost%s%s", addr, wsPath)
	}
	return fmt.Sprintf("ws://localhost:%s%s", addr, wsPath)
}

func setupTestServer(t *testing.T, app *svc.App) *testEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		if err := app.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	time.Sleep(500 * time.Millisecond)

	env := &testEnv{
		app:  app,
		gw:   app.Server(),
		addr: "ws://localhost:1314/ws",
	}

	env.cleanup = func() {
		cancel()
	}

	t.Cleanup(env.cleanup)

	return env
}

func dialTestWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	header := http.Header{"Origin": {url}}

	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

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
