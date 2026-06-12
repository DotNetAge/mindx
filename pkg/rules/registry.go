// Package rules provides a file-based RuleRegistry implementation
// that persists rules to ~/.mindx/data/rules.yml.
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/goharness/rule"
	"gopkg.in/yaml.v3"
)

// ruleFileYAML is the top-level YAML structure for the rules file.
type ruleFileYAML struct {
	Rules []rule.Rule `yaml:"rules"`
}

// FileRuleRegistry implements rule.RuleRegistry with automatic
// persistence to a YAML file on every mutation.
//
// The file path is typically ~/.mindx/data/rules.yml.
// Thread-safe: all reads use RLock, all mutations use Lock + save.
type FileRuleRegistry struct {
	mu    sync.RWMutex
	path  string
	rules []rule.Rule
}

// NewFileRuleRegistry creates a FileRuleRegistry backed by the given YAML file path.
// If the file does not exist it returns an empty registry (the file will be created on first write).
func NewFileRuleRegistry(path string) (*FileRuleRegistry, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	reg := &FileRuleRegistry{path: absPath}
	if err := reg.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load rules from %s: %w", absPath, err)
	}
	return reg, nil
}

// load reads and parses rules from the YAML file.
func (r *FileRuleRegistry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}

	var ry ruleFileYAML
	if err := yaml.Unmarshal(data, &ry); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	for i := range ry.Rules {
		if ry.Rules[i].ID == "" {
			return fmt.Errorf("rule at index %d has empty ID", i)
		}
		if ry.Rules[i].Intro == "" {
			return fmt.Errorf("rule %q has empty intro", ry.Rules[i].ID)
		}
	}

	r.rules = ry.Rules
	return nil
}

// save writes the current rules to the YAML file.
func (r *FileRuleRegistry) save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	data, err := yaml.Marshal(ruleFileYAML{Rules: r.rules})
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	return os.WriteFile(r.path, data, 0644)
}

// Register adds or updates a rule and immediately persists to disk.
func (r *FileRuleRegistry) Register(rule rule.Rule) error {
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

// Unregister removes a rule by ID and immediately persists to disk.
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

// Get retrieves a rule by ID.
func (r *FileRuleRegistry) Get(id string) (*rule.Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.rules {
		if r.rules[i].ID == id {
			return &r.rules[i], true
		}
	}
	return nil, false
}

// All returns a copy of all rules.
func (r *FileRuleRegistry) All() []rule.Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]rule.Rule, len(r.rules))
	copy(out, r.rules)
	return out
}

// GetByScope returns all rules matching the given scope.
func (r *FileRuleRegistry) GetByScope(scope rule.RuleScope) []rule.Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []rule.Rule
	for _, rl := range r.rules {
		if rl.Scope == scope {
			filtered = append(filtered, rl)
		}
	}
	return filtered
}

// FormatPromptSection formats enabled rules as a Markdown list for system prompts.
func (r *FileRuleRegistry) FormatPromptSection() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.rules) == 0 {
		return ""
	}
	var result string
	for _, rl := range r.rules {
		if rl.Enabled {
			result += "- " + rl.Intro + "\n"
		}
	}
	return result
}
