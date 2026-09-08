package agentstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrateLegacyFiles 将旧单文件 Agent（agents/{name}.md）一次性迁移为目录格式。
//
// 迁移规则：
//   - 仅处理顶层 *.md 文件（忽略 .bak 备份与目录）；
//   - meta 块中的 icon / domains / hired 提升为 frontmatter 一级字段，
//     其余 meta 键原样保留在 Meta map；
//   - 旧文件正文（行为规则）迁入 SOUL.md（自带标题，原样保留）；
//     IDENTITY.md 正文留空，角色定义由 name/role/description 兜底生成；
//   - 迁移成功后原文件改名为 {name}.md.bak（备份，不删除）；
//   - 同名目录已存在时视为已迁移：旧文件仅做备份，不覆盖目录。
//
// 返回迁移报告：旧文件名 → 错误（nil 表示成功）。
func MigrateLegacyFiles(agentsDir string) map[string]error {
	report := make(map[string]error)

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return report
		}
		report["*"] = fmt.Errorf("读取 agents 目录失败: %w", err)
		return report
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		legacyPath := filepath.Join(agentsDir, entry.Name())
		err := migrateOne(agentsDir, legacyPath)
		report[entry.Name()] = err
	}
	return report
}

// migrateOne 迁移单个旧单文件。返回 nil 表示迁移成功。
func migrateOne(agentsDir, legacyPath string) error {
	base := strings.TrimSuffix(filepath.Base(legacyPath), ".md")
	if strings.TrimSpace(base) == "" {
		return fmt.Errorf("文件名无效: %s", legacyPath)
	}

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return fmt.Errorf("读取旧文件失败: %w", err)
	}

	front, body, err := splitFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	var raw struct {
		Name         string         `yaml:"name"`
		Role         string         `yaml:"role"`
		Description  string         `yaml:"description"`
		Skills       []string       `yaml:"skills"`
		ExcludeTools []string       `yaml:"exclude_tools"`
		Meta         map[string]any `yaml:"meta"`
	}
	if err := yaml.Unmarshal([]byte(front), &raw); err != nil {
		return fmt.Errorf("解析 frontmatter YAML 失败: %w", err)
	}

	meta := AgentMeta{
		Name:         raw.Name,
		Role:         raw.Role,
		Description:  raw.Description,
		Skills:       raw.Skills,
		ExcludeTools: raw.ExcludeTools,
	}
	if meta.Name == "" {
		meta.Name = base
	}
	// 旧单文件「正文即行为规则」：正文迁入 SOUL.md（自带标题，原样保留）；
	// IDENTITY.md 正文留空，角色定义由 name/role/description 兜底生成。
	soulBody := strings.TrimSpace(body)

	// meta 块字段提升：icon / category / hired 成为一级字段，其余留在 Meta；
	// 旧领域键 domains（单值语义）一次性迁移为 category（取首个非空值）
	meta.Meta = make(map[string]any, len(raw.Meta))
	for k, v := range raw.Meta {
		switch k {
		case "icon":
			meta.Icon, _ = v.(string)
		case "category":
			if s, ok := v.(string); ok {
				meta.Category = s
			}
		case "domains":
			if items, ok := v.([]any); ok {
				for _, it := range items {
					if s, ok := it.(string); ok {
						if s = strings.TrimSpace(s); s != "" && meta.Category == "" {
							meta.Category = s
						}
					}
				}
			}
		case "hired":
			switch h := v.(type) {
			case bool:
				meta.Hired = h
			case string:
				meta.Hired = strings.EqualFold(strings.TrimSpace(h), "true")
			}
		default:
			meta.Meta[k] = v
		}
	}
	if len(meta.Meta) == 0 {
		meta.Meta = nil
	}

	// 写入目录格式；同名目录已存在（同目录内已有 IDENTITY.md）则不覆盖
	agentDir := filepath.Join(agentsDir, strings.ToLower(meta.Name))
	identityPath := filepath.Join(agentDir, identityFileName)
	if _, err := os.Stat(identityPath); err == nil {
		return backupLegacyFile(legacyPath)
	}
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("创建 Agent 目录失败: %w", err)
	}
	identity, err := renderIdentity(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(identityPath, []byte(identity), 0644); err != nil {
		return fmt.Errorf("写入 IDENTITY.md 失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, soulFileName), []byte(soulBody), 0644); err != nil {
		return fmt.Errorf("写入 SOUL.md 失败: %w", err)
	}

	return backupLegacyFile(legacyPath)
}

// backupLegacyFile 将旧单文件改名为 .bak 备份。
func backupLegacyFile(legacyPath string) error {
	if err := os.Rename(legacyPath, legacyPath+legacyBackupExt); err != nil {
		return fmt.Errorf("备份旧文件失败: %w", err)
	}
	return nil
}
