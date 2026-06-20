package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/pkg/logging"
)

func TestSettings_Directories(t *testing.T) {
	tmpDir := t.TempDir()
	s := &Settings{Test: true, testDir: tmpDir}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"SkillsDir", s.SkillsDir(), filepath.Join(tmpDir, "skills")},
		{"ModelsFile", s.ModelsFile(), filepath.Join(tmpDir, "settings", "models.yml")},
		// {"ProgramDir", s.ProgramDir(), "/tmp/mindx-test/programs"},
		// {"DocumentDir", s.DocumentDir(), "/tmp/mindx-test/documents"},
		{"DataDir", s.DataDir(), filepath.Join(tmpDir, "data")},
		{"AgentsDir", s.AgentsDir(), filepath.Join(tmpDir, "agents")},
		{"RulesFile", s.RulesFile(), filepath.Join(tmpDir, "settings", "rules.yml")},
		{"SessionsDir", s.SessionsDir(), filepath.Join(tmpDir, "sessions")},
		{"SchedulesDir", s.SchedulesDir(), filepath.Join(tmpDir, "data", "schedules")},
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

	_ = os.MkdirAll(filepath.Join(tmpDir, "agents"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "settings"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "settings", "models.yml"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "settings", "rules.yml"), []byte{}, 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "sessions"), 0755)

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

func TestResolveModelName_FromAgentModel(t *testing.T) {
	// 构造一个包含模型的 ModelRegistry
	reg := NewTestModelRegistry("gpt-4", "claude-3")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{},
		logger:      logging.DefaultConsoleLogger(),
	}

	name, cfg, err := app.resolveModelName("gpt-4")
	if err != nil {
		t.Fatalf("resolveModelName(gpt-4) error: %v", err)
	}
	if name != "gpt-4" {
		t.Errorf("name = %q, want %q", name, "gpt-4")
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if cfg.Name != "gpt-4" {
		t.Errorf("cfg.Name = %q, want %q", cfg.Name, "gpt-4")
	}
}

func TestResolveModelName_LastModelOverride(t *testing.T) {
	reg := NewTestModelRegistry("gpt-4", "claude-3")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{LastModel: "claude-3"},
		logger:      logging.DefaultConsoleLogger(),
	}

	name, _, err := app.resolveModelName("gpt-4")
	if err != nil {
		t.Fatalf("resolveModelName error: %v", err)
	}
	if name != "claude-3" {
		t.Errorf("name = %q, want %q (LastModel should override)", name, "claude-3")
	}
}

func TestResolveModelName_DefaultModelFallback(t *testing.T) {
	reg := NewTestModelRegistry("gpt-4")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{DefaultModel: "gpt-4"},
		logger:      logging.DefaultConsoleLogger(),
	}

	name, _, err := app.resolveModelName("")
	if err != nil {
		t.Fatalf("resolveModelName('') error: %v", err)
	}
	if name != "gpt-4" {
		t.Errorf("name = %q, want %q", name, "gpt-4")
	}
}

func TestResolveModelName_NotFound(t *testing.T) {
	reg := NewTestModelRegistry("gpt-4")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{},
		logger:      logging.DefaultConsoleLogger(),
	}

	_, _, err := app.resolveModelName("nonexistent")
	if err == nil {
		t.Fatal("resolveModelName(nonexistent) should return error")
	}
}

func TestResolveModelName_Empty(t *testing.T) {
	app := &App{
		models:      NewTestModelRegistry(),
		mindxConfig: &MindxConfig{},
		logger:      logging.DefaultConsoleLogger(),
	}

	_, _, err := app.resolveModelName("")
	if err == nil {
		t.Fatal("resolveModelName('') with no models should return error")
	}
}

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		settings:    &Settings{Test: true, testDir: tmpDir},
		mindxConfig: DefaultMindxConfig(tmpDir),
		logger:      logging.DefaultConsoleLogger(),
		agents:      NewTestAgentRegistry(t, "test-agent"),
	}

	// 先初始化 sessDB
	if err := app.SetTestDir(tmpDir); err != nil {
		t.Fatalf("SetTestDir failed: %v", err)
	}

	sessionInfo, err := app.CreateSession("test-agent", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if sessionInfo == nil {
		t.Fatal("sessionInfo is nil")
	}
	if sessionInfo.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if sessionInfo.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want %q", sessionInfo.AgentName, "test-agent")
	}

	// 验证 currentSessionMeta 被设置
	if app.CurrentSessionMeta() == nil {
		t.Error("currentSessionMeta should be set after CreateSession")
	}
}

func TestIsModelAvailable_NilAgent(t *testing.T) {
	// 无 agents 注册时
	app := &App{
		models: NewTestModelRegistry("gpt-4"),
		agents: NewTestAgentRegistry(t),
		logger: logging.DefaultConsoleLogger(),
	}
	if app.IsModelAvailable() {
		t.Error("IsModelAvailable() should return false when no agent configured")
	}
}

func TestIsModelAvailable_SpecificName(t *testing.T) {
	app := &App{
		models:      NewTestModelRegistry("gpt-4"),
		mindxConfig: &MindxConfig{},
		logger:      logging.DefaultConsoleLogger(),
	}
	// 无实际网络请求，测试的是模型存在性检查前的逻辑
	// 这里验证的是 IsModelAvailable("nonexistent") 路径能正常返回 false
	if app.IsModelAvailable("nonexistent") {
		t.Error("IsModelAvailable(nonexistent) should return false")
	}
}

func TestSameDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	if !sameDirectory(tmpDir, tmpDir) {
		t.Error("sameDirectory should return true for same path")
	}
	// 大小写路径
	if !sameDirectory(tmpDir+"/", tmpDir) {
		t.Error("sameDirectory should handle trailing slash")
	}
}

func TestBuildDelegationGuidance(t *testing.T) {
	guidance := BuildDelegationGuidance()
	if guidance == "" {
		t.Error("BuildDelegationGuidance should not return empty")
	}
	if !contains(guidance, "SubAgent") {
		t.Error("BuildDelegationGuidance should mention SubAgent")
	}
}

func TestApp_Accessors_Nil(t *testing.T) {
	app := &App{}
	if app.CurrentAgentName() != "" {
		t.Error("CurrentAgentName() should return empty when no config")
	}
	if app.Embedder() != nil {
		t.Error("Embedder() should return nil")
	}
	if app.Config() != nil {
		t.Error("Config() should return nil")
	}
}
