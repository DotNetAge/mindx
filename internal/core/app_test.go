package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/pkg/logging"
)

func TestSettings_Directories(t *testing.T) {
	s := &Settings{}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"SkillsDir", s.SkillsDir(), "/tmp/mindx-test/skills"},
		{"ModelsFile", s.ModelsFile(), "/tmp/mindx-test/settings/models.yml"},
		// {"ProgramDir", s.ProgramDir(), "/tmp/mindx-test/programs"},
		// {"DocumentDir", s.DocumentDir(), "/tmp/mindx-test/documents"},
		{"DataDir", s.DataDir(), "/tmp/mindx-test/data"},
		{"AgentsDir", s.AgentsDir(), "/tmp/mindx-test/agents"},
		{"RulesFile", s.RulesFile(), "/tmp/mindx-test/settings/rules.yml"},
		{"SessionsDir", s.SessionsDir(), "/tmp/mindx-test/sessions"},
		{"SchedulesDir", s.SchedulesDir(), "/tmp/mindx-test/data/schedules"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestNewApp(t *testing.T) {
	tmpDir := t.TempDir()

	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	os.MkdirAll(filepath.Join(tmpDir, "agents"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "settings"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "settings", "models.yml"), []byte{}, 0644)
	os.WriteFile(filepath.Join(tmpDir, "settings", "rules.yml"), []byte{}, 0644)
	os.MkdirAll(filepath.Join(tmpDir, "sessions"), 0755)

	app, err := DefaultApp(nil)
	if err != nil {
		t.Fatalf("DefaultApp() error = %v", err)
	}

	if app == nil {
		t.Fatal("DefaultApp() returned nil")
	}

	// if app.Settings().UserPreferences() != tmpDir {
	// 	t.Errorf("App.Workspace = %v, want %v", app.Settings().UserPreferences(), tmpDir)
	// }

	if app.Agents() == nil {
		t.Error("App.Agents() returned nil")
	}

	if app.Models() == nil {
		t.Error("App.Models() returned nil")
	}
}

func TestApp_SetLogger(t *testing.T) {
	app := &App{}
	logger := logging.DefaultConsoleLogger()

	app.SetLogger(logger)

	if app.logger == nil {
		t.Error("SetLogger() did not set logger")
	}
}

func TestApp_Accessors(t *testing.T) {
	app := &App{
		settings: &Settings{Test: true},
		logger:   logging.DefaultConsoleLogger(),
	}

	if app.Settings() == nil {
		t.Error("Settings() returned nil")
	}

	if app.RuleRegistry() != nil {
		t.Error("RuleRegistry() should return nil when not initialized")
	}

	if app.SessionDB() != nil {
		t.Error("SessionDB() should return nil when not initialized")
	}
}
