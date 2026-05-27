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

func (s *Settings) SessionsDir() string {
	return filepath.Join(s.UserPreferences(), "sessions")
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
