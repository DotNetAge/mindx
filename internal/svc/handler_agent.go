package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

func (d *Daemon) handleAgentList(_ context.Context, params json.RawMessage) (any, error) {
	agents := d.app.Agents()
	if agents == nil {
		return []agentstore.AgentMeta{}, nil
	}
	list := agents.List()

	result := make([]agentstore.AgentMeta, 0, len(list))
	for _, a := range list {
		result = append(result, a.Meta)
	}
	return result, nil
}

// agentDetailResult 是 agent.get 的返回：Meta 全量字段内联展开，附加 SOUL.md 正文。
// 仅做增量扩展（新增 soul 字段），既有消费方不受影响。
type agentDetailResult struct {
	agentstore.AgentMeta
	Soul string `json:"soul"`
}

func (d *Daemon) handleAgentGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentGetParams
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

	agent := agents.Get(p.Name)
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}
	return agentDetailResult{AgentMeta: agent.Meta, Soul: agent.Soul}, nil
}

func (d *Daemon) handleAgentCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentCreateParams
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
	if p.Body == "" {
		return nil, fmt.Errorf("body is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	if agents.Get(p.Name) != nil {
		return nil, fmt.Errorf("agent %q already exists", p.Name)
	}

	introduction := p.Introduction
	if introduction == "" {
		introduction = p.Body
	}

	newAgent := &agentstore.Agent{
		Meta: agentstore.AgentMeta{
			Name:         p.Name,
			Role:         p.Role,
			Description:  p.Description,
			Introduction: introduction,
			Skills:       p.Skills,
			Meta:         p.Meta,
		},
	}

	if err := agents.Save(newAgent); err != nil {
		return nil, fmt.Errorf("failed to create agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": newAgent.Meta.Name,
		"message":    "agent created successfully",
	}, nil
}

func (d *Daemon) handleAgentUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentUpdateParams
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
		updated.Meta.Role = p.Role
	}
	if p.Description != "" {
		updated.Meta.Description = p.Description
	}
	if p.Skills != nil {
		updated.Meta.Skills = p.Skills
	}
	if p.ExcludeTools != nil {
		updated.Meta.ExcludeTools = p.ExcludeTools
	}
	if p.AllowsTools != nil {
		updated.Meta.AllowsTools = *p.AllowsTools
	}
	if p.Introduction != "" {
		updated.Meta.Introduction = p.Introduction
	}
	// 指针字段：nil 表示未传（保持不变），非 nil 表示覆盖（含空串清空正文）。
	// 与加载语义一致：parseIdentity/loadAgentDir 均对正文做 TrimSpace。
	if p.IdentityBody != nil {
		updated.Meta.Introduction = strings.TrimSpace(*p.IdentityBody)
	}
	if p.Soul != nil {
		updated.Soul = strings.TrimSpace(*p.Soul)
	}
	if p.Meta != nil {
		updated.Meta.Meta = p.Meta
	}

	if err := agents.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": updated.Meta.Name,
		"message":    "agent config updated",
	}, nil
}

func (d *Daemon) handleAgentScore(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentScoreParams
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

func (d *Daemon) handleAgentReload(_ context.Context, params json.RawMessage) (any, error) {
	if err := d.app.ReloadAgents(); err != nil {
		return nil, fmt.Errorf("agent reload failed: %w", err)
	}
	return map[string]string{
		"status":  "ok",
		"message": "agents reloaded successfully",
	}, nil
}

// handleAgentHire 雇佣 Agent（hired=true），雇佣后对会话可用。
func (d *Daemon) handleAgentHire(_ context.Context, params json.RawMessage) (any, error) {
	return d.setAgentHired(params, true)
}

// handleAgentFire 解雇 Agent（hired=false），解雇后对会话不可用。
func (d *Daemon) handleAgentFire(_ context.Context, params json.RawMessage) (any, error) {
	return d.setAgentHired(params, false)
}

// setAgentHired 是 agent.hire / agent.fire 的公共实现：
// hired 为 IDENTITY.md frontmatter 一级字段，经 agentstore 强类型全量
// 序列化写入并同步内存缓存，无需 reload 即刻生效。
func (d *Daemon) setAgentHired(params json.RawMessage, hired bool) (any, error) {
	var p rpc.AgentHireParams
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

	if err := core.SetAgentHired(agents, p.Name, hired); err != nil {
		return nil, err
	}

	message := fmt.Sprintf("智能体 %q 已雇佣，可用于会话", p.Name)
	if !hired {
		message = fmt.Sprintf("智能体 %q 已解雇，不再用于会话", p.Name)
	}
	return map[string]any{
		"status":     "ok",
		"agent_name": p.Name,
		"hired":      hired,
		"message":    message,
	}, nil
}
