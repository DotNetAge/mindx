package teamstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TeamMeta 是 TEAM.md frontmatter 的强类型定义。
// 命名三件套与 Agent 原理完全一致：name 内部标识（目录名/引用 key），
// nick_name 展示主名（2-3 字外号），icon 人用图标。
type TeamMeta struct {
	Name string `yaml:"name" json:"name"`
	// NickName 昵称（2-3 字外号，展示主名；统一显示规则 = 昵称 + 辅助小字）。
	NickName    string `yaml:"nick_name,omitempty" json:"nick_name,omitempty"`
	Description string `yaml:"description" json:"description"`
	// Leader 负责人代理名：团队会话 resolve 到该 Agent，其 System Prompt
	// 注入团队段（与团队对话 = 与负责人对话）。
	Leader string `yaml:"leader" json:"leader"`
	// Members 团队成员代理名列表（纯引用，不复制 Agent 内容）。
	// 引用单向（Team→Agent）：Agent 对团队零感知，同一 Agent 可被多个团队引用。
	Members  []string `yaml:"members,omitempty" json:"members,omitempty"`
	Icon     string   `yaml:"icon,omitempty" json:"icon,omitempty"`
	Category string   `yaml:"category,omitempty" json:"category,omitempty"`
	Hired    bool     `yaml:"hired,omitempty" json:"hired,omitempty"`
}

// Team 是加载后的内存态团队：强类型元数据 + Profile 正文 + 目录路径。
type Team struct {
	Meta TeamMeta
	// Profile 是 TEAM.md 正文（团队人设定义），提示词组装时优先于兜底文案。
	Profile string
	Dir     string // 团队目录的绝对路径
}

// Name 返回团队名称。
func (t *Team) Name() string { return t.Meta.Name }

// teamFileName 是团队目录内固定文件名。
const teamFileName = "TEAM.md"

// parseTeam 解析 TEAM.md：frontmatter → TeamMeta，正文 → Profile。
// 正文优先（与 IDENTITY/SOUL 同模式）：正文非空时即团队人设定义
// （标题由文件自身处理）。
func parseTeam(content string) (meta TeamMeta, profile string, err error) {
	meta = TeamMeta{}
	front, body, err := splitFrontmatter(content)
	if err != nil {
		return meta, "", fmt.Errorf("解析 TEAM.md 失败: %w", err)
	}
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return meta, "", fmt.Errorf("解析 TEAM.md frontmatter 失败: %w", err)
	}
	return meta, strings.TrimSpace(body), nil
}

// renderTeam 将 TeamMeta 与 Profile 正文渲染为 TEAM.md 文本。
func renderTeam(meta TeamMeta, profile string) (string, error) {
	if strings.TrimSpace(meta.Name) == "" {
		return "", fmt.Errorf("团队名称不能为空")
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("序列化 TEAM.md frontmatter 失败: %w", err)
	}
	return fmt.Sprintf("---\n%s---\n%s\n", string(data), profile), nil
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

// loadTeamDir 从单个团队目录加载 Team。TEAM.md 必需。
func loadTeamDir(dir string) (*Team, error) {
	teamPath := filepath.Join(dir, teamFileName)
	data, err := os.ReadFile(teamPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", teamPath, err)
	}
	meta, profile, err := parseTeam(string(data))
	if err != nil {
		return nil, err
	}
	return &Team{
		Meta:    meta,
		Profile: profile,
		Dir:     dir,
	}, nil
}
