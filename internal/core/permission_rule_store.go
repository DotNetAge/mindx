package core

import (
	"github.com/DotNetAge/goreact/rule"
)

// MindxPermissionRuleStore implements rule.PermissionRuleStore
// by reading/writing permission rules from MindxConfig (persisted in mindx.json).
//
// This is the "秘籍" integration: rules are stored alongside other user preferences
// in ~/.mindx/mindx.json. Most users never touch this — Skill AllowedTools handles
// the common case of pre-approving tools for specific skills.
//
// The store delegates to MindxConfig.Save() for persistence, so rules survive restarts.
type MindxPermissionRuleStore struct {
	config *MindxConfig
}

// NewMindxPermissionRuleStore creates a store backed by the given config.
// The config must already be loaded (via LoadMindxConfig).
func NewMindxPermissionRuleStore(config *MindxConfig) *MindxPermissionRuleStore {
	return &MindxPermissionRuleStore{config: config}
}

// Load implements rule.PermissionRuleStore.
func (s *MindxPermissionRuleStore) Load() (*rule.PermissionRules, error) {
	if s.config == nil {
		return &rule.PermissionRules{}, nil
	}
	rules := s.config.PermissionRules
	if rules == nil {
		return &rule.PermissionRules{}, nil
	}
	return rules, nil
}

// Save implements rule.PermissionRuleStore.
func (s *MindxPermissionRuleStore) Save(rules *rule.PermissionRules) error {
	if s.config == nil {
		return nil
	}
	s.config.PermissionRules = rules
	return s.config.Save()
}
