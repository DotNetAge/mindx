package entity

import (
	"strings"
	"time"
)

// Skill 技能定义（符合 agentskills.io 规范）
// Skill 是 SOP（标准操作程序）知识文档，不是可执行工具
type Skill struct {
	// ========== 基础信息 ==========
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"` // SOP 描述
	Version     string `yaml:"version" json:"version"`
	Author      string `yaml:"author" json:"author"`

	// ========== 核心内容 ==========
	Goal     string   `yaml:"-" json:"goal"`         // 技能目标（从 Markdown 解析）
	Triggers []string `yaml:"-" json:"triggers"`     // 触发条件列表（从 Markdown 解析）
	SOP      string   `yaml:"-" json:"sop"`          // 标准操作程序（从 Markdown 解析）
	Examples []string `yaml:"-" json:"examples"`     // 使用示例（从 Markdown 解析）

	// ========== 工具依赖 ==========
	RequiredTools []string `yaml:"required_tools,omitempty" json:"required_tools,omitempty"` // 必需工具
	OptionalTools []string `yaml:"optional_tools,omitempty" json:"optional_tools,omitempty"` // 可选工具

	// ========== 索引 ==========
	Tags      []string  `yaml:"tags" json:"tags"`           // 标签
	Keywords  []string  `yaml:"-" json:"keywords"`          // 关键词（从内容提取）
	Embedding []float32 `yaml:"-" json:"embedding"`         // 向量表示（Goal + Triggers）

	// ========== 元数据 ==========
	FilePath  string    `yaml:"-" json:"file_path"`         // SKILL.md 文件路径
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
	CreatedAt time.Time `yaml:"-" json:"created_at"`
}

// SkillMatch Skill 匹配结果
type SkillMatch struct {
	Skill *Skill  `json:"skill"`
	Score float64 `json:"score"` // 匹配分数 [0.0, 1.0]
}

// SkillSOP 已在 think_context.go 中定义，这里不重复定义

// GetEmbeddingText 获取用于向量化的文本
// 组合 Goal + Triggers 生成向量
func (s *Skill) GetEmbeddingText() string {
	var builder strings.Builder
	builder.WriteString(s.Goal)
	for _, trigger := range s.Triggers {
		builder.WriteString("\n")
		builder.WriteString(trigger)
	}
	return builder.String()
}

// HasTool 检查是否需要指定工具
func (s *Skill) HasTool(toolName string) bool {
	// 检查必需工具
	for _, tool := range s.RequiredTools {
		if tool == toolName {
			return true
		}
	}
	// 检查可选工具
	for _, tool := range s.OptionalTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// ToSOP 转换为 SkillSOP（用于 ThinkContext）
func (s *Skill) ToSOP() *SkillSOP {
	return &SkillSOP{
		Name:          s.Name,
		Description:   s.Description,
		Keywords:      s.Keywords,
		RequiredTools: s.RequiredTools,
		SOPContent:    s.SOP,
	}
}
