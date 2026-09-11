// Package agentstore 提供 mindx 侧的 Agent 目录化存储。
//
// 存储格式（PR-PROMPTS 第二节）：
//
//	agents/<agent-name>/
//	  IDENTITY.md   # 唯一 meta 入口：frontmatter（强类型一级字段）+ 身份正文
//	  SOUL.md       # 行为规则与负责范围，纯正文
//	  skills/       # Agent 级技能库（由 skillstore 装载，本包不感知）
//
// 设计要点：
//   - Agent 属性强类型化：icon / category / hired 等从 meta map 提升为
//     frontmatter 一级字段，Meta map 仅保留真正的自由扩展杂项；
//   - 全量强类型序列化，写路径不再有"SaveTo 丢失手写字段"的问题；
//   - 旧单文件（{name}.md）在 Load 时一次性迁移为目录格式（原文件备份为
//     {name}.md.bak），加载器不留双格式分支。
package agentstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentMeta 是 IDENTITY.md frontmatter 的强类型定义。
// 字段均为一级字段，icon / category / hired 由旧 meta map 提升而来；
// category 为业务分类（中文，如「产品研发」），由旧 domains 字段一次性迁移而来。
type AgentMeta struct {
	Name         string   `yaml:"name" json:"name"`
	Role         string   `yaml:"role" json:"role"`
	Description  string   `yaml:"description" json:"description"`
	Introduction string   `yaml:"introduction,omitempty" json:"introduction,omitempty"`
	Icon         string   `yaml:"icon,omitempty" json:"icon,omitempty"`
	Category     string   `yaml:"category,omitempty" json:"category,omitempty"`
	Hired        bool     `yaml:"hired,omitempty" json:"hired,omitempty"`
	Skills       []string `yaml:"skills,omitempty" json:"skills,omitempty"`
	ExcludeTools []string `yaml:"exclude_tools,omitempty" json:"exclude_tools,omitempty"`
	// AllowsTools 该员工允许使用的云技能（MCP 服务）清单。
	// 注意：这个属性里放的全是 MCP 工具——条目格式为 "mcp:<server>"（server 粒度），
	// 内置工具不在此列（内置工具的裁剪走 exclude_tools）。
	// createRuntime 组装工具时按此清单决定注入哪些 MCP server 的工具；空清单 = 不注入任何 MCP 工具。
	AllowsTools []string       `yaml:"allows_tools,omitempty" json:"allows_tools,omitempty"`
	Meta        map[string]any `yaml:"meta,omitempty" json:"meta,omitempty"`
}

// Agent 是加载后的内存态 Agent：强类型元数据 + SOUL 正文 + 目录路径。
type Agent struct {
	Meta AgentMeta
	Soul string // SOUL.md 正文（行为规则），可为空
	Dir  string // Agent 目录的绝对路径
}

// Name 返回 Agent 名称。
func (a *Agent) Name() string { return a.Meta.Name }

// identityFileName / soulFileName 是目录内固定文件名。
const (
	identityFileName = "IDENTITY.md"
	soulFileName     = "SOUL.md"
	legacyBackupExt  = ".bak"
)

// parseIdentity 解析 IDENTITY.md：frontmatter → AgentMeta，正文 → 身份正文。
// 正文优先（评审定案）：正文非空时覆盖 frontmatter 的 introduction——带正文的
// IDENTITY.md 以正文作为整个人设定义（标题由文件自身处理）；frontmatter 的
// introduction 降级为正文缺失时的备用说明。
// 迁移语义：旧版一级字段 domains（中文业务领域，恒为单值）一次性迁移为
// category（取首个非空值）；migrated 为 true 时调用方应重写文件消除旧键。
func parseIdentity(content string) (meta AgentMeta, body string, migrated bool, err error) {
	meta = AgentMeta{}
	front, body, err := splitFrontmatter(content)
	if err != nil {
		return meta, "", false, fmt.Errorf("解析 IDENTITY.md 失败: %w", err)
	}
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return meta, "", false, fmt.Errorf("解析 IDENTITY.md frontmatter 失败: %w", err)
	}
	var legacy map[string]any
	if err := yaml.Unmarshal([]byte(front), &legacy); err == nil {
		if v, ok := legacy["domains"]; ok {
			// 旧键存在即标记迁移（含空值形态），写回后文件中不再保留 domains
			migrated = true
			if items, ok := v.([]any); ok {
				for _, it := range items {
					if s, ok := it.(string); ok {
						if s = strings.TrimSpace(s); s != "" && meta.Category == "" {
							meta.Category = s
						}
					}
				}
			}
		}
	}
	body = strings.TrimSpace(body)
	if body != "" {
		meta.Introduction = body
	}
	return meta, body, migrated, nil
}

// renderIdentity 将 AgentMeta 与身份正文渲染为 IDENTITY.md 文本。
// 正文与 frontmatter 的 introduction 保持一致，保证文件对人类可读。
func renderIdentity(meta AgentMeta) (string, error) {
	if strings.TrimSpace(meta.Name) == "" {
		return "", fmt.Errorf("agent 名称不能为空")
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("序列化 IDENTITY.md frontmatter 失败: %w", err)
	}
	return fmt.Sprintf("---\n%s---\n%s\n", string(data), meta.Introduction), nil
}

// splitFrontmatter 从 Markdown 文本中分离 YAML frontmatter 与正文。
// 要求首行为 "---"，frontmatter 以下一个 "---" 行闭合。
func splitFrontmatter(content string) (front, body string, err error) {
	content = strings.TrimLeft(content, "\n\r")
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("缺少 YAML frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", fmt.Errorf("frontmatter 未闭合（缺少结尾 ---）")
}

// loadAgentDir 从单个 Agent 目录加载 Agent。
// IDENTITY.md 必需；SOUL.md 缺省为空（不视为错误）。
// 旧版 frontmatter（含 domains 键）在加载时一次性迁移为 category 并写回磁盘，
// 不留双格式（与旧单文件迁移语义一致）；写回失败视为加载失败，不静默降级。
func loadAgentDir(dir string) (*Agent, error) {
	identityPath := filepath.Join(dir, identityFileName)
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", identityPath, err)
	}
	meta, _, migrated, err := parseIdentity(string(data))
	if err != nil {
		return nil, err
	}
	if migrated {
		rendered, rerr := renderIdentity(meta)
		if rerr != nil {
			return nil, fmt.Errorf("迁移 %s 失败: %w", identityPath, rerr)
		}
		if werr := os.WriteFile(identityPath, []byte(rendered), 0644); werr != nil {
			return nil, fmt.Errorf("写回迁移结果 %s 失败: %w", identityPath, werr)
		}
	}

	soul := ""
	soulData, err := os.ReadFile(filepath.Join(dir, soulFileName))
	if err == nil {
		soul = strings.TrimSpace(string(soulData))
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 %s 失败: %w", filepath.Join(dir, soulFileName), err)
	}

	return &Agent{
		Meta: meta,
		Soul: soul,
		Dir:  dir,
	}, nil
}
