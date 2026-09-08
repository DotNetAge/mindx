package core

import (
	"github.com/DotNetAge/mindx/pkg/rules"
)

// MindxPermissionRuleStore 实现 rules.PermissionRuleStore，
// 权限规则随 MindxConfig 持久化在 ~/.mindx/mindx.json 中。
//
// 这是「秘籍」集成路径：权限规则与其它用户偏好存放在一起。
// 大多数用户不会触碰它——Skill 的 AllowedTools 已覆盖为特定技能
// 预授权工具的常见场景。
//
// 持久化委托给 MindxConfig.Save()，规则因此跨重启保留。
type MindxPermissionRuleStore struct {
	config *MindxConfig
}

// NewMindxPermissionRuleStore 创建以给定配置为后端的权限规则存储。
// 配置必须已加载（经 LoadMindxConfig）。
func NewMindxPermissionRuleStore(config *MindxConfig) *MindxPermissionRuleStore {
	return &MindxPermissionRuleStore{config: config}
}

// Load 获取当前的权限规则集合（实现 rules.PermissionRuleStore）。
func (s *MindxPermissionRuleStore) Load() (*rules.PermissionRules, error) {
	if s.config == nil {
		return &rules.PermissionRules{}, nil
	}
	pr := s.config.PermissionRules
	if pr == nil {
		return &rules.PermissionRules{}, nil
	}
	return pr, nil
}

// Save 持久化给定的权限规则（实现 rules.PermissionRuleStore）。
func (s *MindxPermissionRuleStore) Save(pr *rules.PermissionRules) error {
	if s.config == nil {
		return nil
	}
	s.config.PermissionRules = pr
	return s.config.Save()
}
