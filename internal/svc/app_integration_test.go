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

	err := loadEnv()
	if err != nil {
		t.Fatalf("failed to load env: %v", err)
	}

	_, err = svc.NewServer(":0", "/ws", nil)
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

	os.Setenv("MINDX_WS_ADDR", ":2323")

	return nil
}
