package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/infrastructure/persistence"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SkillExecutor struct {
	skillsDir      string
	envMgr         *EnvManager
	store          persistence.Store
	logger         logging.Logger
	mu             sync.RWMutex
	skillInfos     map[string]*entity.SkillInfo
	internalSkills map[string]InternalSkillFunc
	mcpMgr         *MCPManager
}

type InternalSkillFunc func(params map[string]any) (string, error)

func NewSkillExecutor(skillsDir string, envMgr *EnvManager, store persistence.Store, mcpMgr *MCPManager, logger logging.Logger) *SkillExecutor {
	return &SkillExecutor{
		skillsDir:      skillsDir,
		envMgr:         envMgr,
		store:          store,
		logger:         logger.Named("SkillExecutor"),
		skillInfos:     make(map[string]*entity.SkillInfo),
		internalSkills: make(map[string]InternalSkillFunc),
		mcpMgr:         mcpMgr,
	}
}

func (e *SkillExecutor) SetSkillInfos(infos map[string]*entity.SkillInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skillInfos = infos
}

func (e *SkillExecutor) RegisterInternalSkill(name string, fn InternalSkillFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.internalSkills[name] = fn
	e.logger.Info(i18n.T("skill.register_internal_success"), logging.String(i18n.T("skill.name"), name))
}

func (e *SkillExecutor) Execute(name string, def *entity.SkillDef, params map[string]any) (string, error) {
	e.logger.Info(i18n.T("skill.start_execute"), logging.String(i18n.T("skill.name"), name), logging.Any(i18n.T("skill.params"), params))

	e.mu.RLock()
	_, exists := e.skillInfos[name]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	startTime := time.Now()

	if def.IsInternal {
		return e.executeInternal(name, params, startTime)
	}

	if IsMCPSkill(def) {
		return e.executeMCP(name, def, params, startTime)
	}

	return e.executeExternal(name, def, params, startTime)
}

func (e *SkillExecutor) executeInternal(name string, params map[string]any, startTime time.Time) (string, error) {
	e.mu.RLock()
	fn, exists := e.internalSkills[name]
	e.mu.RUnlock()

	if !exists {
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("internal skill not registered: %s", name)
	}

	result, err := fn(params)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		e.UpdateStats(name, false, duration)
		e.logger.Error(i18n.T("skill.execute_failed"), logging.String(i18n.T("skill.output"), result), logging.Err(err))
		return result, err
	}

	e.UpdateStats(name, true, duration)
	e.logger.Info(i18n.T("skill.execute_success"), logging.String(i18n.T("skill.output"), result))
	return result, nil
}

func (e *SkillExecutor) executeMCP(name string, def *entity.SkillDef, params map[string]any, startTime time.Time) (string, error) {
	if e.mcpMgr == nil {
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("mcp manager not initialized")
	}

	mcpMeta, ok := GetMCPSkillMetadata(def)
	if !ok {
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("invalid mcp skill metadata")
	}

	result, err := e.mcpMgr.CallTool(mcpMeta.Server, mcpMeta.Tool, params)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		e.UpdateStats(name, false, duration)
		e.logger.Error(i18n.T("skill.execute_failed"), logging.String(i18n.T("skill.output"), result), logging.Err(err))
		return result, err
	}

	e.UpdateStats(name, true, duration)
	e.logger.Info(i18n.T("skill.execute_success"), logging.String(i18n.T("skill.output"), result))
	return result, nil
}

func (e *SkillExecutor) executeExternal(name string, def *entity.SkillDef, params map[string]any, startTime time.Time) (string, error) {
	cmd, err := e.buildCommand(def, params)
	if err != nil {
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("failed to build command: %w", err)
	}

	timeout := 30 * time.Second
	if def.Timeout > 0 {
		timeout = time.Duration(def.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	skillDir := e.getSkillDir(name)
	cmd.Dir = skillDir

	env, err := e.envMgr.PrepareExecutionEnv(name, nil)
	if err != nil {
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("failed to prepare env: %w", err)
	}

	cmdEnv := make([]string, 0, len(env))
	for key, value := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = cmdEnv

	if len(params) > 0 {
		jsonParams, err := json.Marshal(params)
		if err != nil {
			e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
			return "", fmt.Errorf("failed to serialize params: %w", err)
		}
		cmd.Stdin = bytes.NewReader(jsonParams)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		var jsonOutput map[string]any
		if json.Unmarshal(output, &jsonOutput) == nil {
			e.UpdateStats(name, true, time.Since(startTime).Milliseconds())
			e.logger.Info(i18n.T("skill.execute_success_code"), logging.String(i18n.T("skill.output"), string(output)))
			return string(output), nil
		}
		e.UpdateStats(name, false, time.Since(startTime).Milliseconds())
		e.logger.Error(i18n.T("skill.execute_failed"), logging.String(i18n.T("skill.output"), string(output)), logging.Err(err))
		return string(output), fmt.Errorf("failed to execute skill: %w", err)
	}

	e.UpdateStats(name, true, time.Since(startTime).Milliseconds())
	e.logger.Info(i18n.T("skill.execute_success"), logging.String(i18n.T("skill.output"), string(output)))
	return string(output), nil
}

func (e *SkillExecutor) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	e.logger.Info(i18n.T("skill.exec_func"),
		logging.String(i18n.T("skill.function"), function.Name),
		logging.Any(i18n.T("skill.arguments"), function.Arguments))

	params := make(map[string]any)
	for k, v := range function.Arguments {
		params[k] = v
	}

	e.mu.RLock()
	info, exists := e.skillInfos[function.Name]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("skill not found: %s", function.Name)
	}

	return e.Execute(function.Name, info.Def, params)
}

func (e *SkillExecutor) buildCommand(def *entity.SkillDef, params map[string]any) (*exec.Cmd, error) {
	if def.Command == "" {
		return nil, fmt.Errorf("no command for skill")
	}

	parts := ParseCommand(def.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command format error")
	}

	skillDir := e.getSkillDir(def.Name)

	cmdPath := parts[0]
	if !strings.HasPrefix(cmdPath, "/") && !strings.HasPrefix(cmdPath, ".") {
		cmdPath = filepath.Join(skillDir, cmdPath)
	} else if strings.HasPrefix(cmdPath, "./") {
		cmdPath = filepath.Join(skillDir, cmdPath[2:])
	}

	cmd := exec.Command(cmdPath, parts[1:]...)
	cmd.Dir = skillDir

	env, err := e.envMgr.PrepareExecutionEnv(def.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare env: %w", err)
	}

	cmdEnv := make([]string, 0, len(env))
	for key, value := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = cmdEnv

	if len(params) > 0 {
		jsonParams, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize params: %w", err)
		}
		cmd.Stdin = bytes.NewReader(jsonParams)
	}

	return cmd, nil
}

func (e *SkillExecutor) getSkillDir(name string) string {
	return filepath.Join(e.skillsDir, name)
}

func (e *SkillExecutor) UpdateStats(name string, success bool, duration int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	info, exists := e.skillInfos[name]
	if !exists {
		return
	}

	if success {
		info.SuccessCount++
	} else {
		info.ErrorCount++
	}

	info.ExecutionTimes = append(info.ExecutionTimes, duration)
	if len(info.ExecutionTimes) > 100 {
		info.ExecutionTimes = info.ExecutionTimes[1:]
	}

	sum := int64(0)
	for _, t := range info.ExecutionTimes {
		sum += t
	}
	info.AvgExecutionMs = sum / int64(len(info.ExecutionTimes))

	now := time.Now()
	info.LastRunTime = &now

	if e.store != nil {
		if err := e.saveStatsToStore(name, info); err != nil {
			e.logger.Warn(i18n.T("skill.save_stats_failed"), logging.String("skill", name), logging.Err(err))
		}
	}
}

func (e *SkillExecutor) saveStatsToStore(name string, info *entity.SkillInfo) error {
	stats := entity.SkillStats{
		SuccessCount:   info.SuccessCount,
		ErrorCount:     info.ErrorCount,
		ExecutionTimes: info.ExecutionTimes,
		LastRunTime:    info.LastRunTime,
	}

	metadata, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("序列化统计数据失败: %w", err)
	}

	entry := entity.VectorEntry{
		Key:      "skill_stats:" + name,
		Vector:   []float64{},
		Metadata: metadata,
	}

	return e.store.Put(entry.Key, entry.Vector, entry.Metadata)
}

func (e *SkillExecutor) LoadStatsFromStore(name string) (*entity.SkillStats, error) {
	if e.store == nil {
		return nil, nil
	}

	entry, err := e.store.Get("skill_stats:" + name)
	if err != nil {
		return nil, nil
	}

	if entry == nil || entry.Metadata == nil {
		return nil, nil
	}

	var stats entity.SkillStats
	if err := json.Unmarshal(entry.Metadata, &stats); err != nil {
		return nil, fmt.Errorf("解析统计数据失败: %w", err)
	}

	return &stats, nil
}

func (e *SkillExecutor) LoadAllStats(skillInfos map[string]*entity.SkillInfo) {
	if e.store == nil {
		return
	}

	for name, info := range skillInfos {
		stats, err := e.LoadStatsFromStore(name)
		if err != nil {
			e.logger.Warn(i18n.T("skill.load_stats_failed"), logging.String("skill", name), logging.Err(err))
			continue
		}

		if stats != nil {
			info.SuccessCount = stats.SuccessCount
			info.ErrorCount = stats.ErrorCount
			info.ExecutionTimes = stats.ExecutionTimes
			info.LastRunTime = stats.LastRunTime

			if len(info.ExecutionTimes) > 0 {
				sum := int64(0)
				for _, t := range info.ExecutionTimes {
					sum += t
				}
				info.AvgExecutionMs = sum / int64(len(info.ExecutionTimes))
			}
		}
	}
}

func ParseCommand(cmdStr string) []string {
	var parts []string
	var current strings.Builder
	var inQuote bool

	for i := 0; i < len(cmdStr); i++ {
		c := cmdStr[i]

		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
