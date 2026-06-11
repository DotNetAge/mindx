package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/goreact/rule"
)

// ---------------------------------------------------------------------------
// Rule JSON-RPC handlers
// Data stored in ~/.mindx/data/rules.yml via FileRuleRegistry
// ---------------------------------------------------------------------------

// ruleGetParams is the params for rule.get.
type ruleGetParams struct {
	ID string `json:"id"` // required: rule ID
}

// ruleCreateParams is the params for rule.create.
type ruleCreateParams struct {
	ID       string         `json:"id"`                 // required: unique identifier
	Intro    string         `json:"intro"`              // required: behavioral description shown in system prompt
	Scope    rule.RuleScope `json:"scope,omitempty"`    // default: "global"
	Priority int            `json:"priority,omitempty"` // default: 0
	Enabled  bool           `json:"enabled,omitempty"`  // default: true
}

// ruleUpdateParams is the params for rule.update.
type ruleUpdateParams struct {
	ID       string          `json:"id"`                 // required: rule ID to update
	Intro    *string         `json:"intro,omitempty"`    // optional: new intro
	Scope    *rule.RuleScope `json:"scope,omitempty"`    // optional: new scope
	Priority *int            `json:"priority,omitempty"` // optional: new priority
	Enabled  *bool           `json:"enabled,omitempty"`  // optional: new enabled state
}

// ruleDeleteParams is the params for rule.delete.
type ruleDeleteParams struct {
	ID string `json:"id"` // required: rule ID to delete
}

func (d *Daemon) handleRuleList(_ context.Context, _ json.RawMessage) (any, error) {
	reg := d.app.RuleRegistry()
	if reg == nil {
		return nil, fmt.Errorf("rule registry not initialized")
	}
	all := reg.All()
	return map[string]interface{}{
		"count": len(all),
		"rules": all,
	}, nil
}

func (d *Daemon) handleRuleGet(_ context.Context, params json.RawMessage) (any, error) {
	var p ruleGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	reg := d.app.RuleRegistry()
	if reg == nil {
		return nil, fmt.Errorf("rule registry not initialized")
	}

	r, ok := reg.Get(p.ID)
	if !ok {
		return map[string]interface{}{"found": false}, nil
	}
	return map[string]interface{}{"found": true, "rule": r}, nil
}

func (d *Daemon) handleRuleCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p ruleCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}
	if p.Intro == "" {
		return nil, fmt.Errorf("intro is required")
	}

	reg := d.app.RuleRegistry()
	if reg == nil {
		return nil, fmt.Errorf("rule registry not initialized")
	}

	newRule := rule.Rule{
		ID:       p.ID,
		Intro:    p.Intro,
		Scope:    p.Scope,
		Priority: p.Priority,
		Enabled:  p.Enabled,
	}
	if newRule.Scope == "" {
		newRule.Scope = rule.ScopeGlobal
	}

	if err := reg.Register(newRule); err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	d.logger.Info("rule created", "id", p.ID)
	return map[string]string{"status": "ok", "id": p.ID}, nil
}

func (d *Daemon) handleRuleUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p ruleUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	reg := d.app.RuleRegistry()
	if reg == nil {
		return nil, fmt.Errorf("rule registry not initialized")
	}

	existing, ok := reg.Get(p.ID)
	if !ok {
		return nil, fmt.Errorf("rule %q not found", p.ID)
	}

	updated := *existing // copy
	if p.Intro != nil {
		updated.Intro = *p.Intro
	}
	if p.Scope != nil {
		updated.Scope = *p.Scope
	}
	if p.Priority != nil {
		updated.Priority = *p.Priority
	}
	if p.Enabled != nil {
		updated.Enabled = *p.Enabled
	}

	if err := reg.Register(updated); err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	d.logger.Info("rule updated", "id", p.ID)
	return map[string]string{"status": "ok", "id": p.ID}, nil
}

func (d *Daemon) handleRuleDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p ruleDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	reg := d.app.RuleRegistry()
	if reg == nil {
		return nil, fmt.Errorf("rule registry not initialized")
	}

	existing, ok := reg.Get(p.ID)
	if !ok {
		return nil, fmt.Errorf("rule %q not found", p.ID)
	}

	reg.Unregister(p.ID)

	d.logger.Info("rule deleted", "id", p.ID, "intro", existing.Intro)
	return map[string]any{"status": "ok", "deleted_id": p.ID}, nil
}
