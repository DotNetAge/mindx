package core

type Skill struct {
	GetName     func() string
	Execute     func(name string, params map[string]interface{}) error
	ExecuteFunc func(function ToolCallFunction) error
}

// SkillMgr 技能管理器
// 技能管理器采用FIFO队列，每次执行一个技能
type SkillManager interface {
	Execute(skill *Skill, params map[string]interface{}) error // 执行技能
	ExecuteFunc(function ToolCallFunction) (string, error)     // 执行工具
	GetSkills() ([]*Skill, error)                              // 获取全部技能
	SearchSkills(keywords ...string) ([]*Skill, error)         // 搜索名称、用法与关键字相似度最高的技能
	RegisterInternalSkill(name string, fn func(params map[string]any) (string, error)) // 注册内部技能
}
