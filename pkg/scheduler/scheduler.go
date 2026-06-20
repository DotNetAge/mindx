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

type CommandExecutor func(ctx context.Context, agent string, sessionID string, content string, projectDir string) error

// JobLifecycleInfo describes a scheduled job lifecycle event broadcast to clients.
type JobLifecycleInfo struct {
	EntryID   string `json:"entry_id"`
	RunID     string `json:"run_id"`
	Agent     string `json:"agent"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // "started", "completed", "failed"
	Error     string `json:"error,omitempty"`
}

// LifecycleCallback is called when a scheduled job starts, completes, or fails.
type LifecycleCallback func(info JobLifecycleInfo)

type Scheduler struct {
	cron        *cron.Cron
	store       *FileSchedulerStore
	executor    CommandExecutor
	entries     map[string]cron.EntryID
	mu          sync.RWMutex
	logger      logging.Logger
	lifecycleCb LifecycleCallback
}

func NewScheduler(store *FileSchedulerStore, executor CommandExecutor, logger logging.Logger) *Scheduler {
	if logger == nil {
		logger = logging.DefaultConsoleLogger()
	}
	c := cron.New(
		cron.WithSeconds(),
		cron.WithLogger(cron.VerbosePrintfLogger(log.New(log.Writer(), "[scheduler] ", log.LstdFlags))),
	)
	return &Scheduler{
		cron:     c,
		store:    store,
		executor: executor,
		entries:  make(map[string]cron.EntryID),
		logger:   logger,
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

func (s *Scheduler) executeJob(entry *ScheduleEntry) {
	runID := uuid.New().String()[:8]
	s.logger.Info("executing schedule job", "id", entry.ID, "agent", entry.Agent, "run_id", runID)

	if s.lifecycleCb != nil {
		s.lifecycleCb(JobLifecycleInfo{
			EntryID: entry.ID, RunID: runID, Agent: entry.Agent,
			SessionID: entry.SessionID, Status: "started",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	execErr := s.executor(ctx, entry.Agent, entry.SessionID, entry.Content, entry.ProjectDir)
	if storeErr := s.store.UpdateLastRun(entry.ID, runID, execErr); storeErr != nil {
		s.logger.Warn("failed to update last run", "id", entry.ID, "error", storeErr)
	}

	if execErr != nil {
		s.logger.Error("schedule job failed", execErr, "id", entry.ID, "run_id", runID)
		if s.lifecycleCb != nil {
			s.lifecycleCb(JobLifecycleInfo{
				EntryID: entry.ID, RunID: runID, Agent: entry.Agent,
				Status: "failed", Error: execErr.Error(),
			})
		}
	} else {
		s.logger.Info("schedule job completed", "id", entry.ID, "run_id", runID)
		if s.lifecycleCb != nil {
			s.lifecycleCb(JobLifecycleInfo{
				EntryID: entry.ID, RunID: runID, Agent: entry.Agent,
				Status: "completed",
			})
		}
	}
}

func (s *Scheduler) List() ([]ScheduleEntry, error) {
	return s.store.List(context.Background())
}
