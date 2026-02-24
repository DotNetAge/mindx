package skills

import (
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type SkillLoader struct {
	skillsDir  string
	logger     logging.Logger
	mu         sync.RWMutex
	skills     map[string]*core.Skill
	skillInfos map[string]*entity.SkillInfo
}

func NewSkillLoader(skillsDir string, logger logging.Logger) *SkillLoader {
	return &SkillLoader{
		skillsDir:  skillsDir,
		logger:     logger.Named("SkillLoader"),
		skills:     make(map[string]*core.Skill),
		skillInfos: make(map[string]*entity.SkillInfo),
	}
}

func (l *SkillLoader) LoadAll() error {
	l.logger.Info(i18n.T("skill.start_loading"), logging.String(i18n.T("skill.dir"), l.skillsDir))

	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return fmt.Errorf("failed to read dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(l.skillsDir, skillName)

		if err := l.Load(skillName, skillPath); err != nil {
			l.logger.Warn(i18n.T("skill.load_skill_failed"), logging.String(i18n.T("skill.skill"), skillName), logging.Err(err))
			continue
		}
	}

	return nil
}

func (l *SkillLoader) Load(name, path string) error {
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	def, err := ParseSkillDef(data)
	if err != nil {
		return fmt.Errorf("failed to parse skill def: %w", err)
	}

	if !def.Enabled {
		l.logger.Debug(i18n.T("skill.disabled_skip"), logging.String(i18n.T("skill.name"), name))
		return nil
	}

	missingBins, missingEnv := CheckDependencies(def)
	canRun := len(missingBins) == 0 && len(missingEnv) == 0

	skillName := name
	skill := &core.Skill{
		GetName: func() string {
			return skillName
		},
		Execute: func(skillNameParam string, params map[string]any) error {
			return nil
		},
	}

	format := "standard"
	if def.Metadata != nil {
		if _, hasOpenclaw := def.Metadata["openclaw"]; hasOpenclaw {
			format = "external"
		}
	}

	status := "ready"
	if !def.Enabled {
		status = "disabled"
	} else if !canRun {
		status = "error"
	}

	info := &entity.SkillInfo{
		Def:          def,
		Directory:    path,
		Content:      string(data),
		CanRun:       canRun,
		MissingBins:  missingBins,
		MissingEnv:   missingEnv,
		Format:       format,
		Status:       status,
		SuccessCount: 0,
		ErrorCount:   0,
	}

	l.mu.Lock()
	l.skills[name] = skill
	l.skillInfos[name] = info
	l.mu.Unlock()

	l.logger.Info(i18n.T("skill.load_skill_success"), logging.String(i18n.T("skill.name"), name), logging.String(i18n.T("skill.description"), def.Description))
	return nil
}

func (l *SkillLoader) GetSkills() map[string]*core.Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]*core.Skill, len(l.skills))
	for k, v := range l.skills {
		result[k] = v
	}
	return result
}

func (l *SkillLoader) GetSkillInfos() map[string]*entity.SkillInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]*entity.SkillInfo, len(l.skillInfos))
	for k, v := range l.skillInfos {
		result[k] = v
	}
	return result
}

func (l *SkillLoader) GetSkill(name string) (*core.Skill, *entity.SkillInfo, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skill, exists := l.skills[name]
	if !exists {
		return nil, nil, false
	}
	info := l.skillInfos[name]
	return skill, info, true
}

func (l *SkillLoader) UpdateSkillInfo(name string, info *entity.SkillInfo) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.skillInfos[name] = info
}

func ParseSkillDef(data []byte) (*entity.SkillDef, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("invalid skill format")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter")
	}

	yamlContent := strings.TrimSpace(parts[1])

	var def entity.SkillDef
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	return &def, nil
}

func CheckDependencies(def *entity.SkillDef) ([]string, []string) {
	var missingBins, missingEnv []string

	if def.Requires != nil {
		for _, bin := range def.Requires.Bins {
			if _, err := exec.LookPath(bin); err != nil {
				missingBins = append(missingBins, bin)
			}
		}

		for _, env := range def.Requires.Env {
			if os.Getenv(env) == "" {
				missingEnv = append(missingEnv, env)
			}
		}
	}

	return missingBins, missingEnv
}

// RegisterMCPSkills 将 MCP server 发现的工具注册为 skill
func (l *SkillLoader) RegisterMCPSkills(serverName string, defs []*entity.SkillDef) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, def := range defs {
		skillName := def.Name
		skill := &core.Skill{
			GetName: func() string { return skillName },
			Execute: func(name string, params map[string]any) error { return nil },
		}

		info := &entity.SkillInfo{
			Def:    def,
			CanRun: true,
			Format: "mcp",
			Status: "ready",
		}

		l.skills[skillName] = skill
		l.skillInfos[skillName] = info
		l.logger.Info("注册 MCP 工具",
			logging.String("skill", skillName),
			logging.String("server", serverName))
	}
}

// UnregisterMCPSkills 移除某 MCP server 的所有 skill
func (l *SkillLoader) UnregisterMCPSkills(serverName string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := fmt.Sprintf("mcp_%s_", serverName)
	for name := range l.skills {
		if strings.HasPrefix(name, prefix) {
			delete(l.skills, name)
			delete(l.skillInfos, name)
			l.logger.Info("注销 MCP 工具",
				logging.String("skill", name),
				logging.String("server", serverName))
		}
	}
}
