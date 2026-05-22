package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/goreact/reactor"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/google/uuid"
)

var atAgentRegex = regexp.MustCompile(`^@([\w-]+)(?:\s+([\w-]+))?\s+(.+)$`)

type Daemon struct {
	app          *core.App
	gw           *gateway.Server
	scheduler    *scheduler.Scheduler
	schedulerDB  *scheduler.FileSchedulerStore
	memoryWatch  *memory.FileWatchService
	sharedMemory *memory.RAGMemory
	addr         string
	wsPath       string
	logger       logging.Logger
}

func NewDaemon(app *core.App, addr, wsPath string) *Daemon {
	logDir := logging.ResolveLogDir()
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(logDir, "mindx-daemon.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})

	schedulerDB, _ := scheduler.NewFileSchedulerStore(app.Settings().SchedulesDir())

	// ── File Watch Service (LongTerm memory monitoring) ──
	var memoryWatch *memory.FileWatchService
	var sharedMemory *memory.RAGMemory
	if emb := app.Embedder(); emb != nil {
		sharedMem, memErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: goreactcore.MemoryTypeLongTerm,
			AgentName:  "_shared",
			MemoryDir:  filepath.Join(app.Settings().UserPreferences(), "memory"),
			Embedder:   emb,
		})
		if memErr != nil {
			logger.Warn("filewatch: failed to create shared LongTerm indexer, watch disabled", "error", memErr)
		} else {
			sharedMemory = sharedMem
			watchList, wlErr := memory.NewWatchListStore(app.Settings().DataDir())
			if wlErr != nil {
				logger.Warn("filewatch: failed to create watchlist store, watch disabled", "error", wlErr)
			} else {
				memoryWatch = memory.NewFileWatchService(
					sharedMem.Indexer(),
					watchList,
					filepath.Join(app.Settings().DataDir(), "memory-cache"),
					logger,
				)
			}
		}
	}

	d := &Daemon{
		app:          app,
		addr:         addr,
		wsPath:       wsPath,
		schedulerDB:  schedulerDB,
		memoryWatch:  memoryWatch,
		sharedMemory: sharedMemory,
		logger:       logger,
	}

	d.scheduler = scheduler.NewScheduler(schedulerDB, d.executeScheduleCommand, logger)
	return d
}

func (d *Daemon) Start(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("Scheduler failed to start", "error", err)
		}
	}

	if d.memoryWatch != nil {
		go func() {
			if err := d.memoryWatch.Start(ctx); err != nil {
				d.logger.Warn("FileWatch service exited with error", "error", err)
			}
		}()
	}

	d.logger.Info("MindX daemon starting", "addr", fmt.Sprintf("ws://localhost%s%s", d.addr, d.wsPath))

	if err := d.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	<-ctx.Done()
	d.logger.Info("Shutting down")

	if d.memoryWatch != nil {
		d.memoryWatch.Stop()
	}

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) TestStart(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("scheduler failed to start", "error", err)
		}
	}

	if err := d.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	return nil
}

func (d *Daemon) TestStop(ctx context.Context) error {
	if d.gw == nil {
		return nil
	}

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) initGateway() {
	d.gw = gateway.New(
		gateway.WithAddr(d.addr),
		gateway.WithPath(d.wsPath),
		gateway.WithHandler(d.defaultHandler),
	)
	d.registerRPCMethods()
}

func (d *Daemon) registerRPCMethods() {
	d.gw.RegisterMethod("session.list", d.handleSessionList)
	d.gw.RegisterMethod("session.get", d.handleSessionGet)
	d.gw.RegisterMethod("session.meta", d.handleSessionMeta)

	d.gw.RegisterMethod("memory.query", d.handleMemoryQuery)
	d.gw.RegisterMethod("memory.store", d.handleMemoryStore)
	d.gw.RegisterMethod("memory.delete", d.handleMemoryDelete)

	d.gw.RegisterMethod("agent.list", d.handleAgentList)
	d.gw.RegisterMethod("agent.get", d.handleAgentGet)
	d.gw.RegisterMethod("agent.update", d.handleAgentUpdate)

	d.gw.RegisterMethod("model.list", d.handleModelList)
	d.gw.RegisterMethod("model.get", d.handleModelGet)

	d.gw.RegisterMethod("skill.list", d.handleSkillList)
	d.gw.RegisterMethod("skill.get", d.handleSkillGet)
}

// ---------------------------------------------------------------------------
// Session RPC Handlers
// ---------------------------------------------------------------------------

type sessionListParams struct {
	Agent string `json:"agent,omitempty"`
}

func (d *Daemon) handleSessionList(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionListParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	sessions, err := sessDB.ListSessions(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}

	if p.Agent != "" {
		filtered := make([]goreactcore.SessionInfo, 0)
		for i := range sessions {
			if sessions[i].AgentName == p.Agent {
				filtered = append(filtered, sessions[i])
			}
		}
		sessions = filtered
	}

	return sessions, nil
}

type sessionGetParams struct {
	SessionID string `json:"session_id"`
}

func (d *Daemon) handleSessionGet(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	info, err := sessDB.Get(context.Background(), p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q failed: %w", p.SessionID, err)
	}

	meta, metaErr := sessDB.GetSessionMeta(p.SessionID)

	result := map[string]interface{}{
		"session_id": p.SessionID,
		"messages":   info,
	}
	if metaErr == nil && meta != nil {
		result["meta"] = meta
	}
	return result, nil
}

func (d *Daemon) handleSessionMeta(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	meta, err := sessDB.GetSessionMeta(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session meta %q failed: %w", p.SessionID, err)
	}

	return meta, nil
}

// ---------------------------------------------------------------------------
// Memory RPC Handlers
// ---------------------------------------------------------------------------

type memoryQueryParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	Type     string  `json:"type,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

func (d *Daemon) handleMemoryQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryQueryParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	opts := []goreactcore.RetrieveOption{}
	if p.Limit > 0 {
		opts = append(opts, goreactcore.WithMemoryLimit(p.Limit))
	}
	if p.MinScore > 0 {
		opts = append(opts, goreactcore.WithMinScore(p.MinScore))
	}
	if p.Type != "" {
		switch p.Type {
		case "longterm":
			opts = append(opts, goreactcore.WithMemoryTypes(goreactcore.MemoryTypeLongTerm))
		case "session":
			opts = append(opts, goreactcore.WithMemoryTypes(goreactcore.MemoryTypeSession))
		}
	}

	records, err := mem.Retrieve(context.Background(), p.Query, opts...)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	if records == nil {
		return []goreactcore.MemoryRecord{}, nil
	}
	return records, nil
}

type memoryStoreParams struct {
	Title   string   `json:"title,omitempty"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Type    string   `json:"type,omitempty"`
}

func (d *Daemon) handleMemoryStore(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryStoreParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	record := goreactcore.MemoryRecord{
		Title:     p.Title,
		Content:   p.Content,
		Tags:      p.Tags,
		CreatedAt: time.Now(),
	}
	if p.Type == "session" {
		record.Type = goreactcore.MemoryTypeSession
	} else {
		record.Type = goreactcore.MemoryTypeLongTerm
	}

	id, err := mem.Store(context.Background(), record)
	if err != nil {
		return nil, fmt.Errorf("memory store failed: %w", err)
	}

	return map[string]string{"id": id}, nil
}

type memoryDeleteParams struct {
	ID string `json:"id"`
}

func (d *Daemon) handleMemoryDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryDeleteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	if err := mem.Delete(context.Background(), p.ID); err != nil {
		return nil, fmt.Errorf("memory delete failed: %w", err)
	}

	return map[string]string{"status": "ok", "deleted_id": p.ID}, nil
}

// ---------------------------------------------------------------------------
// Agent RPC Handlers
// ---------------------------------------------------------------------------

func (d *Daemon) handleAgentList(_ context.Context, params json.RawMessage) (any, error) {
	agents := d.app.Agents()
	if agents == nil {
		return []goreactcore.AgentConfig{}, nil
	}
	list := agents.List()
	if list == nil {
		return []goreactcore.AgentConfig{}, nil
	}

	type agentEntry struct {
		Name                string         `json:"name"`
		Role                string         `json:"role,omitempty"`
		Description         string         `json:"description"`
		Introduction        string         `json:"introduction,omitempty"`
		Model               string         `json:"model"`
		Skills              []string       `json:"skills,omitempty"`
		Body                string         `json:"body,omitempty"`
		EnableOrchestration bool           `json:"enable_orchestration"`
		MaxDecomposeDepth   int            `json:"max_decompose_depth,omitempty"`
		Meta                map[string]any `json:"meta,omitempty"`
	}

	result := make([]agentEntry, len(list))
	for i, a := range list {
		result[i] = agentEntry{
			Name:                a.Name,
			Role:                a.Role,
			Description:         a.Description,
			Introduction:        a.Introduction,
			Model:               a.Model,
			Skills:              a.Skills,
			Body:                a.Body,
			EnableOrchestration: a.EnableOrchestration,
			MaxDecomposeDepth:   a.MaxDecomposeDepth,
			Meta:                a.Meta,
		}
	}
	return result, nil
}

type agentGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleAgentGet(_ context.Context, params json.RawMessage) (any, error) {
	var p agentGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
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

type agentUpdateParams struct {
	Name                string         `json:"name"`
	Role                string         `json:"role,omitempty"`
	Description         string         `json:"description,omitempty"`
	Introduction        string         `json:"introduction,omitempty"`
	Model               string         `json:"model,omitempty"`
	Skills              []string       `json:"skills,omitempty"`
	Body                string         `json:"body,omitempty"`
	EnableOrchestration *bool          `json:"enable_orchestration,omitempty"`
	MaxDecomposeDepth   *int           `json:"max_decompose_depth,omitempty"`
	Meta                map[string]any `json:"meta,omitempty"`
}

func (d *Daemon) handleAgentUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentUpdateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
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
	if p.Introduction != "" {
		updated.Introduction = p.Introduction
	}
	if p.Body != "" {
		updated.Body = p.Body
	}
	if p.EnableOrchestration != nil {
		updated.EnableOrchestration = *p.EnableOrchestration
	}
	if p.MaxDecomposeDepth != nil {
		updated.MaxDecomposeDepth = *p.MaxDecomposeDepth
	}
	if p.Meta != nil {
		updated.Meta = p.Meta
	}

	if err := agents.SaveTo(&updated); err != nil {
		return nil, fmt.Errorf("failed to save agent config: %w", err)
	}

	return map[string]string{
		"status":      "ok",
		"agent_name":  updated.Name,
		"message":     "agent config updated",
	}, nil
}

// ---------------------------------------------------------------------------
// Model RPC Handlers
// ---------------------------------------------------------------------------

func (d *Daemon) handleModelList(_ context.Context, _ json.RawMessage) (any, error) {
	models := d.app.Models()
	if models == nil {
		return []goreactcore.ModelConfig{}, nil
	}
	list := models.List()
	if list == nil {
		return []goreactcore.ModelConfig{}, nil
	}
	return list, nil
}

type modelGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleModelGet(_ context.Context, params json.RawMessage) (any, error) {
	var p modelGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	models := d.app.Models()
	if models == nil {
		return nil, fmt.Errorf("model registry not available")
	}

	cfg := models.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	return cfg, nil
}

// ---------------------------------------------------------------------------
// Skill RPC Handlers
// ---------------------------------------------------------------------------

type skillListParams struct {
	AgentName string `json:"agent_name,omitempty"`
}

func (d *Daemon) handleSkillList(_ context.Context, params json.RawMessage) (any, error) {
	var p skillListParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	agentName := p.AgentName
	if agentName == "" {
		agentName = d.app.CurrentAgentName()
	}

	m, err := d.app.CurrentAgent()
	if err != nil {
		return []goreactcore.Skill{}, nil
	}
	if m.Reactor() == nil {
		return []goreactcore.Skill{}, nil
	}
	skills := m.Reactor().SkillRegistry().ListSkills()

	type skillEntry struct {
		Name         string            `json:"name"`
		Description  string            `json:"description"`
		RootDir      string            `json:"root_dir,omitempty"`
		Source       string            `json:"source,omitempty"`
		Instructions string            `json:"instructions,omitempty"`
		Paths        []string          `json:"paths,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
	}

	result := make([]skillEntry, len(skills))
	for i, s := range skills {
		result[i] = skillEntry{
			Name:         s.Name,
			Description:  s.Description,
			RootDir:      s.RootDir,
			Source:       s.Source,
			Instructions: s.Instructions,
			Paths:        s.Paths,
			Metadata:     s.Metadata,
		}
	}
	return result, nil
}

type skillGetParams struct {
	Name      string `json:"name"`
	AgentName string `json:"agent_name,omitempty"`
}

func (d *Daemon) handleSkillGet(_ context.Context, params json.RawMessage) (any, error) {
	var p skillGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	m, err := d.app.CurrentAgent()
	if err != nil {
		return nil, fmt.Errorf("current agent not available: %w", err)
	}
	if m.Reactor() == nil {
		return nil, fmt.Errorf("reactor not initialized for current agent")
	}

	skill, err := m.Reactor().SkillRegistry().GetSkill(p.Name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", p.Name, err)
	}

	return skill, nil
}

func (d *Daemon) executeScheduleCommand(ctx context.Context, agent string, sessionID string, content string, projectDir string) error {
	originalCWD, _ := os.Getwd()

	targetDir := projectDir
	if targetDir == "" {
		meta := d.restoreSessionEnvironment(sessionID)
		if meta != nil {
			targetDir = meta.ProjectWorkingDir
		}
	}

	if targetDir != "" {
		if err := os.Chdir(targetDir); err != nil {
			d.logger.Warn("failed to chdir to project dir, using current dir",
				"project_dir", targetDir,
				"error", err,
			)
		} else {
			defer func() {
				if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
					d.logger.Warn("failed to restore cwd after scheduled task",
						"original", originalCWD,
						"error", restoreErr,
					)
				}
			}()
			os.Setenv("MINDX_PROJECT_DIR", targetDir)
			os.Setenv("MINDX_SESSION_ID", sessionID)
			d.logger.Info("set execution context for scheduled task",
				"session_id", sessionID,
				"project_dir", targetDir,
				"original_cwd", originalCWD,
			)
		}
	}

	resolvedAgent, err := d.app.ResolveAgent(agent)
	if err != nil {
		return fmt.Errorf("resolve agent %q: %w", agent, err)
	}
	if sessionID == "" || sessionID == "new" {
		sessionID = generateSessionID()
	}
	_, err = resolvedAgent.Ask(sessionID, content)
	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
	}
	return nil
}

// restoreSessionEnvironment loads session metadata and restores the project directory.
// Returns nil if the session metadata cannot be found (e.g., sessions created before this feature).
func (d *Daemon) restoreSessionEnvironment(sessionID string) *session.SessionMeta {
	if d.app == nil || d.app.SessDB() == nil {
		return nil
	}
	meta, err := d.app.SessDB().GetSessionMeta(sessionID)
	if err != nil {
		d.logger.Debug("could not load session meta for scheduled task",
			"session_id", sessionID,
			"error", err,
		)
		return nil
	}
	return meta
}

func (d *Daemon) defaultHandler(msg *gateway.Message) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.Text == "" {
		d.logger.Warn("defaultHandler: missing or invalid text field",
			"data", string(msg.Data), "error", err)
		return
	}
	text := payload.Text

	agentName, providedSessionID, content := parseAgentTarget(text)

	agent, err := d.app.ResolveAgent(agentName)
	if err != nil {
		d.sendEvent(msg.ClientID, msg.SessionID, gateway.RespError, "错误", err.Error())
		return
	}

	sessionID := d.resolveSessionID(msg.SessionID, providedSessionID)
	resolvedAgentName := agentName
	if resolvedAgentName == "" {
		resolvedAgentName = agent.Name()
	}

	d.logger.Info("request start",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
		"input_preview", truncate(content, 100),
	)

	eventCh, cancelEvents := agent.EventsFiltered(func(e goreactcore.ReactEvent) bool {
		switch e.Type {
		case goreactcore.ThinkingDelta, goreactcore.ThinkingDone, goreactcore.ActionStart,
			goreactcore.ActionProgress, goreactcore.ToolExecStart, goreactcore.ToolExecEnd,
			goreactcore.ActionEnd, goreactcore.FinalAnswer,
			goreactcore.ExecutionSummary, goreactcore.Error, goreactcore.SubtaskSpawned,
			goreactcore.SubtaskCompleted, goreactcore.ClarifyNeeded, goreactcore.PermissionRequest,
			goreactcore.PermissionDenied, goreactcore.CycleEnd, goreactcore.TaskSummary:
			return true
		default:
			return false
		}
	})
	defer cancelEvents()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range eventCh {
			d.forwardEvent(msg.ClientID, event)
		}
	}()

	_, err = agent.Ask(sessionID, content)
	if err != nil {
		d.logger.Error("request failed", err,
			"client_id", msg.ClientID,
			"session_id", sessionID,
			"agent", resolvedAgentName,
		)
		d.sendEvent(msg.ClientID, sessionID, gateway.RespError, "错误", err.Error())
	}

	<-done
	d.logger.Info("request done",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)
}

func parseAgentTarget(text string) (agentName string, sessionID string, content string) {
	matches := atAgentRegex.FindStringSubmatch(text)
	if len(matches) >= 4 {
		agentName = matches[1]
		sessionID = matches[2]
		content = matches[3]
		return
	}
	if len(matches) == 2 {
		agentName = matches[1]
		content = strings.TrimPrefix(text, matches[0])
		return
	}
	return "", "", text
}

func (d *Daemon) resolveSessionID(clientProvided string, commandProvided string) string {
	if commandProvided != "" && commandProvided != "new" {
		d.logger.Info("using session_id from command", "session", commandProvided)
		return commandProvided
	}
	if clientProvided != "" {
		return clientProvided
	}

	if d.app.SessionDB() != nil {
		currentAgent := d.app.CurrentAgentName()
		if currentAgent != "" {
			sid, err := d.app.SessionDB().GetByRole(context.Background(), currentAgent)
			if err == nil && sid != nil && sid.SessionID != "" {
				d.logger.Info("resumed session from store", "agent", currentAgent, "session", sid.SessionID)
				return sid.SessionID
			}
		}
	}

	sid := generateSessionID()
	d.logger.Info("created new session", "session", sid)
	return sid
}

func (d *Daemon) forwardEvent(clientID string, event goreactcore.ReactEvent) {
	sid := event.SessionID
	switch event.Type {
	case goreactcore.ThinkingDelta:
		text, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ThinkingDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespThinkingDelta, "思考中", text, gateway.WithSessionID(sid))

	case goreactcore.ThinkingDone:
		thought, ok := event.Data.(*reactor.Thought)
		if !ok {
			d.logger.Warn("unexpected ThinkingDone data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildThinkingDoneMarkdown(*thought)
		d.sendEvent(clientID, sid, gateway.RespThinkingDone, "思考完成", md)

	case goreactcore.ActionStart:
		action, ok := event.Data.(goreactcore.ActionStartData)
		if !ok {
			d.logger.Warn("unexpected ActionStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionStart, "开始操作", map[string]interface{}{
			"tool_count":       action.ToolCount,
			"tool_names":       action.ToolNames,
			"predicted_tokens": action.TotalPredictedTokens,
			"iteration":        action.Iteration,
		}, gateway.WithSessionID(sid))

	case goreactcore.ToolExecStart:
		data, ok := event.Data.(goreactcore.ToolExecStartData)
		if !ok {
			d.logger.Warn("unexpected ToolExecStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionStart, "工具开始", map[string]interface{}{
			"tool_name": data.ToolName,
			"params":    data.Params,
		}, gateway.WithSessionID(sid))

	case goreactcore.ToolExecEnd:
		data, ok := event.Data.(goreactcore.ToolExecEndData)
		if !ok {
			d.logger.Warn("unexpected ToolExecEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionResult, "工具结果", map[string]interface{}{
			"tool_name": data.ToolName,
			"success":   data.Success,
			"result":    data.Result,
			"error":     data.Error,
			"duration":  data.Duration.String(),
		}, gateway.WithSessionID(sid))

	case goreactcore.ActionProgress:
		progress, ok := event.Data.(goreactcore.ActionProgressData)
		if !ok {
			d.logger.Warn("unexpected ActionProgress data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionProgress, "操作进度", map[string]interface{}{
			"completed": progress.CompletedCount,
			"total":     progress.TotalCount,
			"status":    progress.Status,
		}, gateway.WithSessionID(sid))

	case goreactcore.ActionEnd:
		data, ok := event.Data.(goreactcore.ActionEndData)
		if !ok {
			d.logger.Warn("unexpected ActionEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionEnd, "操作完成", map[string]interface{}{
			"total":   data.TotalTools,
			"success": data.SuccessCount,
			"failed":  data.FailedCount,
			"summary": data.Summary,
		}, gateway.WithSessionID(sid))

	case goreactcore.SubtaskSpawned:
		info, ok := event.Data.(goreactcore.SubtaskInfo)
		if !ok {
			d.logger.Warn("unexpected SubtaskSpawned data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskSpawnedMarkdown(info)
		d.sendEvent(clientID, sid, gateway.RespSubtaskSpawned, "子任务生成", md)

	case goreactcore.SubtaskCompleted:
		result, ok := event.Data.(goreactcore.SubtaskResult)
		if !ok {
			d.logger.Warn("unexpected SubtaskCompleted data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskCompletedMarkdown(result)
		d.sendEvent(clientID, sid, gateway.RespSubtaskCompleted, "子任务完成", md)

	case goreactcore.FinalAnswer:
		answer, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected FinalAnswer data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespFinalAnswer, "最终答案", answer)

	case goreactcore.ClarifyNeeded:
		question, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ClarifyNeeded data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespClarifyNeeded, "需要澄清", question)

	case goreactcore.PermissionRequest:
		req, ok := event.Data.(goreactcore.PermissionRequestData)
		if !ok {
			d.logger.Warn("unexpected PermissionRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		if len(req.Questions) > 0 {
			// AskUser-style: forward full structured data to frontend
			jsonData, _ := json.Marshal(req)
			d.sendEvent(clientID, sid, gateway.RespPermissionRequest, "需要澄清", string(jsonData))
		} else {
			md := buildPermissionRequestMarkdown(req)
			d.sendEvent(clientID, sid, gateway.RespPermissionRequest, "权限请求", md)
		}

	case goreactcore.PermissionDenied:
		reason, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected PermissionDenied data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespPermissionDenied, "权限拒绝", reason)

	case goreactcore.ExecutionSummary:
		summary, ok := event.Data.(goreactcore.ExecutionSummaryData)
		if !ok {
			d.logger.Warn("unexpected ExecutionSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendExecutionSummary(clientID, sid, summary)

	case goreactcore.CycleEnd:
		cycle, ok := event.Data.(goreactcore.CycleInfo)
		if !ok {
			d.logger.Warn("unexpected CycleEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildCycleEndMarkdown(cycle)
		d.sendEvent(clientID, sid, gateway.RespCycleEnd, "循环结束", md)

	case goreactcore.TaskSummary:
		taskSummary, ok := event.Data.(goreactcore.TaskSummaryData)
		if !ok {
			d.logger.Warn("unexpected TaskSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildTaskSummaryMarkdown(taskSummary)
		d.gw.SendResponse(clientID, gateway.RespTaskSummary, "任务总结", md,
			gateway.WithSessionID(sid),
			gateway.WithResponseMeta(map[string]interface{}{
				"input_tokens":  taskSummary.InputTokens,
				"output_tokens": taskSummary.OutputTokens,
			}))

	case goreactcore.Error:
		errMsg, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected Error data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespError, "错误", errMsg)
	}
}

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string) {
	d.gw.SendResponse(clientID, respType, title, data, gateway.WithSessionID(sessionID))
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goreactcore.ExecutionSummaryData) {
	tableData := map[string]interface{}{
		"headers": []string{"Metric", "Value"},
		"rows": []map[string]string{
			{"metric": "Iterations", "value": fmt.Sprintf("%d", summary.TotalIterations)},
			{"metric": "Tool Calls", "value": fmt.Sprintf("%d", summary.ToolCalls)},
			{"metric": "Tools Used", "value": strings.Join(summary.ToolsUsed, ", ")},
			{"metric": "Duration", "value": formatDuration(summary.TotalDuration)},
			{"metric": "Tokens Used", "value": fmt.Sprintf("%d", summary.TokensUsed)},
			{"metric": "Termination", "value": summary.TerminationReason},
		},
	}
	d.gw.SendResponse(clientID, gateway.RespExecutionSummary, "执行摘要", tableData, gateway.WithSessionID(sessionID))
}

func (d *Daemon) Gateway() *gateway.Server {
	return d.gw
}

func (d *Daemon) App() *core.App {
	return d.app
}

func (d *Daemon) Scheduler() *scheduler.Scheduler {
	return d.scheduler
}

func (d *Daemon) SchedulerDB() *scheduler.FileSchedulerStore {
	return d.schedulerDB
}

func (d *Daemon) Addr() string {
	return d.addr
}

func (d *Daemon) WSPath() string {
	return d.wsPath
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%s", uuid.New().String()[:8])
}

func buildThinkingDoneMarkdown(t reactor.Thought) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### 思考完成\n\n")
	b.WriteString(fmt.Sprintf("**决策**: `%s`  **置信度**: %.0f%%\n\n", t.Decision, t.Confidence*100))
	if t.Reasoning != "" {
		b.WriteString(fmt.Sprintf("**推理**: %s\n\n", t.Reasoning))
	}
	if t.ToolCalls != nil && len(t.ToolCalls) > 0 {
		b.WriteString("**即将调用工具**:\n\n")
		for toolName, params := range t.ToolCalls {
			b.WriteString(fmt.Sprintf("- `%s` — `%v`\n", toolName, params))
		}
		b.WriteString("\n")
	}
	if t.ClarificationQuestion != "" {
		b.WriteString(fmt.Sprintf("**问题**: %s\n\n", t.ClarificationQuestion))
	}
	return b.String()
}

func buildSubtaskSpawnedMarkdown(info goreactcore.SubtaskInfo) string {
	return fmt.Sprintf("### 🌿 子任务生成: `%s`\n\n**Agent**: %s\n**描述**: %s\n", info.TaskID, info.AgentName, info.Description)
}

func buildSubtaskCompletedMarkdown(result goreactcore.SubtaskResult) string {
	var b strings.Builder
	if result.Success {
		b.WriteString(fmt.Sprintf("### ✅ 子任务完成: `%s`\n\n", result.TaskID))
		b.WriteString(fmt.Sprintf("**回答**: %s\n", truncate(result.Answer, 300)))
	} else {
		b.WriteString(fmt.Sprintf("### ❌ 子任务失败: `%s`\n\n", result.TaskID))
		b.WriteString(fmt.Sprintf("**错误**: %s\n", result.Error))
	}
	return b.String()
}

func buildPermissionRequestMarkdown(req goreactcore.PermissionRequestData) string {
	return fmt.Sprintf("### 🔒 权限请求: `%s`\n\n**原因**: %s\n**安全级别**: %d\n", req.ToolName, req.Reason, req.SecurityLevel)
}

func buildCycleEndMarkdown(cycle goreactcore.CycleInfo) string {
	return fmt.Sprintf("### 🔄 T-A-O 循环结束 (迭代 #%d, 耗时 %s)\n", cycle.Iteration, formatDuration(cycle.Duration))
}

func buildTaskSummaryMarkdown(ts goreactcore.TaskSummaryData) string {
	return fmt.Sprintf("### 📋 任务总结\n\n%s\n\n**Token**: 输入 %d / 输出 %d\n", ts.Summary, ts.InputTokens, ts.OutputTokens)
}

func formatParams(params map[string]any) string {
	if len(params) == 0 {
		return "(无)"
	}
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Sprintf("%v", params)
	}
	return truncate(string(b), 200)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(100 * time.Millisecond).String()
}

func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}
