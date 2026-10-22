package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ScheduleEntry struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	SessionID  string    `json:"session_id,omitempty"`
	ProjectDir string    `json:"project_dir,omitempty"`
	Content    string    `json:"content"`
	CronExpr   string    `json:"cron_expr"`
	Enabled    bool      `json:"enabled"`
	OneShot    bool      `json:"one_shot,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	LastRunID  string    `json:"last_run_id,omitempty"`
	LastStatus string    `json:"last_status,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	SuccessCnt int       `json:"success_count"`
	FailureCnt int       `json:"failure_count"`
	// RunHistory 是近几次执行的留痕：At 记录真实触发时刻，供前端把日历上的
	// 「历史执行」与「理论排期」区分开（月视图过去日期只渲染真执行）。
	RunHistory []RunRecord `json:"run_history,omitempty"`
}

// RunRecord 是一次运行的留痕。At 为到点触发时刻（MarkStarted 写入），
// Status 终态为 success / failed / missed（排队或取消），由终态回写时按
// RunID 更新；一次运行只有一条记录，先 started 后终态。
type RunRecord struct {
	RunID  string    `json:"run_id"`
	At     time.Time `json:"at"`
	Status string    `json:"status"`
	Error  string    `json:"error,omitempty"`
}

// runHistoryCap 限制每条任务保留的留痕条数：足够月视图逐日标注真实执行，
// 同时防止长期高频任务的 JSON 无限膨胀；超出时丢弃最旧的。
const runHistoryCap = 20

// appendRunRecord 追加一条留痕并按 cap 截断。
func appendRunRecord(entry *ScheduleEntry, rec RunRecord) {
	entry.RunHistory = append(entry.RunHistory, rec)
	if len(entry.RunHistory) > runHistoryCap {
		entry.RunHistory = entry.RunHistory[len(entry.RunHistory)-runHistoryCap:]
	}
}

// settleRunRecord 把指定 run 的留痕从 started 结算为终态；找不到匹配的
// started 记录时不写入（防御：避免脏数据混入历史）。
func settleRunRecord(entry *ScheduleEntry, runID, status, errMsg string) {
	for i := len(entry.RunHistory) - 1; i >= 0; i-- {
		if entry.RunHistory[i].RunID == runID && entry.RunHistory[i].Status == "started" {
			entry.RunHistory[i].Status = status
			entry.RunHistory[i].Error = errMsg
			return
		}
	}
}

type FileSchedulerStore struct {
	dataDir string
	mu      sync.RWMutex
}

func NewFileSchedulerStore(dataDir string) (*FileSchedulerStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create scheduler data dir: %w", err)
	}
	return &FileSchedulerStore{dataDir: dataDir}, nil
}

func (s *FileSchedulerStore) filePath(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

func (s *FileSchedulerStore) Save(ctx context.Context, entry *ScheduleEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("schedule entry ID is required")
	}

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedule entry: %w", err)
	}

	path := s.filePath(entry.ID)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write schedule entry: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename schedule entry: %w", err)
	}
	return nil
}

func (s *FileSchedulerStore) Load(ctx context.Context, id string) (*ScheduleEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schedule entry %q not found: %w", id, err)
		}
		return nil, fmt.Errorf("failed to read schedule entry: %w", err)
	}

	entry, err := unmarshalEntry(data)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *FileSchedulerStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete schedule entry: %w", err)
	}
	return nil
}

func (s *FileSchedulerStore) List(ctx context.Context) ([]ScheduleEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := filepath.Glob(filepath.Join(s.dataDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list schedule entries: %w", err)
	}

	var result []ScheduleEntry
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entry, err := unmarshalEntry(data)
		if err != nil {
			continue
		}
		result = append(result, *entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (s *FileSchedulerStore) UpdateLastRun(id string, runID string, err error) error {
	return s.updateRunStatus(id, func(entry *ScheduleEntry) bool {
		entry.LastRunAt = time.Now()
		entry.LastRunID = runID
		if err != nil {
			entry.LastStatus = "failed"
			entry.LastError = err.Error()
			entry.FailureCnt++
			settleRunRecord(entry, runID, "failed", err.Error())
		} else {
			entry.LastStatus = "success"
			entry.LastError = ""
			entry.SuccessCnt++
			settleRunRecord(entry, runID, "success", "")
		}
		return true
	})
}

// MarkStarted 记录任务已触发：写入本次运行时间与 run_id，状态置为 started，
// 并追加一条执行留痕（At 为真实触发时刻，终态回写时按 runID 结算）。
func (s *FileSchedulerStore) MarkStarted(id string, runID string) error {
	return s.updateRunStatus(id, func(entry *ScheduleEntry) bool {
		now := time.Now()
		entry.LastRunAt = now
		entry.LastRunID = runID
		entry.LastStatus = "started"
		entry.LastError = ""
		appendRunRecord(entry, RunRecord{RunID: runID, At: now, Status: "started"})
		return true
	})
}

// MarkMissed 标记任务因客户端离线等原因未能执行。
func (s *FileSchedulerStore) MarkMissed(id string) error {
	return s.updateRunStatus(id, func(entry *ScheduleEntry) bool {
		entry.LastRunAt = time.Now()
		entry.LastStatus = "missed"
		entry.LastError = "客户端离线，任务被跳过"
		entry.FailureCnt++
		settleRunRecord(entry, entry.LastRunID, "missed", "客户端离线，任务被跳过")
		return true
	})
}

// CancelRun 标记一次已触发但被用户取消的运行（前端通知框「取消」操作）。
// 仅当该运行仍处于 started 状态时生效，避免误标已完成/已失败的运行。
func (s *FileSchedulerStore) CancelRun(id string, runID string) error {
	return s.updateRunStatus(id, func(entry *ScheduleEntry) bool {
		if entry.LastRunID != runID || entry.LastStatus != "started" {
			return false
		}
		entry.LastStatus = "missed"
		entry.LastError = "用户取消"
		entry.FailureCnt++
		settleRunRecord(entry, runID, "missed", "用户取消")
		return true
	})
}

// Disable 禁用调度条目。一次性任务（OneShot）到点执行后调用，
// 避免 cron 到点后再次触发；已禁用时幂等返回。
func (s *FileSchedulerStore) Disable(id string) error {
	return s.updateRunStatus(id, func(entry *ScheduleEntry) bool {
		if !entry.Enabled {
			return false
		}
		entry.Enabled = false
		return true
	})
}

// updateRunStatus 读取调度条目，应用 mutate 变更后原子写回。
// mutate 返回 false 表示无需写入（如 CancelRun 条件不满足）。
func (s *FileSchedulerStore) updateRunStatus(id string, mutate func(*ScheduleEntry) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("failed to read entry for status update: %w", readErr)
	}

	var entry ScheduleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	if !mutate(&entry) {
		return nil
	}
	entry.UpdatedAt = time.Now()

	updated, marshalErr := json.MarshalIndent(&entry, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal updated entry: %w", marshalErr)
	}

	tmpPath := path + ".tmp"
	if writeErr := os.WriteFile(tmpPath, updated, 0600); writeErr != nil {
		return fmt.Errorf("failed to write updated entry: %w", writeErr)
	}
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename updated entry: %w", renameErr)
	}
	return nil
}

type legacyEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr"`
	Command  string `json:"command"`
	Args     string `json:"args,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Enabled  bool   `json:"enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	LastRunID  string    `json:"last_run_id,omitempty"`
	LastStatus string    `json:"last_status,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	SuccessCnt int       `json:"success_count"`
	FailureCnt int       `json:"failure_count"`
}

func unmarshalEntry(data []byte) (*ScheduleEntry, error) {
	var entry ScheduleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schedule entry: %w", err)
	}

	if entry.Content == "" {
		var legacy legacyEntry
		if err := json.Unmarshal(data, &legacy); err == nil && legacy.Command != "" {
			entry.Content = legacy.Command
			if entry.Agent == "" && legacy.Agent != "" {
				entry.Agent = legacy.Agent
			}
			if !entry.Enabled && legacy.Enabled {
				entry.Enabled = legacy.Enabled
			}
		}
	}

	return &entry, nil
}
