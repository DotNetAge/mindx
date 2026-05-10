package core

import "path/filepath"

type Settings struct {
	Workspace   string
	Path        string
	MasterAgent string
}

func (s *Settings) SkillsDir() string {
	return filepath.Join(s.Workspace, "skills")
}

func (s *Settings) ModelsFile() string {
	return filepath.Join(s.Workspace, "settings", "models.yml")
}

func (s *Settings) ProgramDir() string {
	return filepath.Join(s.Workspace, "programs")
}

func (s *Settings) DocumentDir() string {
	return filepath.Join(s.Workspace, "documents")
}

func (s *Settings) DataDir() string {
	return filepath.Join(s.Workspace, "data")
}

func (s *Settings) AgentsDir() string {
	return filepath.Join(s.Workspace, "agents")
}

func (s *Settings) RulesFile() string {
	return filepath.Join(s.Workspace, "settings", "rules.yml")
}

func (s *Settings) SessionsDir() string {
	return filepath.Join(s.Workspace, "sessions")
}

func (s *Settings) SchedulesDir() string {
	return filepath.Join(s.DataDir(), "schedules")
}
