package entity

import "time"

// SkillDef 技能定义（从 SKILL.md 读取）
type SkillDef struct {
	Name         string                 `yaml:"name" json:"name"`
	Description  string                 `yaml:"description" json:"description"`
	Version      string                 `yaml:"version" json:"version"`
	Category     string                 `yaml:"category" json:"category"`
	Tags         []string               `yaml:"tags" json:"tags"`
	Emoji        string                 `yaml:"emoji" json:"emoji"`
	OS           []string               `yaml:"os" json:"os"`
	Enabled      bool                   `yaml:"enabled" json:"enabled"`
	Timeout      int                    `yaml:"timeout" json:"timeout"`
	Command      string                 `yaml:"command" json:"command"`
	Parameters   map[string]ParameterDef `yaml:"parameters" json:"parameters"`
	Requires     *Requires              `yaml:"requires,omitempty" json:"requires,omitempty"`
	Install      []InstallMethod        `yaml:"install,omitempty" json:"install,omitempty"`
	Homepage     string                 `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Metadata     map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	OutputFormat string                 `yaml:"output_format,omitempty" json:"output_format,omitempty"`
	Guidance     string                 `yaml:"guidance,omitempty" json:"guidance,omitempty"`
	IsInternal   bool                   `yaml:"is_internal,omitempty" json:"is_internal,omitempty"`
}

// Requires 依赖定义
type Requires struct {
	Bins []string `yaml:"bins,omitempty" json:"bins,omitempty"`
	Env  []string `yaml:"env,omitempty" json:"env,omitempty"`
}

// InstallMethod 安装方法
type InstallMethod struct {
	ID      string   `yaml:"id" json:"id"`
	Kind    string   `yaml:"kind" json:"kind"` // brew, apt, npm, pip, snap, choco, etc.
	Package string   `yaml:"package,omitempty" json:"package,omitempty"`
	Formula string   `yaml:"formula,omitempty" json:"formula,omitempty"`
	Bins    []string `yaml:"bins,omitempty" json:"bins,omitempty"`
	Label   string   `yaml:"label" json:"label"`
	OS      []string `yaml:"os,omitempty" json:"os,omitempty"`
}

// ParameterDef 参数定义
type ParameterDef struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
}

// SkillStats 技能统计数据
type SkillStats struct {
	SuccessCount   int        `json:"successCount"`
	ErrorCount     int        `json:"errorCount"`
	ExecutionTimes []int64    `json:"executionTimes"`
	LastRunTime    *time.Time `json:"lastRunTime,omitempty"`
}

// SkillInfo 技能完整信息
type SkillInfo struct {
	Def         *SkillDef `json:"def"`
	Directory   string    `json:"directory"`
	Content     string    `json:"content"`
	CanRun      bool      `json:"canRun"`
	MissingBins []string  `json:"missingBins"`
	MissingEnv  []string  `json:"missingEnv"`

	// 前端需要的字段
	Format string `json:"format"`
	Status string `json:"status"`

	// 向量（用于相似性搜索）
	Vector []float64 `json:"vector,omitempty"`

	// 统计信息
	SuccessCount    int        `json:"successCount"`
	ErrorCount      int        `json:"errorCount"`
	LastRunTime     *time.Time `json:"lastRunTime,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	AvgExecutionMs  int64      `json:"avgExecutionMs"`
	ExecutionTimes  []int64    `json:"executionTimes"`
}
