package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	goharnessconfig "github.com/DotNetAge/goharness/config"
)

func (d *Daemon) handleAgentList(_ context.Context, params json.RawMessage) (any, error) {
	agents := d.app.Agents()
	if agents == nil {
		return []goharnessconfig.AgentConfig{}, nil
	}
	list := agents.List()
	if list == nil {
		return []goharnessconfig.AgentConfig{}, nil
	}

	type agentEntry struct {
		Name         string         `json:"name"`
		Role         string         `json:"role,omitempty"`
		Description  string         `json:"description"`
		Introduction string         `json:"introduction,omitempty"`
		Model        string         `json:"model"`
		Skills       []string       `json:"skills,omitempty"`
		ExcludeTools []string       `json:"exclude_tools,omitempty"`
		Meta         map[string]any `json:"meta,omitempty"`
	}

	result := make([]agentEntry, len(list))
	for i, a := range list {
		result[i] = agentEntry{
			Name:         a.Name,
			Role:         a.Role,
			Description:  a.Description,
			Introduction: a.Introduction,
			Model:        a.Model,
			Skills:       a.Skills,
			ExcludeTools: a.ExcludeTools,
			Meta:         a.Meta,
		}
	}
	return result, nil
}

type agentGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleAgentGet(_ context.Context, params json.RawMessage) (any, error) {
	var p agentGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	cfg := agents.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}

	return cfg, nil
}

type agentCreateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Description  string         `json:"description"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model"`
	Skills       []string       `json:"skills,omitempty"`
	Body         string         `json:"body"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (d *Daemon) handleAgentCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.Role == "" {
		return nil, fmt.Errorf("role is required")
	}
	if p.Description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if p.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if p.Body == "" {
		return nil, fmt.Errorf("body is required")
	}

	if d.app.Models() == nil || d.app.Models().Get(p.Model) == nil {
		return nil, fmt.Errorf("model %q not found", p.Model)
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	existing := agents.Get(p.Name)
	if existing != nil {
		return nil, fmt.Errorf("agent %q already exists", p.Name)
	}

	newAgent := goharnessconfig.AgentConfig{
		Name:         p.Name,
		Role:         p.Role,
		Description:  p.Description,
		Introduction: p.Introduction,
		Model:        p.Model,
		Skills:       p.Skills,
		Meta:         p.Meta,
	}
	if p.Body != "" && newAgent.Introduction == "" {
		newAgent.Introduction = p.Body
	}

	if err := agents.SaveTo(&newAgent); err != nil {
		return nil, fmt.Errorf("failed to create agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": newAgent.Name,
		"message":    "agent created successfully",
	}, nil
}

type agentUpdateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role,omitempty"`
	Description  string         `json:"description,omitempty"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model,omitempty"`
	Skills       []string       `json:"skills,omitempty"`
	ExcludeTools []string       `json:"exclude_tools,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (d *Daemon) handleAgentUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	existing := agents.Get(p.Name)
	if existing == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}

	updated := *existing

	if p.Role != "" {
		updated.Role = p.Role
	}
	if p.Description != "" {
		updated.Description = p.Description
	}
	if p.Model != "" {
		updated.Model = p.Model
	}
	if p.Skills != nil {
		updated.Skills = p.Skills
	}
	if p.ExcludeTools != nil {
		updated.ExcludeTools = p.ExcludeTools
	}
	if p.Introduction != "" {
		updated.Introduction = p.Introduction
	}

	if p.Meta != nil {
		updated.Meta = p.Meta
	}

	if err := agents.SaveTo(&updated); err != nil {
		return nil, fmt.Errorf("failed to save agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": updated.Name,
		"message":    "agent config updated",
	}, nil
}

type agentScoreParams struct {
	AgentName string `json:"agent_name"`
	Task      string `json:"task"`
	Score     int    `json:"score"`
	Notes     string `json:"notes,omitempty"`
}

func (d *Daemon) handleAgentScore(_ context.Context, params json.RawMessage) (any, error) {
	var p agentScoreParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.AgentName == "" {
		return nil, fmt.Errorf("agent_name is required")
	}
	if p.Task == "" {
		return nil, fmt.Errorf("task is required")
	}
	if p.Score < 1 || p.Score > 10 {
		return nil, fmt.Errorf("score must be between 1 and 10")
	}

	// Verify agent exists (but do NOT write to its config file)
	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}
	if agents.Get(p.AgentName) == nil {
		return nil, fmt.Errorf("agent %q not found", p.AgentName)
	}

	if d.kvStore == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	scoreKey := fmt.Sprintf("score:%s:%d", p.AgentName, time.Now().UnixNano())

	entry := map[string]any{
		"agent_name": p.AgentName,
		"task":       p.Task,
		"score":      p.Score,
		"timestamp":  timestamp,
	}
	if p.Notes != "" {
		entry["notes"] = p.Notes
	}

	itemData, err := json.Marshal(kvItem{
		Key:       scoreKey,
		Value:     entry,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal score entry: %w", err)
	}

	if err := d.kvStore.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(scoreKey), itemData)
	}); err != nil {
		return nil, fmt.Errorf("failed to store score in kvstore: %w", err)
	}

	// Read all historical scores for this agent via prefix scan
	prefix := fmt.Sprintf("score:%s:", p.AgentName)
	var allScores []int
	var completes int

	_ = d.kvStore.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var item kvItem
			if json.Unmarshal(v, &item) == nil {
				if m, ok := item.Value.(map[string]any); ok {
					if sc, ok := m["score"].(int); ok {
						allScores = append(allScores, sc)
					}
				}
			}
			completes++
		}
		return nil
	})

	return map[string]any{
		"status":    "scored",
		"agent":     p.AgentName,
		"task":      p.Task,
		"score":     p.Score,
		"notes":     p.Notes,
		"timestamp": timestamp,
		"scores":    allScores,
		"completes": completes,
	}, nil
}
