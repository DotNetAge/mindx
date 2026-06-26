package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

func newTestDaemon(t *testing.T) (*Daemon, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	sessionsDir := filepath.Join(tmpDir, "sessions")
	dataDir := filepath.Join(tmpDir, "data")
	prefsDir := filepath.Join(tmpDir, "prefs")
	_ = os.MkdirAll(sessionsDir, 0755)
	_ = os.MkdirAll(dataDir, 0755)
	_ = os.MkdirAll(prefsDir, 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "agents"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "settings"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "settings", "models.yml"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "settings", "rules.yml"), []byte{}, 0644)

	app, err := core.DefaultApp(core.DefaultMindxConfig(tmpDir))
	if err != nil {
		t.Fatalf("DefaultApp() error = %v", err)
	}

	_ = app.SetTestDir(tmpDir)

	d := NewDaemon(app, ":0", "/ws", nil)

	cleanup := func() {
		d.stopBackgroundServices()
	}

	return d, cleanup
}

func mustCreateSession(t *testing.T, sessDB *mindxses.FileSessionStore, agentName string) string {
	t.Helper()
	info, err := sessDB.Create(context.Background(), agentName)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	msg := goharnesssession.Message{
		Role:      "user",
		Content:   "init",
		Timestamp: time.Now().UnixMilli(),
	}
	if err := sessDB.Append(context.Background(), info.SessionID, agentName, msg); err != nil {
		t.Fatalf("append init msg: %v", err)
	}
	return info.SessionID
}

// ==========================================================================
// Session RPC Handlers — handleSessionList
// ==========================================================================

func TestHandleSessionList_Empty(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleSessionList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleSessionList error = %v", err)
	}

	sessions, ok := result.([]goharnesssession.SessionInfo)
	if !ok {
		t.Fatalf("expected []SessionInfo, got %T", result)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestHandleSessionList_WithSessions(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	if sessDB == nil {
		t.Fatal("SessDB() is nil")
	}

	mustCreateSession(t, sessDB, "agent-alpha")
	mustCreateSession(t, sessDB, "agent-beta")
	mustCreateSession(t, sessDB, "agent-alpha")

	result, err := d.handleSessionList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleSessionList error = %v", err)
	}

	sessions, ok := result.([]goharnesssession.SessionInfo)
	if !ok {
		t.Fatalf("expected []SessionInfo, got %T", result)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestHandleSessionList_FilterByAgent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	mustCreateSession(t, sessDB, "agent-alpha")
	mustCreateSession(t, sessDB, "agent-beta")
	mustCreateSession(t, sessDB, "agent-alpha")

	params, _ := json.Marshal(map[string]string{"agent": "agent-alpha"})
	result, err := d.handleSessionList(context.Background(), params)
	if err != nil {
		t.Fatalf("handleSessionList error = %v", err)
	}

	sessions, ok := result.([]goharnesssession.SessionInfo)
	if !ok {
		t.Fatalf("expected []SessionInfo, got %T", result)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for agent-alpha, got %d", len(sessions))
	}
	for _, s := range sessions {
		if s.AgentName != "agent-alpha" {
			t.Errorf("unexpected agent: %s", s.AgentName)
		}
	}
}

func TestHandleSessionList_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	badParams := json.RawMessage("{invalid json")
	_, err := d.handleSessionList(context.Background(), badParams)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleSessionList_NilParams(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleSessionList(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil params should be accepted, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for empty session list")
	}
}

// ==========================================================================
// Session RPC Handlers — handleSessionGet
// ==========================================================================

func TestHandleSessionGet_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sid := mustCreateSession(t, sessDB, "test-agent")

	params, _ := json.Marshal(map[string]string{"session_id": sid})
	result, err := d.handleSessionGet(context.Background(), params)
	if err != nil {
		t.Fatalf("handleSessionGet error = %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	if m["session_id"] != sid {
		t.Errorf("session_id = %v, want %s", m["session_id"], sid)
	}
	if m["meta"] == nil {
		t.Error("expected meta to be present")
	}
}

func TestHandleSessionGet_MissingSessionID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleSessionGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestHandleSessionGet_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"session_id": "sess_nonexistent"})
	result, err := d.handleSessionGet(context.Background(), params)
	if err != nil {
		t.Fatalf("handleSessionGet for nonexistent session should not error on missing session.yml, got: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	msgs, ok := m["messages"].([]goharnesssession.Message)
	if !ok {
		t.Fatalf("expected messages to be []Message, got %T", m["messages"])
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for nonexistent session, got %d", len(msgs))
	}
}

// ==========================================================================
// Session RPC Handlers — handleSessionMeta
// ==========================================================================

func TestHandleSessionMeta_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sid := mustCreateSession(t, sessDB, "test-agent")

	params, _ := json.Marshal(map[string]string{"session_id": sid})
	result, err := d.handleSessionMeta(context.Background(), params)
	if err != nil {
		t.Fatalf("handleSessionMeta error = %v", err)
	}

	meta, ok := result.(*mindxses.SessionMeta)
	if !ok {
		t.Fatalf("expected *SessionMeta, got %T", result)
	}
	if meta.SessionID != sid {
		t.Errorf("meta.SessionID = %s, want %s", meta.SessionID, sid)
	}
	if meta.AgentName != "test-agent" {
		t.Errorf("meta.AgentName = %s, want test-agent", meta.AgentName)
	}
}

func TestHandleSessionMeta_MissingSessionID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleSessionMeta(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestHandleSessionMeta_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"session_id": "sess_noop"})
	_, err := d.handleSessionMeta(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ==========================================================================
// Memory RPC Handlers — validation & nil memory guard
// ==========================================================================

func TestHandleMemoryQuery_NilMemory(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"query": "test"})
	_, err := d.handleMemoryQuery(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when sharedMemory is nil")
	}
}

func TestHandleMemoryQuery_MissingQuery(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleMemoryQuery(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestHandleMemoryQuery_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleMemoryQuery(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleMemoryStore_NilMemory(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"content": "hello"})
	_, err := d.handleMemoryStore(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when sharedMemory is nil")
	}
}

func TestHandleMemoryStore_MissingContent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"title": "test"})
	_, err := d.handleMemoryStore(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestHandleMemoryStore_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleMemoryStore(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleMemoryStore_SessionTypeNilMemory(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"content": "test content",
		"type":    "session",
	})
	_, err := d.handleMemoryStore(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when sharedMemory is nil (even with session type)")
	}
}

func TestHandleMemoryDelete_NilMemory(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"id": "mem_123"})
	_, err := d.handleMemoryDelete(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when sharedMemory is nil")
	}
}

func TestHandleMemoryDelete_MissingID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleMemoryDelete(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleMemoryDelete_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleMemoryDelete(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ==========================================================================
// Registration verification
// ==========================================================================

func TestRPCMethods_InitGatewayRegistersAll(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	d.initGateway()

	if d.gw == nil {
		t.Fatal("initGateway did not create gateway")
	}

	result, err := d.handleSessionList(context.Background(), nil)
	if err != nil {
		t.Fatalf("session.list after initGateway: %v", err)
	}
	if result == nil {
		t.Fatal("session.list returned nil")
	}
}

// ==========================================================================
// Agent RPC Handlers
// ==========================================================================

func TestHandleAgentList_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleAgentList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleAgentList error = %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleAgentList_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleAgentList(context.Background(), json.RawMessage("bad"))
	if err != nil {
		t.Fatalf("handleAgentList ignores params, unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even with bad params")
	}
}

func TestHandleAgentGet_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleAgentGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleAgentGet_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"name": "nonexistent"})
	_, err := d.handleAgentGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestHandleAgentUpdate_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"role": "updated"})
	_, err := d.handleAgentUpdate(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleAgentUpdate_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"name":        "nonexistent",
		"description": "new desc",
	})
	_, err := d.handleAgentUpdate(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestHandleAgentUpdate_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleAgentUpdate(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func mustCreateAgentFile(t *testing.T, agentsDir string, name string) {
	t.Helper()
	_ = os.MkdirAll(agentsDir, 0755)
	content := fmt.Sprintf(`---
name: %s
role: Test Role
description: Original description
model: test-model
skills:
  - skill-a
---

## Body Content

This is the original body.
`, name)
	filePath := filepath.Join(agentsDir, name+".md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
}

func TestHandleAgentUpdate_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agentsDir := filepath.Join(d.app.Settings().UserPreferences(), "agents")
	mustCreateAgentFile(t, agentsDir, "test-updater")

	reloaded, _ := goharnessconfig.LoadAgentsFrom(agentsDir)
	if reloaded != nil {
		d.app.SetAgentsRegistry(reloaded)
	}

	params, _ := json.Marshal(map[string]interface{}{
		"name":        "test-updater",
		"role":        "Updated Role",
		"description": "Updated description",
		"model":       "new-model",
		"skills":      []string{"skill-b", "skill-c"},
	})

	result, err := d.handleAgentUpdate(context.Background(), params)
	if err != nil {
		t.Fatalf("handleAgentUpdate error = %v", err)
	}

	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", result)
	}
	if m["status"] != "ok" {
		t.Errorf("status = %s, want ok", m["status"])
	}
	if m["agent_name"] != "test-updater" {
		t.Errorf("agent_name = %s, want test-updater", m["agent_name"])
	}

	filePath := filepath.Join(agentsDir, "test-updater.md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Updated description") {
		t.Error("file should contain updated description")
	}
	if !strings.Contains(content, "new-model") {
		t.Error("file should contain updated model")
	}
	if strings.Contains(content, "Original description") {
		t.Error("file should NOT contain original description")
	}
}

func TestHandleAgentUpdate_PartialFieldsOnly(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agentsDir := filepath.Join(d.app.Settings().UserPreferences(), "agents")
	mustCreateAgentFile(t, agentsDir, "partial-agent")

	reloaded, _ := goharnessconfig.LoadAgentsFrom(agentsDir)
	if reloaded != nil {
		d.app.SetAgentsRegistry(reloaded)
	}

	params, _ := json.Marshal(map[string]interface{}{
		"name":        "partial-agent",
		"description": "Only description changed",
	})

	result, err := d.handleAgentUpdate(context.Background(), params)
	if err != nil {
		t.Fatalf("handleAgentUpdate error = %v", err)
	}
	_ = result

	cfg := d.app.Agents().Get("partial-agent")
	if cfg == nil {
		t.Fatal("agent should still exist after partial update")
	}
	if cfg.Description != "Only description changed" {
		t.Errorf("description = %s, want 'Only description changed'", cfg.Description)
	}
	if cfg.Role != "Test Role" {
		t.Errorf("role should remain unchanged, got %s", cfg.Role)
	}
	if cfg.Model != "test-model" {
		t.Errorf("model should remain unchanged, got %s", cfg.Model)
	}
}

// ==========================================================================
// Model RPC Handlers
// ==========================================================================

func TestHandleModelList_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleModelList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleModelList error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleModelGet_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleModelGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleModelGet_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"name": "nonexistent-model"})
	_, err := d.handleModelGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

// ==========================================================================
// Skill RPC Handlers
// ==========================================================================

func TestHandleSkillList_NoReactor(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleSkillList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleSkillList error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result (empty slice)")
	}
}

func TestHandleSkillList_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleSkillList(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleSkillGet_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleSkillGet(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}
