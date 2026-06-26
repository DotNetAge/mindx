package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/goharness/rule"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// Rule JSON-RPC handlers
// Data stored in ~/.mindx/data/rules.yml via FileRuleRegistry
// ---------------------------------------------------------------------------

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
	var p rpc.RuleGetParams
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
	var p rpc.RuleCreateParams
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
		Scope:    rule.RuleScope(p.Scope),
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
	var p rpc.RuleUpdateParams
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
		updated.Scope = rule.RuleScope(*p.Scope)
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
	var p rpc.RuleDeleteParams
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
