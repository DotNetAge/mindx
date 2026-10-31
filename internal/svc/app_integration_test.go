package svc_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/joho/godotenv"
)

func TestIntegration_CommandMetas(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 隔离用户根：DefaultUserPrefsDir 按 $HOME 解析 ~/.mindx，真实 kv.db 被
	// 运行中 daemon 持锁，NewDaemon 重试后显式失败——测试必须用临时 HOME
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 临时 HOME 无初装种子（NewServer 不做 ExtractWorkspace），种最小模型配置
	settingsDir := filepath.Join(home, ".mindx", "settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("mkdir settings: %v", err)
	}
	modelsYml := "models:\n  - name: test-model\n    title: Test Model\n    description: integration test stub\n    provider: dashscope\n    cost_per_1m_in: 1\n    cost_per_1m_out: 2\n    context_length: 8192\n    is_local: false\n    func_calling: true\n    structuring: true\n"
	if err := os.WriteFile(filepath.Join(settingsDir, "models.yml"), []byte(modelsYml), 0o644); err != nil {
		t.Fatalf("write models.yml: %v", err)
	}

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	_, err = svc.NewServer(":0", "/ws", nil, nil)
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

func loadEnv() error {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	envFile := filepath.Join(projectRoot, ".env")

	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	_ = os.Setenv("MINDX_WS_ADDR", ":2323")

	return nil
}
