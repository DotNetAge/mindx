// Package rules 是 mindx 的规则域包（PR-PROMPTS P4 评审定案）：
// 行为规则类型与 RuleRegistry 接口、权限规则类型，以及两种实现——
// FileRuleRegistry（YAML 文件持久化，~/.mindx/data/rules.yml）
// 与 MemRuleRegistry（内存聚合，不做持久化）。
// 规则数据的所有权与实现归 mindx，goharness 不再持有任何规则代码。
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ruleFileYAML 是规则 YAML 文件的顶层结构。
type ruleFileYAML struct {
	Rules []Rule `yaml:"rules"`
}

// FileRuleRegistry 基于 YAML 文件持久化的 RuleRegistry 实现，
// 每次变更立即写盘。所有读操作走 RLock，变更走 Lock + save，保证线程安全。
type FileRuleRegistry struct {
	mu    sync.RWMutex
	path  string
	rules []Rule
}

// NewFileRuleRegistry 创建以给定 YAML 文件为后端的 FileRuleRegistry。
// 文件不存在时返回空注册表（首次写盘时创建文件）。
func NewFileRuleRegistry(path string) (*FileRuleRegistry, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析路径: %w", err)
	}
	reg := &FileRuleRegistry{path: absPath}
	if err := reg.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("从 %s 加载规则: %w", absPath, err)
	}
	return reg, nil
}

// load 从 YAML 文件读取并解析规则。
// 校验所有规则的 ID 和 Intro 字段均非空。
func (r *FileRuleRegistry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}

	var ry ruleFileYAML
	if err := yaml.Unmarshal(data, &ry); err != nil {
		return fmt.Errorf("解析规则配置文件: %w", err)
	}

	for i := range ry.Rules {
		if ry.Rules[i].ID == "" {
			return fmt.Errorf("第 %d 条规则缺少 ID", i)
		}
		if ry.Rules[i].Intro == "" {
			return fmt.Errorf("规则 %q 缺少介绍文本", ry.Rules[i].ID)
		}
	}

	r.rules = ry.Rules
	return nil
}

// save 将当前规则写入 YAML 文件。
func (r *FileRuleRegistry) save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录: %w", err)
	}

	data, err := yaml.Marshal(ruleFileYAML{Rules: r.rules})
	if err != nil {
		return fmt.Errorf("序列化规则: %w", err)
	}

	return os.WriteFile(r.path, data, 0644)
}

// Register 添加或更新一条规则并立即持久化。
// 相同 ID 的规则会被覆盖。
func (r *FileRuleRegistry) Register(rule Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.rules {
		if r.rules[i].ID == rule.ID {
			r.rules[i] = rule
			return r.save()
		}
	}
	r.rules = append(r.rules, rule)
	return r.save()
}

// Unregister 按 ID 移除一条规则并立即持久化。
// 规则不存在时为空操作。
func (r *FileRuleRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			_ = r.save()
			return
		}
	}
}

// Get 按 ID 检索一条规则。找到时返回该规则和 true，否则返回 nil 和 false。
func (r *FileRuleRegistry) Get(id string) (*Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.rules {
		if r.rules[i].ID == id {
			return &r.rules[i], true
		}
	}
	return nil, false
}

// All 返回注册表中所有规则的副本。
func (r *FileRuleRegistry) All() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Rule, len(r.rules))
	copy(out, r.rules)
	return out
}

// GetByScope 返回匹配给定范围的所有规则。
func (r *FileRuleRegistry) GetByScope(scope RuleScope) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Rule
	for _, rl := range r.rules {
		if rl.Scope == scope {
			filtered = append(filtered, rl)
		}
	}
	return filtered
}

// FormatPromptSection 将已启用的规则格式化为 Markdown 列表，用于嵌入 System Prompt。
// 未定义规则或全部禁用时返回空字符串。
func (r *FileRuleRegistry) FormatPromptSection() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.rules) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, rl := range r.rules {
		if rl.Enabled {
			sb.WriteString("- ")
			sb.WriteString(rl.Intro)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
