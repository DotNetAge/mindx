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
	"github.com/DotNetAge/goharness/logging"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
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
	info, err := goharnesssession.CreateSession(context.Background(), sessDB, agentName)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	msg := goharnesssession.Message{
		Role:      "user",
		Content:   "init",
		Timestamp: time.Now().UnixMilli(),
	}
	sess, loadErr := goharnesssession.Load(context.Background(), info.SessionID, agentName, sessDB, logging.DefaultLogger())
	if loadErr != nil {
		t.Fatalf("load session: %v", loadErr)
	}
	_ = sess.Append(context.Background(), msg)
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
	msgs, ok := m["messages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected messages to be []map[string]interface{}, got %T", m["messages"])
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

	meta, ok := result.(*goharnesssession.SessionInfo)
	if !ok {
		t.Fatalf("expected *SessionInfo, got %T", result)
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

	// agent.Model 归一化要求目标模型真实存在（无 provider，组合串退化为裸名"new-model"）。
	d.app.Models().Register("new-model", &goharnessconfig.ModelConfig{Name: "new-model"})

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

// ==========================================================================
// User Config
// ==========================================================================

func TestHandleUserConfig_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleUserConfig(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleUserConfig error = %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	if _, exists := m["initialized"]; !exists {
		t.Error("expected 'initialized' in result")
	}
}

// ==========================================================================
// I18n RPC Handlers
// ==========================================================================

func TestHandleI18nGet_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleI18nGet(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleI18nGet error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if _, exists := m["tag"]; !exists {
		t.Error("expected 'tag' in result")
	}
	if _, exists := m["name"]; !exists {
		t.Error("expected 'name' in result")
	}
}

func TestHandleI18nList_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleI18nList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleI18nList error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if _, exists := m["languages"]; !exists {
		t.Error("expected 'languages' in result")
	}
	if _, exists := m["current"]; !exists {
		t.Error("expected 'current' in result")
	}
}

func TestHandleI18nSwitch_EmptyLang(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"lang": ""})
	_, err := d.handleI18nSwitch(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for empty lang")
	}
}

func TestHandleI18nSwitch_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleI18nSwitch(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ==========================================================================
// Server RPC Handlers
// ==========================================================================

func TestHandleServerVersion_NotSet(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// In test builds, core.Version is empty, so this should return an error.
	_, err := d.handleServerVersion(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when core.Version is empty")
	}
}

// ==========================================================================
// Schedule RPC Handlers
// ==========================================================================

func TestHandleScheduleList_Empty(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleScheduleList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleScheduleList error = %v", err)
	}
	entries, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHandleScheduleAdd_MissingAgent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"content": "test", "cron_expr": "* * * * *"})
	_, err := d.handleScheduleAdd(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestHandleScheduleAdd_MissingContent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"agent": "a", "cron_expr": "* * * * *"})
	_, err := d.handleScheduleAdd(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestHandleScheduleAdd_MissingCronExpr(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"agent": "a", "content": "test"})
	_, err := d.handleScheduleAdd(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing cron_expr")
	}
}

func TestHandleScheduleAdd_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleScheduleAdd(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleScheduleDelete_MissingID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleScheduleDelete(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleScheduleDelete_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleScheduleDelete(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ==========================================================================
// Log RPC Handlers
// ==========================================================================

func TestHandleLog_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleLogRead(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleLogClear_NotConfirmed(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]bool{"confirmed": false})
	_, err := d.handleLogClear(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when confirmed is false")
	}
}

func TestHandleLogClear_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleLogClear(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleLogCount_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleLogCount(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleLogCount error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if _, exists := m["counts"]; !exists {
		t.Error("expected 'counts' in result")
	}
}

// ==========================================================================
// Provider RPC Handlers (defined in handler_model.go)
// ==========================================================================

func TestHandleProviderList_OK(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleProviderList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleProviderList error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleProviderCreate_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleProviderCreate(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleProviderCreate_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleProviderCreate(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleProviderUpdate_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleProviderUpdate(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleProviderUpdate_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleProviderUpdate(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleProviderDelete_MissingName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{})
	_, err := d.handleProviderDelete(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestHandleProviderDelete_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"name": "nonexistent"})
	_, err := d.handleProviderDelete(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestHandleProviderDelete_InvalidJSON(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleProviderDelete(context.Background(), json.RawMessage("bad"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ==========================================================================
// Graph RPC Handlers — 图数据库 CRUD 测试
// ==========================================================================

func TestHandleGraphUpsertAndQueryNodes(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.graphStore == nil {
		t.Skip("graph store not available")
	}

	// 1. Upsert 两个节点
	params, _ := json.Marshal(rpc.GraphUpsertNodesParams{
		Nodes: []rpc.GraphNodeParam{
			{ID: "n1", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice", "age": 30}},
			{ID: "n2", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob", "age": 25}},
		},
	})
	result, err := d.handleGraphUpsertNodes(context.Background(), params)
	if err != nil {
		t.Fatalf("upsert nodes error = %v", err)
	}
	m := result.(map[string]interface{})
	if m["upserted"] != 2 {
		t.Errorf("expected 2 upserted, got %v", m["upserted"])
	}

	// 2. ListNodes 验证
	result, err = d.handleGraphListNodes(context.Background(), nil)
	if err != nil {
		t.Fatalf("list nodes error = %v", err)
	}
	nodes := result.([]map[string]interface{})
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// 3. GetNode 按 ID 查询
	getParams, _ := json.Marshal(rpc.GraphGetNodeParams{ID: "n1"})
	result, err = d.handleGraphGetNode(context.Background(), getParams)
	if err != nil {
		t.Fatalf("get node error = %v", err)
	}
	node := result.(map[string]interface{})
	if node["id"] != "n1" {
		t.Errorf("expected id n1, got %v", node["id"])
	}

	// 4. GraphQuery 用 Cypher 查询
	queryParams, _ := json.Marshal(rpc.GraphQueryParams{
		Query:  "MATCH (n:Person) WHERE n.age > $minAge RETURN n.name AS name, n.age AS age",
		Params: map[string]interface{}{"minAge": 28},
	})
	result, err = d.handleGraphQuery(context.Background(), queryParams)
	if err != nil {
		t.Fatalf("graph query error = %v", err)
	}
	qResult := result.(map[string]interface{})
	rows := qResult["rows"].([]map[string]interface{})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (Alice > 28), got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", rows[0]["name"])
	}
}

func TestHandleGraphUpsertAndQueryEdges(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.graphStore == nil {
		t.Skip("graph store not available")
	}

	// 先创建两个节点
	nodeParams, _ := json.Marshal(rpc.GraphUpsertNodesParams{
		Nodes: []rpc.GraphNodeParam{
			{ID: "e1", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice"}},
			{ID: "e2", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob"}},
		},
	})
	_, err := d.handleGraphUpsertNodes(context.Background(), nodeParams)
	if err != nil {
		t.Fatalf("setup nodes error = %v", err)
	}

	// 1. Upsert 一条边
	edgeParams, _ := json.Marshal(rpc.GraphUpsertEdgesParams{
		Edges: []rpc.GraphEdgeParam{
			{FromNodeID: "e1", ToNodeID: "e2", Type: "KNOWS", Properties: map[string]interface{}{"since": 2020}},
		},
	})
	_, err = d.handleGraphUpsertEdges(context.Background(), edgeParams)
	if err != nil {
		t.Fatalf("upsert edges error = %v", err)
	}

	// 2. ListEdges 验证
	result, err := d.handleGraphListEdges(context.Background(), nil)
	if err != nil {
		t.Fatalf("list edges error = %v", err)
	}
	edges := result.([]map[string]interface{})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0]["type"] != "KNOWS" {
		t.Errorf("expected KNOWS, got %v", edges[0]["type"])
	}

	// 3. GetNeighbors 验证
	neighborParams, _ := json.Marshal(rpc.GraphGetNeighborsParams{ID: "e1", Depth: 1, Limit: 10})
	result, err = d.handleGraphGetNeighbors(context.Background(), neighborParams)
	if err != nil {
		t.Fatalf("get neighbors error = %v", err)
	}
	neighbors := result.([]map[string]interface{})
	if len(neighbors) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(neighbors))
	}
	node := neighbors[0]["node"].(map[string]interface{})
	if node["id"] != "e2" {
		t.Errorf("expected neighbor e2, got %v", node["id"])
	}
}

func TestHandleGraphExec_CypherWrite(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.graphDB == nil {
		t.Skip("graph db not available")
	}

	// Exec Cypher: 创建节点
	execParams, _ := json.Marshal(rpc.GraphQueryParams{
		Query:  "CREATE (n:TestNode {name: $name}) RETURN n",
		Params: map[string]interface{}{"name": "test"},
	})
	_, err := d.handleGraphExec(context.Background(), execParams)
	if err != nil {
		t.Fatalf("graph exec error = %v", err)
	}

	// 用 Query 验证节点已被创建
	queryParams, _ := json.Marshal(rpc.GraphQueryParams{
		Query: "MATCH (n:TestNode) RETURN n.name AS name",
	})
	qr, err := d.handleGraphQuery(context.Background(), queryParams)
	if err != nil {
		t.Fatalf("graph query after exec error = %v", err)
	}
	qResult := qr.(map[string]interface{})
	rows := qResult["rows"].([]map[string]interface{})
	if len(rows) != 1 || rows[0]["name"] != "test" {
		t.Errorf("expected 1 row with name=test, got %v", rows)
	}
}

func TestHandleGraph_NilGuard(t *testing.T) {
	// graphDB/graphStore 为 nil 时的守卫测试
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 模拟 graphDB 为 nil 的场景
	d.graphDB = nil
	d.graphStore = nil

	// 所有 graph.* handler 应返回 error
	handlers := []struct {
		name   string
		fn     func(ctx context.Context, params json.RawMessage) (any, error)
		params json.RawMessage
	}{
		{"graph.query", d.handleGraphQuery, mustJSON(t, rpc.GraphQueryParams{Query: "MATCH (n) RETURN n"})},
		{"graph.exec", d.handleGraphExec, mustJSON(t, rpc.GraphQueryParams{Query: "CREATE (n)"})},
		{"graph.upsert_nodes", d.handleGraphUpsertNodes, mustJSON(t, rpc.GraphUpsertNodesParams{Nodes: []rpc.GraphNodeParam{{ID: "x", Labels: []string{"X"}}}})},
		{"graph.upsert_edges", d.handleGraphUpsertEdges, mustJSON(t, rpc.GraphUpsertEdgesParams{Edges: []rpc.GraphEdgeParam{{FromNodeID: "a", ToNodeID: "b", Type: "X"}}})},
		{"graph.get_node", d.handleGraphGetNode, mustJSON(t, rpc.GraphGetNodeParams{ID: "x"})},
		{"graph.get_neighbors", d.handleGraphGetNeighbors, mustJSON(t, rpc.GraphGetNeighborsParams{ID: "x"})},
		{"graph.list_nodes", d.handleGraphListNodes, nil},
		{"graph.list_edges", d.handleGraphListEdges, nil},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			_, err := h.fn(context.Background(), h.params)
			if err == nil {
				t.Error("expected error when graphDB is nil, got nil")
			}
		})
	}
}

func TestHandleGraph_ValidationErrors(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.graphStore == nil {
		t.Skip("graph store not available")
	}

	t.Run("missing query", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphQueryParams{Query: ""})
		_, err := d.handleGraphQuery(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty query")
		}
	})

	t.Run("missing exec query", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphQueryParams{Query: ""})
		_, err := d.handleGraphExec(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty query")
		}
	})

	t.Run("missing node id", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphGetNodeParams{ID: ""})
		_, err := d.handleGraphGetNode(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty id")
		}
	})

	t.Run("missing neighbor id", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphGetNeighborsParams{ID: ""})
		_, err := d.handleGraphGetNeighbors(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty id")
		}
	})

	t.Run("empty nodes upsert", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphUpsertNodesParams{Nodes: []rpc.GraphNodeParam{}})
		_, err := d.handleGraphUpsertNodes(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty nodes")
		}
	})

	t.Run("empty edges upsert", func(t *testing.T) {
		params, _ := json.Marshal(rpc.GraphUpsertEdgesParams{Edges: []rpc.GraphEdgeParam{}})
		_, err := d.handleGraphUpsertEdges(context.Background(), params)
		if err == nil {
			t.Error("expected error for empty edges")
		}
	})
}

// mustJSON is a test helper that marshals v to json.RawMessage.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return json.RawMessage(data)
}

// ==========================================================================
// Session Pricing — buildSessionPricing
// ==========================================================================

func TestBuildSessionPricing_DefaultFallback(t *testing.T) {
	// newTestDaemon 使用空 models.yml：无任何模型配置，
	// 定价必须回退到默认价格，而不是零值（否则费用恒为 0）。
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	pricing := d.buildSessionPricing()
	if pricing.InputPricePer1M != core.DefaultInputCost {
		t.Errorf("InputPricePer1M = %v, want %v", pricing.InputPricePer1M, core.DefaultInputCost)
	}
	if pricing.OutputPricePer1M != core.DefaultOutputCost {
		t.Errorf("OutputPricePer1M = %v, want %v", pricing.OutputPricePer1M, core.DefaultOutputCost)
	}
}

func TestBuildSessionPricing_ModelCost(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 注册带价格的模型并设为当前默认模型
	d.app.Models().Register("test-model", &goharnessconfig.ModelConfig{
		Name:         "test-model",
		Provider:     "test-provider",
		CostPer1MIn:  8.5,
		CostPer1MOut: 20.0,
	})
	d.app.Config().DefaultModel = "test-model"
	d.app.Config().LastModel = "test-model"

	pricing := d.buildSessionPricing()
	if pricing.InputPricePer1M != 8.5 {
		t.Errorf("InputPricePer1M = %v, want 8.5", pricing.InputPricePer1M)
	}
	if pricing.OutputPricePer1M != 20.0 {
		t.Errorf("OutputPricePer1M = %v, want 20.0", pricing.OutputPricePer1M)
	}
}

func TestBuildSessionPricing_SwitchesWithModel(t *testing.T) {
	// 切换默认模型后，定价必须立即跟随新模型，
	// 而不是沿用启动时固化的模型（旧实现通过 d.modelName 固化导致价格过期）。
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	d.app.Models().Register("model-a", &goharnessconfig.ModelConfig{
		Name: "model-a", Provider: "test-provider", CostPer1MIn: 1.0, CostPer1MOut: 2.0,
	})
	d.app.Models().Register("model-b", &goharnessconfig.ModelConfig{
		Name: "model-b", Provider: "test-provider", CostPer1MIn: 5.0, CostPer1MOut: 10.0,
	})
	d.app.Config().DefaultModel = "model-a"
	d.app.Config().LastModel = "model-a"
	if p := d.buildSessionPricing(); p.InputPricePer1M != 1.0 {
		t.Fatalf("pricing for model-a InputPricePer1M = %v, want 1.0", p.InputPricePer1M)
	}

	d.app.Config().DefaultModel = "model-b"
	d.app.Config().LastModel = "model-b"
	if p := d.buildSessionPricing(); p.InputPricePer1M != 5.0 {
		t.Fatalf("pricing after switch InputPricePer1M = %v, want 5.0", p.InputPricePer1M)
	}
}
