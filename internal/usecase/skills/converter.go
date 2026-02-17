package skills

import (
	"fmt"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type SkillConverter struct {
	skillsDir  string
	logger     logging.Logger
	mu         sync.RWMutex
	skillInfos map[string]*entity.SkillInfo
}

func NewSkillConverter(skillsDir string, logger logging.Logger) *SkillConverter {
	return &SkillConverter{
		skillsDir:  skillsDir,
		logger:     logger.Named("SkillConverter"),
		skillInfos: make(map[string]*entity.SkillInfo),
	}
}

func (c *SkillConverter) SetSkillInfos(infos map[string]*entity.SkillInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.skillInfos = infos
}

func (c *SkillConverter) Convert(name string) error {
	c.mu.RLock()
	info, exists := c.skillInfos[name]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("技能不存在: %s", name)
	}

	c.logger.Info(i18n.T("skill.start_convert"), logging.String("name", name))

	skillPath := filepath.Join(c.skillsDir, name)
	skillFile := filepath.Join(skillPath, "SKILL.md")

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("读取技能文件失败: %w", err)
	}

	content := string(data)

	if !strings.HasPrefix(content, "---") {
		return fmt.Errorf("技能格式无效：缺少 YAML frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fmt.Errorf("技能 frontmatter 格式无效")
	}

	yamlContent := strings.TrimSpace(parts[1])
	markdownContent := strings.TrimSpace(parts[2])

	var def entity.SkillDef
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return fmt.Errorf("解析 YAML 失败: %w", err)
	}

	if def.Name == "" {
		def.Name = name
	}
	if def.Version == "" {
		def.Version = "1.0.0"
	}
	if def.Category == "" {
		def.Category = "general"
	}
	def.Enabled = true

	updatedYAML, err := yaml.Marshal(&def)
	if err != nil {
		return fmt.Errorf("序列化 YAML 失败: %w", err)
	}

	newContent := fmt.Sprintf("---\n%s---\n\n%s", string(updatedYAML), markdownContent)

	if err := os.WriteFile(skillFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("写入技能文件失败: %w", err)
	}

	c.mu.Lock()
	info.Def = &def
	c.skillInfos[name] = info
	c.mu.Unlock()

	c.logger.Info(i18n.T("skill.convert_complete"), logging.String("name", name))
	return nil
}

func (c *SkillConverter) BatchConvert(names []string) (success []string, failed map[string]string) {
	success = make([]string, 0)
	failed = make(map[string]string)

	for _, name := range names {
		if err := c.Convert(name); err != nil {
			failed[name] = err.Error()
			c.logger.Warn(i18n.T("skill.batch_convert_failed"), logging.String("name", name), logging.Err(err))
		} else {
			success = append(success, name)
		}
	}

	return success, failed
}
