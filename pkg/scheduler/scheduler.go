package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// JobLifecycleInfo describes a scheduled job lifecycle event broadcast to clients.
type JobLifecycleInfo struct {
	EntryID    string `json:"entry_id"`
	RunID      string `json:"run_id"`
	Agent      string `json:"agent"`
	SessionID  string `json:"session_id"`
	Content    string `json:"content,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
	Status     string `json:"status"` // "started", "completed", "failed", "missed"
	Error      string `json:"error,omitempty"`
}

// LifecycleCallback is called when a scheduled job starts, completes, or fails.
// started 由到点触发（Daemon 据此直接拉起 Runtime 在后台执行，不依赖客户端在线）；
// completed / failed / missed 由执行链路通过 ReportResult 收尾时触发。
type LifecycleCallback func(info JobLifecycleInfo)

// EntriesChangedCallback 在任务库增删改被 watchLoop 感知后回调（注册集或
// 表达式集合发生变化时触发一次），供上层广播前端刷新（角标/列表）。
type EntriesChangedCallback func()

type Scheduler struct {
	cron        *cron.Cron
	store       *FileSchedulerStore
	entries     map[string]cron.EntryID
	cronExprs   map[string]string // 任务 ID → 注册时的 cron 表达式，reload 时比对变更
	mu          sync.RWMutex
	logger      logging.Logger
	lifecycleCb LifecycleCallback
	entriesCb   EntriesChangedCallback
}

func NewScheduler(store *FileSchedulerStore, logger logging.Logger) *Scheduler {
	if logger == nil {
		logger = logging.DefaultConsoleLogger()
	}
	c := cron.New(
		cron.WithSeconds(),
		cron.WithLogger(cron.VerbosePrintfLogger(log.New(log.Writer(), "[scheduler] ", log.LstdFlags))),
	)
	return &Scheduler{
		cron:      c,
		store:     store,
		entries:   make(map[string]cron.EntryID),
		cronExprs: make(map[string]string),
		logger:    logger,
	}
}

// OnLifecycle sets a callback that fires when a scheduled job starts, completes, or fails.
func (s *Scheduler) OnLifecycle(cb LifecycleCallback) {
	s.lifecycleCb = cb
}

// OnEntriesChanged sets a callback that fires when watchLoop detects the
// registered entry set (ids or cron expressions) changed.
func (s *Scheduler) OnEntriesChanged(cb EntriesChangedCallback) {
	s.entriesCb = cb
}

func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.reloadAll(); err != nil {
		return err
	}

	s.cron.Start()
	go s.watchLoop(ctx)
	s.logger.Info("scheduler started", "jobs", len(s.entries))
	return nil
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.logger.Info("scheduler stopped")
}

func (s *Scheduler) reloadAll() error {
	// 注册集快照：reload 结束后对比，判定是否需要通知上层（前端角标/列表刷新）
	s.mu.RLock()
	prevCount := len(s.entries)
	prevExprs := make(map[string]string, len(s.cronExprs))
	for id, expr := range s.cronExprs {
		prevExprs[id] = expr
	}
	s.mu.RUnlock()

	entries, err := s.store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load schedules from store: %w", err)
	}

	fileIDs := make(map[string]bool)
	for _, entry := range entries {
		fileIDs[entry.ID] = true
		if !entry.Enabled {
			s.removeJob(entry.ID)
			continue
		}
		// 任务已注册但 cron 表达式被编辑过：先移除旧注册再按新表达式重建，
		// 否则 addJob 对已存在 ID 直接跳过，修改后的触发时刻永不生效
		s.mu.RLock()
		registeredExpr, exists := s.cronExprs[entry.ID]
		s.mu.RUnlock()
		if exists && registeredExpr != entry.CronExpr {
			s.logger.Info("schedule job cron changed, re-registering", "id", entry.ID, "old", registeredExpr, "new", entry.CronExpr)
			s.removeJob(entry.ID)
		}
		if err := s.addJob(&entry); err != nil {
			s.logger.Warn("failed to add schedule job", "id", entry.ID, "error", err)
		}
	}

	s.mu.Lock()
	for id := range s.entries {
		if !fileIDs[id] {
			s.removeJob(id)
		}
	}
	s.mu.Unlock()

	s.mu.RLock()
	changed := len(s.entries) != prevCount
	if !changed {
		for id, expr := range s.cronExprs {
			prev, ok := prevExprs[id]
			if !ok || prev != expr {
				changed = true
				break
			}
		}
	}
	cb := s.entriesCb
	s.mu.RUnlock()
	if changed && cb != nil {
		cb()
	}
	return nil
}

func (s *Scheduler) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.reloadAll(); err != nil {
				s.logger.Warn("scheduler reload failed", "error", err)
			}
		case <-ctx.Done():
			s.logger.Info("scheduler context cancelled, stopping watch loop")
			return
		}
	}
}

func (s *Scheduler) addJob(entry *ScheduleEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[entry.ID]; exists {
		return nil
	}

	e := *entry
	id, err := s.cron.AddFunc(e.CronExpr, func() {
		s.executeJob(&e)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.entries[entry.ID] = id
	s.cronExprs[entry.ID] = e.CronExpr
	s.logger.Info("added schedule job", "id", entry.ID, "agent", entry.Agent, "cron", entry.CronExpr)
	return nil
}

func (s *Scheduler) removeJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.entries[id]
	if !exists {
		return
	}

	s.cron.Remove(entryID)
	delete(s.entries, id)
	delete(s.cronExprs, id)
	s.logger.Info("removed schedule job", "id", id)
}

// executeJob 在到点时刻被调用。它不做任何具体执行，只生成 run_id 并通过
// started 生命周期事件交给上层（Daemon）拉起 Runtime 在后台执行——
// 执行主体在 daemon 侧，不依赖客户端在线。
func (s *Scheduler) executeJob(entry *ScheduleEntry) {
	runID := uuid.New().String()[:8]
	s.logger.Info("schedule job fired", "id", entry.ID, "agent", entry.Agent, "run_id", runID)

	if s.lifecycleCb != nil {
		s.lifecycleCb(JobLifecycleInfo{
			EntryID:    entry.ID,
			RunID:      runID,
			Agent:      entry.Agent,
			SessionID:  entry.SessionID,
			Content:    entry.Content,
			ProjectDir: entry.ProjectDir,
			Status:     "started",
		})
	}

	// 一次性任务：到点执行后自动禁用，避免 cron 到点再次触发。
	if entry.OneShot {
		if err := s.store.Disable(entry.ID); err != nil {
			s.logger.Warn("failed to disable one-shot job", "id", entry.ID, "error", err)
		}
	}
}

// ReportResult 记录一次调度执行的最终结果，由执行链路在对应对话结束后调用。
// 内部更新存储并触发 completed / failed 生命周期事件（供上层广播）。
func (s *Scheduler) ReportResult(entryID string, runID string, err error) {
	info := JobLifecycleInfo{EntryID: entryID, RunID: runID, Status: "completed"}
	if entry, loadErr := s.store.Load(context.Background(), entryID); loadErr == nil {
		info.Agent = entry.Agent
		info.SessionID = entry.SessionID
		info.Content = entry.Content
		info.ProjectDir = entry.ProjectDir
	}
	if err != nil {
		info.Status = "failed"
		info.Error = err.Error()
	}

	if storeErr := s.store.UpdateLastRun(entryID, runID, err); storeErr != nil {
		s.logger.Warn("failed to update last run", "id", entryID, "error", storeErr)
	}

	if s.lifecycleCb != nil {
		s.lifecycleCb(info)
	}
}

func (s *Scheduler) List() ([]ScheduleEntry, error) {
	return s.store.List(context.Background())
}
