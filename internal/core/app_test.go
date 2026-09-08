package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/rules"
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

// TestBuildRulesSection_MergesPermissionAndUserRules 回归测试：权限规则与用户规则
// 必须同时出现在扩展规则段中。原实现经 goharness 的 WithRuleRegistry 注入，
// 两次调用共用一个 ruleReg 槽位互相覆盖（后注册者胜出），导致另一类规则丢失。
func TestBuildRulesSection_MergesPermissionAndUserRules(t *testing.T) {
	app := &App{}

	// 权限规则（mindx.json 持久化存储）
	cfg := &MindxConfig{
		PermissionRules: &rules.PermissionRules{
			AlwaysAllow: []rules.PermissionRule{{Behavior: rules.RuleAllow, ToolName: "Bash", Description: "运行 go build"}},
			AlwaysDeny:  []rules.PermissionRule{{Behavior: rules.RuleDeny, ToolName: "Bash", Description: "执行 rm -rf"}},
			AlwaysAsk:   []rules.PermissionRule{{Behavior: rules.RuleAsk, ToolName: "WebFetch", Description: "访问外部网址"}},
		},
	}
	app.permissionRuleStore = NewMindxPermissionRuleStore(cfg)

	// 用户规则（rules.yml 注册表）
	userRules := rules.NewMemRuleRegistry()
	if err := userRules.Register(rules.Rule{ID: "no-force-push", Intro: "禁止强推主分支", Scope: rules.ScopeGlobal, Enabled: true}); err != nil {
		t.Fatalf("注册用户规则失败: %v", err)
	}
	app.rules = userRules

	got := app.BuildRulesSection()

	// 四类内容必须同时出现
	for _, want := range []string{
		"Always allow 运行 go build",
		"Always deny 执行 rm -rf",
		"Ask before 访问外部网址",
		agentDiscoveryIntro,
		"禁止强推主分支",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildRulesSection() 缺少 %q\n实际输出:\n%s", want, got)
		}
	}
}

// TestBuildRulesSection_DisabledUserRuleSkipped 已禁用的用户规则不得进入提示词。
func TestBuildRulesSection_DisabledUserRuleSkipped(t *testing.T) {
	app := &App{}
	userRules := rules.NewMemRuleRegistry()
	if err := userRules.Register(rules.Rule{ID: "off", Intro: "已禁用规则", Scope: rules.ScopeGlobal, Enabled: false}); err != nil {
		t.Fatalf("注册用户规则失败: %v", err)
	}
	app.rules = userRules

	got := app.BuildRulesSection()
	if strings.Contains(got, "已禁用规则") {
		t.Errorf("已禁用规则不应渲染进提示词\n实际输出:\n%s", got)
	}
}

func TestResolveModelName_LastModelPriority(t *testing.T) {
	// 模型选择为用户级语义：LastModel 优先于 DefaultModel（Agent 不持有模型属性）
	reg := NewTestModelRegistry("gpt-4", "claude-3")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{LastModel: "claude-3", DefaultModel: "gpt-4"},
		logger:      logging.DefaultConsoleLogger(),
	}

	name, cfg, err := app.resolveModelName()
	if err != nil {
		t.Fatalf("resolveModelName error: %v", err)
	}
	if name != "claude-3" {
		t.Errorf("name = %q, want %q (LastModel 优先)", name, "claude-3")
	}
	if cfg == nil || cfg.Name != "claude-3" {
		t.Fatalf("cfg.Name = %v, want %q", cfg, "claude-3")
	}
}

func TestResolveModelName_DefaultModelFallback(t *testing.T) {
	reg := NewTestModelRegistry("gpt-4")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{DefaultModel: "gpt-4"},
		logger:      logging.DefaultConsoleLogger(),
	}

	name, _, err := app.resolveModelName()
	if err != nil {
		t.Fatalf("resolveModelName error: %v", err)
	}
	if name != "gpt-4" {
		t.Errorf("name = %q, want %q", name, "gpt-4")
	}
}

func TestResolveModelName_NotFound(t *testing.T) {
	reg := NewTestModelRegistry("gpt-4")
	app := &App{
		models:      reg,
		mindxConfig: &MindxConfig{LastModel: "nonexistent"},
		logger:      logging.DefaultConsoleLogger(),
	}

	_, _, err := app.resolveModelName()
	if err == nil {
		t.Fatal("resolveModelName 对未注册模型应返回错误")
	}
}

func TestResolveModelName_Empty(t *testing.T) {
	app := &App{
		models:      NewTestModelRegistry(),
		mindxConfig: &MindxConfig{},
		logger:      logging.DefaultConsoleLogger(),
	}

	_, _, err := app.resolveModelName()
	if err == nil {
		t.Fatal("未配置任何模型时应返回错误")
	}
}

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		settings:    &Settings{Test: true, testDir: tmpDir},
		mindxConfig: DefaultMindxConfig(tmpDir),
		logger:      logging.DefaultConsoleLogger(),
		agents:      NewTestAgentStore(t, "test-agent"),
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
	if !contains(guidance, "Execution") {
		t.Error("BuildDelegationGuidance should mention Execution")
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
