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
// started 由到点触发（Daemon 据此决定离线跳过或广播 OnJobStart）；
// completed / failed 由执行链路通过 ReportResult 在对话结束后触发。
type LifecycleCallback func(info JobLifecycleInfo)

type Scheduler struct {
	cron        *cron.Cron
	store       *FileSchedulerStore
	entries     map[string]cron.EntryID
	mu          sync.RWMutex
	logger      logging.Logger
	lifecycleCb LifecycleCallback
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
		cron:    c,
		store:   store,
		entries: make(map[string]cron.EntryID),
		logger:  logger,
	}
}

// OnLifecycle sets a callback that fires when a scheduled job starts, completes, or fails.
func (s *Scheduler) OnLifecycle(cb LifecycleCallback) {
	s.lifecycleCb = cb
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
	s.logger.Info("removed schedule job", "id", id)
}

// executeJob 在到点时刻被调用。它不做任何具体执行，只生成 run_id 并通过
// started 生命周期事件交给上层（Daemon）决策：有在线客户端则广播 OnJobStart
// 交由前端前台执行；离线则直接标记 missed。
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
