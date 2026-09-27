package core

import (
	"path/filepath"
)

type Settings struct {
	Test    bool
	testDir string
}

func (s *Settings) UserPreferences() string {
	if s.Test && s.testDir != "" {
		return s.testDir
	}
	if s.Test {
		return "./tmp/mindx-test"
	}
	return DefaultUserPrefsDir()
}
func (s *Settings) SkillsDir() string {
	return filepath.Join(s.UserPreferences(), "skills")
}

func (s *Settings) ModelsFile() string {
	return filepath.Join(s.UserPreferences(), "settings", "models.yml")
}

func (s *Settings) ProvidersFile() string {
	return filepath.Join(s.UserPreferences(), "settings", "providers.yml")
}

// func (s *Settings) ProgramDir() string {
// 	return filepath.Join(s.UserPreferences(), "programs")
// }

// func (s *Settings) DocumentDir() string {
// 	return filepath.Join(s.UserPreferences, "documents")
// }

func (s *Settings) DataDir() string {
	return filepath.Join(s.UserPreferences(), "data")
}

func (s *Settings) AgentsDir() string {
	return filepath.Join(s.UserPreferences(), "agents")
}

func (s *Settings) RulesFile() string {
	return filepath.Join(s.UserPreferences(), "settings", "rules.yml")
}

// McpFile 返回 MCP 配置文件路径 ~/.mindx/settings/mcp.json。
// 文件不存在时由调用方决定如何处理（首次启动无配置属正常情况）。
func (s *Settings) McpFile() string {
	return filepath.Join(s.UserPreferences(), "settings", "mcp.json")
}

// DataRulesFile returns the path for the persistent rule store used at runtime
// by the FileRuleRegistry (CRUD via JSON-RPC). Default: ~/.mindx/data/rules.yml.
func (s *Settings) DataRulesFile() string {
	return filepath.Join(s.DataDir(), "rules.yml")
}

func (s *Settings) SessionsDir() string {
	return filepath.Join(s.UserPreferences(), "sessions")
}

// SessionDirsFile 返回工作目录清单文件路径（<DataDir>/session_dirs.json）。
// RoutedSessionStore 据此发现各工作目录下的 .sessions 分片；清单独立于
// ~/.mindx/sessions（迁移源目录），避免搬迁完成后旧目录更名 .bak 时把清单带走。
func (s *Settings) SessionDirsFile() string {
	return filepath.Join(s.DataDir(), "session_dirs.json")
}

func (s *Settings) SchedulesDir() string {
	return filepath.Join(s.DataDir(), "schedules")
}

func (s *Settings) VenvDir() string {
	return filepath.Join(s.UserPreferences(), ".venv")
}

func (s *Settings) LogsDir() string {
	return filepath.Join(s.UserPreferences(), "logs")
}
