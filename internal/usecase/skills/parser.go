package skills

import (
	"bufio"
	"bytes"
	"fmt"
	"mindx/internal/entity"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillParser SKILL.md 解析器
// 解析符合 agentskills.io 规范的 SKILL.md 文件
type SkillParser struct {
}

// NewSkillParser 创建解析器
func NewSkillParser() *SkillParser {
	return &SkillParser{}
}

// Parse 解析 SKILL.md 文件
func (p *SkillParser) Parse(filePath string) (*entity.Skill, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	skill, err := p.ParseContent(content)
	if err != nil {
		return nil, err
	}

	skill.FilePath = filePath
	return skill, nil
}

// ParseContent 解析 SKILL.md 内容
func (p *SkillParser) ParseContent(content []byte) (*entity.Skill, error) {
	// 1. 分离 YAML frontmatter 和 Markdown 内容
	frontmatter, markdown, err := p.splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// 2. 解析 YAML frontmatter
	skill, err := p.parseFrontmatter(frontmatter)
	if err != nil {
		return nil, err
	}

	// 3. 解析 Markdown 内容
	if err := p.parseMarkdown(skill, markdown); err != nil {
		return nil, err
	}

	// 4. 提取关键词
	p.extractKeywords(skill)

	return skill, nil
}

// splitFrontmatter 分离 YAML frontmatter 和 Markdown 内容
func (p *SkillParser) splitFrontmatter(content []byte) ([]byte, []byte, error) {
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, nil, fmt.Errorf("missing YAML frontmatter")
	}

	// 跳过第一个 ---
	content = content[4:]

	// 查找第二个 ---
	parts := bytes.SplitN(content, []byte("---"), 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid YAML frontmatter format")
	}

	frontmatter := bytes.TrimSpace(parts[0])
	markdown := bytes.TrimSpace(parts[1])

	return frontmatter, markdown, nil
}

// parseFrontmatter 解析 YAML frontmatter
func (p *SkillParser) parseFrontmatter(data []byte) (*entity.Skill, error) {
	var meta struct {
		Name          string   `yaml:"name"`
		Description   string   `yaml:"description"`
		Version       string   `yaml:"version"`
		Author        string   `yaml:"author"`
		Tags          []string `yaml:"tags"`
		RequiredTools []string `yaml:"required_tools"`
		OptionalTools []string `yaml:"optional_tools"`
	}

	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// 验证必需字段
	if meta.Name == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	if meta.Description == "" {
		return nil, fmt.Errorf("missing required field: description")
	}
	if meta.Version == "" {
		return nil, fmt.Errorf("missing required field: version")
	}

	skill := &entity.Skill{
		Name:          meta.Name,
		Description:   meta.Description,
		Version:       meta.Version,
		Author:        meta.Author,
		Tags:          meta.Tags,
		RequiredTools: meta.RequiredTools,
		OptionalTools: meta.OptionalTools,
	}

	return skill, nil
}

// parseMarkdown 解析 Markdown 内容
func (p *SkillParser) parseMarkdown(skill *entity.Skill, markdown []byte) error {
	sections := p.extractSections(markdown)

	// 解析 Goal
	if goal, ok := sections["goal"]; ok {
		skill.Goal = strings.TrimSpace(goal)
	}

	// 解析 Triggers
	if triggers, ok := sections["triggers"]; ok {
		skill.Triggers = p.parseList(triggers)
	}

	// 解析 SOP
	if sop, ok := sections["sop"]; ok {
		skill.SOP = strings.TrimSpace(sop)
	}

	// 解析 Examples
	if examples, ok := sections["examples"]; ok {
		skill.Examples = p.parseExamples(examples)
	}

	return nil
}

// extractSections 提取 Markdown 章节
func (p *SkillParser) extractSections(markdown []byte) map[string]string {
	sections := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(markdown))

	var currentSection string
	var currentContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// 检查是否是一级标题
		if strings.HasPrefix(line, "# ") {
			// 保存上一个章节
			if currentSection != "" {
				sections[currentSection] = currentContent.String()
			}

			// 开始新章节
			currentSection = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
			currentContent.Reset()
		} else if currentSection != "" {
			// 添加内容到当前章节
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// 保存最后一个章节
	if currentSection != "" {
		sections[currentSection] = currentContent.String()
	}

	return sections
}

// parseList 解析列表（用于 Triggers）
func (p *SkillParser) parseList(content string) []string {
	var items []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 移除列表标记（- 或 * 或数字.）
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		// 移除数字列表标记（如 "1. "）
		if idx := strings.Index(line, ". "); idx > 0 && idx < 5 {
			line = line[idx+2:]
		}

		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}

	return items
}

// parseExamples 解析示例（用于 Examples）
func (p *SkillParser) parseExamples(content string) []string {
	var examples []string
	var currentExample strings.Builder
	inExample := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// 检查是否是新示例开始（**场景**: 或空行后的 **用户**:）
		if strings.HasPrefix(line, "**场景") {
			// 保存上一个示例
			if inExample && currentExample.Len() > 0 {
				examples = append(examples, strings.TrimSpace(currentExample.String()))
				currentExample.Reset()
			}
			inExample = true
			currentExample.WriteString(line)
			currentExample.WriteString("\n")
		} else if strings.HasPrefix(line, "**用户**:") {
			// 如果前面有内容且遇到空行，说明是新示例
			if inExample && currentExample.Len() > 0 {
				// 检查是否应该开始新示例（通过检查前面是否有空行）
				trimmed := strings.TrimSpace(currentExample.String())
				if strings.HasSuffix(trimmed, "\n") || !inExample {
					examples = append(examples, trimmed)
					currentExample.Reset()
				}
			}
			inExample = true
			currentExample.WriteString(line)
			currentExample.WriteString("\n")
		} else if inExample {
			// 检查是否是空行分隔符
			if strings.TrimSpace(line) == "" && currentExample.Len() > 0 {
				// 可能是示例之间的分隔
				currentExample.WriteString("\n")
			} else {
				currentExample.WriteString(line)
				currentExample.WriteString("\n")
			}
		}
	}

	// 保存最后一个示例
	if currentExample.Len() > 0 {
		examples = append(examples, strings.TrimSpace(currentExample.String()))
	}

	return examples
}

// extractKeywords 提取关键词
func (p *SkillParser) extractKeywords(skill *entity.Skill) {
	keywordSet := make(map[string]bool)

	// 从 Tags 提取（保持原样，不分词）
	for _, tag := range skill.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			keywordSet[tag] = true
		}
	}

	// 从 Name 提取（分词）
	nameWords := p.tokenize(skill.Name)
	for _, word := range nameWords {
		if word != "" {
			keywordSet[word] = true
		}
	}

	// 从 Description 提取（分词）
	descWords := p.tokenize(skill.Description)
	for _, word := range descWords {
		if word != "" {
			keywordSet[word] = true
		}
	}

	// 从 Triggers 提取（分词）
	for _, trigger := range skill.Triggers {
		triggerWords := p.tokenize(trigger)
		for _, word := range triggerWords {
			if word != "" {
				keywordSet[word] = true
			}
		}
	}

	// 转换为列表
	keywords := make([]string, 0, len(keywordSet))
	for keyword := range keywordSet {
		keywords = append(keywords, keyword)
	}

	skill.Keywords = keywords
}

// tokenize 分词（简单实现）
func (p *SkillParser) tokenize(text string) []string {
	text = strings.ToLower(text)

	// 替换分隔符为空格
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"/", " ",
		"、", " ",
		"，", " ",
		"。", " ",
	)
	text = replacer.Replace(text)

	// 分割并过滤
	words := strings.Fields(text)
	result := make([]string, 0, len(words))

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 1 { // 忽略单字符
			result = append(result, word)
		}
	}

	return result
}
