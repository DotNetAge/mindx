package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OldSkillMeta 旧的 SKILL.md 元数据
type OldSkillMeta struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Version     string                 `yaml:"version"`
	Category    string                 `yaml:"category"`
	Tags        []string               `yaml:"tags"`
	OS          []string               `yaml:"os"`
	Enabled     bool                   `yaml:"enabled"`
	Timeout     int                    `yaml:"timeout"`
	Command     string                 `yaml:"command"`
	Parameters  map[string]interface{} `yaml:"parameters"`
	Requires    *Requires              `yaml:"requires,omitempty"`
	Homepage    string                 `yaml:"homepage,omitempty"`
	IsInternal  bool                   `yaml:"is_internal,omitempty"`
	Guidance    string                 `yaml:"guidance,omitempty"`
}

// Requires 依赖定义
type Requires struct {
	Bins []string `yaml:"bins,omitempty"`
	Env  []string `yaml:"env,omitempty"`
}

// NewSkillMeta 新的 SKILL.md 元数据
type NewSkillMeta struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Version       string   `yaml:"version"`
	Author        string   `yaml:"author"`
	Tags          []string `yaml:"tags"`
	RequiredTools []string `yaml:"required_tools,omitempty"`
	OptionalTools []string `yaml:"optional_tools,omitempty"`
}

// MigrationResult 迁移结果
type MigrationResult struct {
	SkillName string
	OldPath   string
	NewPath   string
	Status    string // success, failed, skipped
	Error     string
	Changes   []string
}

// SkillMigrator 技能迁移器
type SkillMigrator struct {
	inputDir  string
	outputDir string
	dryRun    bool
	results   []*MigrationResult
}

// NewSkillMigrator 创建迁移器
func NewSkillMigrator(inputDir, outputDir string, dryRun bool) *SkillMigrator {
	return &SkillMigrator{
		inputDir:  inputDir,
		outputDir: outputDir,
		dryRun:    dryRun,
		results:   make([]*MigrationResult, 0),
	}
}

// Migrate 迁移单个 Skill
func (m *SkillMigrator) Migrate(skillPath string) *MigrationResult {
	result := &MigrationResult{
		OldPath: skillPath,
		Changes: make([]string, 0),
	}

	// 1. 读取旧文件
	content, err := os.ReadFile(skillPath)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		return result
	}

	// 2. 解析旧格式
	oldMeta, oldContent, err := m.parseOldFormat(content)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("failed to parse old format: %v", err)
		return result
	}

	result.SkillName = oldMeta.Name

	// 3. 转换为新格式
	newMeta, newContent := m.convertToNewFormat(oldMeta, oldContent)

	// 4. 生成新文件内容
	newFileContent := m.generateNewFile(newMeta, newContent)

	// 5. 确定输出路径
	skillDir := filepath.Dir(skillPath)
	skillName := filepath.Base(skillDir)

	var newPath string
	if m.outputDir != "" {
		newPath = filepath.Join(m.outputDir, skillName, "SKILL.md")
	} else {
		newPath = filepath.Join(skillDir, "SKILL.new.md")
	}
	result.NewPath = newPath

	// 6. 写入新文件（如果不是 dry-run）
	if !m.dryRun {
		// 创建目录
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("failed to create directory: %v", err)
			return result
		}

		// 写入文件
		if err := os.WriteFile(newPath, []byte(newFileContent), 0644); err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("failed to write file: %v", err)
			return result
		}
	}

	result.Status = "success"
	return result
}

// parseOldFormat 解析旧格式
func (m *SkillMigrator) parseOldFormat(content []byte) (*OldSkillMeta, string, error) {
	// 分离 frontmatter 和内容
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("invalid format: missing frontmatter")
	}

	// 解析 YAML
	var meta OldSkillMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// 提取内容
	oldContent := strings.TrimSpace(parts[2])

	return &meta, oldContent, nil
}

// convertToNewFormat 转换为新格式
func (m *SkillMigrator) convertToNewFormat(old *OldSkillMeta, oldContent string) (*NewSkillMeta, string) {
	// 1. 转换元数据
	newMeta := &NewSkillMeta{
		Name:        old.Name,
		Description: m.convertDescription(old.Description),
		Version:     old.Version,
		Author:      "mindx",
		Tags:        m.convertTags(old),
	}

	// 2. 提取工具依赖
	if old.Command != "" {
		// 如果有 command，说明这是一个工具，不是 Skill
		// 将其作为 required_tools
		newMeta.RequiredTools = []string{old.Name}
	}

	// 3. 生成新内容
	newContent := m.generateNewContent(old, oldContent)

	return newMeta, newContent
}

// convertDescription 转换描述
func (m *SkillMigrator) convertDescription(desc string) string {
	// 如果描述不包含"标准操作程序"，添加它
	if !strings.Contains(desc, "标准操作程序") && !strings.Contains(desc, "SOP") {
		return desc + "的标准操作程序"
	}
	return desc
}

// convertTags 转换标签
func (m *SkillMigrator) convertTags(old *OldSkillMeta) []string {
	tags := make([]string, 0)

	// 保留原有 tags
	tags = append(tags, old.Tags...)

	// 添加 category 作为 tag
	if old.Category != "" {
		tags = append(tags, old.Category)
	}

	return tags
}

// generateNewContent 生成新内容
func (m *SkillMigrator) generateNewContent(old *OldSkillMeta, oldContent string) string {
	var sb strings.Builder

	// Goal
	sb.WriteString("# Goal\n\n")
	sb.WriteString(m.generateGoal(old))
	sb.WriteString("\n\n")

	// Triggers
	sb.WriteString("# Triggers\n\n")
	sb.WriteString(m.generateTriggers(old))
	sb.WriteString("\n\n")

	// SOP
	sb.WriteString("# SOP\n\n")
	sb.WriteString(m.generateSOP(old, oldContent))
	sb.WriteString("\n\n")

	// Examples
	sb.WriteString("# Examples\n\n")
	sb.WriteString(m.generateExamples(old))
	sb.WriteString("\n")

	return sb.String()
}

// generateGoal 生成 Goal
func (m *SkillMigrator) generateGoal(old *OldSkillMeta) string {
	// 从 description 提取目标
	goal := strings.TrimSuffix(old.Description, "的标准操作程序")
	goal = strings.TrimSuffix(goal, "技能")
	goal = strings.TrimPrefix(goal, "技能，")

	return goal
}

// generateTriggers 生成 Triggers
func (m *SkillMigrator) generateTriggers(old *OldSkillMeta) string {
	var sb strings.Builder

	// 从 guidance 提取
	if old.Guidance != "" {
		lines := strings.Split(old.Guidance, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString("- ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}

	// 如果没有 guidance，生成默认的
	if sb.Len() == 0 {
		sb.WriteString("- 用户要求使用 ")
		sb.WriteString(old.Name)
		sb.WriteString("\n")

		// 从 tags 生成
		for _, tag := range old.Tags {
			if len(tag) > 1 {
				sb.WriteString("- 用户提到\"")
				sb.WriteString(tag)
				sb.WriteString("\"\n")
			}
		}
	}

	return sb.String()
}

// generateSOP 生成 SOP
func (m *SkillMigrator) generateSOP(old *OldSkillMeta, oldContent string) string {
	var sb strings.Builder

	// 如果有 command，说明这是一个工具调用
	if old.Command != "" {
		sb.WriteString("1. 解析用户输入，提取参数\n")
		sb.WriteString("2. 调用 ")
		sb.WriteString(old.Name)
		sb.WriteString(" 工具\n")
		sb.WriteString("3. 处理返回结果\n")
		sb.WriteString("4. 生成友好的响应\n")
	} else {
		// 尝试从旧内容提取
		if oldContent != "" {
			sb.WriteString(oldContent)
		} else {
			sb.WriteString("1. 理解用户需求\n")
			sb.WriteString("2. 执行相应操作\n")
			sb.WriteString("3. 返回结果\n")
		}
	}

	return sb.String()
}

// generateExamples 生成 Examples
func (m *SkillMigrator) generateExamples(old *OldSkillMeta) string {
	var sb strings.Builder

	sb.WriteString("**用户**: 请使用 ")
	sb.WriteString(old.Name)
	sb.WriteString("\n")
	sb.WriteString("**助手**: 好的，我来帮你处理。\n")

	return sb.String()
}

// generateNewFile 生成新文件内容
func (m *SkillMigrator) generateNewFile(meta *NewSkillMeta, content string) string {
	var sb strings.Builder

	// YAML frontmatter
	sb.WriteString("---\n")

	yamlData, _ := yaml.Marshal(meta)
	sb.Write(yamlData)

	sb.WriteString("---\n\n")

	// Markdown content
	sb.WriteString(content)

	return sb.String()
}

// MigrateAll 迁移所有 Skills
func (m *SkillMigrator) MigrateAll() error {
	// 查找所有 SKILL.md 文件
	skillFiles, err := m.findSkillFiles()
	if err != nil {
		return fmt.Errorf("failed to find skill files: %w", err)
	}

	fmt.Printf("Found %d SKILL.md files\n", len(skillFiles))

	// 迁移每个文件
	for i, skillFile := range skillFiles {
		fmt.Printf("[%d/%d] Migrating %s...\n", i+1, len(skillFiles), skillFile)

		result := m.Migrate(skillFile)
		m.results = append(m.results, result)

		if result.Status == "success" {
			fmt.Printf("  ✓ Success: %s\n", result.NewPath)
		} else {
			fmt.Printf("  ✗ Failed: %s\n", result.Error)
		}
	}

	return nil
}

// findSkillFiles 查找所有 SKILL.md 文件
func (m *SkillMigrator) findSkillFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(m.inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == "SKILL.md" {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// GenerateReport 生成迁移报告
func (m *SkillMigrator) GenerateReport() string {
	var sb strings.Builder

	sb.WriteString("# SKILL.md 迁移报告\n\n")
	sb.WriteString(fmt.Sprintf("> 迁移时间：%s\n\n", "2026-03-06"))
	sb.WriteString("---\n\n")

	// 统计
	total := len(m.results)
	success := 0
	failed := 0

	for _, r := range m.results {
		if r.Status == "success" {
			success++
		} else {
			failed++
		}
	}

	sb.WriteString("## 📊 迁移统计\n\n")
	sb.WriteString(fmt.Sprintf("- 总计：%d 个 Skills\n", total))
	sb.WriteString(fmt.Sprintf("- 成功：%d 个 (%.1f%%)\n", success, float64(success)/float64(total)*100))
	sb.WriteString(fmt.Sprintf("- 失败：%d 个 (%.1f%%)\n\n", failed, float64(failed)/float64(total)*100))

	// 成功列表
	if success > 0 {
		sb.WriteString("## ✅ 成功迁移\n\n")
		for _, r := range m.results {
			if r.Status == "success" {
				sb.WriteString(fmt.Sprintf("- %s\n", r.SkillName))
			}
		}
		sb.WriteString("\n")
	}

	// 失败列表
	if failed > 0 {
		sb.WriteString("## ❌ 迁移失败\n\n")
		for _, r := range m.results {
			if r.Status == "failed" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", r.SkillName, r.Error))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func main() {
	inputDir := flag.String("input", "skills", "输入目录")
	outputDir := flag.String("output", "skills.new", "输出目录")
	dryRun := flag.Bool("dry-run", false, "只检查，不实际迁移")
	report := flag.Bool("report", false, "生成迁移报告")

	flag.Parse()

	migrator := NewSkillMigrator(*inputDir, *outputDir, *dryRun)

	if *report {
		// 生成报告
		reportContent := migrator.GenerateReport()
		fmt.Println(reportContent)
		return
	}

	// 执行迁移
	if err := migrator.MigrateAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}

	// 输出报告
	reportContent := migrator.GenerateReport()

	// 保存报告
	reportPath := filepath.Join(*outputDir, "MIGRATION-REPORT.md")
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write report: %v\n", err)
	}

	fmt.Printf("\n迁移完成！报告已保存到：%s\n", reportPath)
}
