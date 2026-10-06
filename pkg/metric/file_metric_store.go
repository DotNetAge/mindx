package metric

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// FileMetricStore 是决策计量事件的本地持久化设施，复刻
// pkg/session FileTokenUsageStore 的单文件 + 全量读写模式。
//
// 文件布局：
//
//	<dataDir>/metric_events.yml        计量事件（round / rating / overall）
//	<dataDir>/metric_confoundings.yml  混杂登记事件
//
// 明细按 Retention 滚动保留（默认 90 天），聚合结果由上层持久化、不受滚动影响。
// 线程安全（RWMutex）。
type FileMetricStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewFileMetricStore 创建以 dataDir 为根的 FileMetricStore。
func NewFileMetricStore(dataDir string) *FileMetricStore {
	return &FileMetricStore{dataDir: dataDir}
}

func (s *FileMetricStore) eventsPath() string {
	return filepath.Join(s.dataDir, "metric_events.yml")
}

func (s *FileMetricStore) confoundingsPath() string {
	return filepath.Join(s.dataDir, "metric_confoundings.yml")
}

// loadEvents 读取全部计量事件；文件不存在时返回空切片。
func (s *FileMetricStore) loadEvents() ([]MetricEvent, error) {
	data, err := os.ReadFile(s.eventsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var events []MetricEvent
	if err := yaml.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.eventsPath(), err)
	}
	return events, nil
}

// saveEvents 原子写入全部计量事件。
func (s *FileMetricStore) saveEvents(events []MetricEvent) error {
	data, err := yaml.Marshal(events)
	if err != nil {
		return fmt.Errorf("marshal metric events: %w", err)
	}
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	return os.WriteFile(s.eventsPath(), data, 0644)
}

// loadConfoundings 读取全部混杂登记；文件不存在时返回空切片。
func (s *FileMetricStore) loadConfoundings() ([]ConfoundingEvent, error) {
	data, err := os.ReadFile(s.confoundingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var events []ConfoundingEvent
	if err := yaml.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.confoundingsPath(), err)
	}
	return events, nil
}

// saveConfoundings 原子写入全部混杂登记。
func (s *FileMetricStore) saveConfoundings(events []ConfoundingEvent) error {
	data, err := yaml.Marshal(events)
	if err != nil {
		return fmt.Errorf("marshal metric confoundings: %w", err)
	}
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	return os.WriteFile(s.confoundingsPath(), data, 0644)
}

// AppendEvent 写入一条计量事件（round / rating / overall）。
// 事件 ID 为空时自动生成。
func (s *FileMetricStore) AppendEvent(_ context.Context, ev MetricEvent) (MetricEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.loadEvents()
	if err != nil {
		return MetricEvent{}, err
	}
	if ev.ID == "" {
		ev.ID = newEventID()
	}
	if ev.TS.IsZero() {
		ev.TS = time.Now()
	}
	events = append(events, ev)
	if err := s.saveEvents(events); err != nil {
		return MetricEvent{}, err
	}
	return ev, nil
}

// AppendJudgment 向指定事件追加一条判定记录（版本链追加，不改写历史）。
// 追加成功时判定时间取当前时刻。找不到目标事件时返回错误。
func (s *FileMetricStore) AppendJudgment(_ context.Context, eventID string, j JudgmentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.loadEvents()
	if err != nil {
		return err
	}
	for i := range events {
		if events[i].ID != eventID {
			continue
		}
		if j.At.IsZero() {
			j.At = time.Now()
		}
		events[i].Judgments = append(events[i].Judgments, j)
		return s.saveEvents(events)
	}
	return fmt.Errorf("metric event %q not found", eventID)
}

// Query 按 filter 过滤计量事件（内存过滤，条件零值表示不限制）。
func (s *FileMetricStore) Query(_ context.Context, filter MetricFilter) ([]MetricEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, err := s.loadEvents()
	if err != nil {
		return nil, err
	}

	var result []MetricEvent
	for _, ev := range events {
		if filter.SessionID != "" && ev.SessionID != filter.SessionID {
			continue
		}
		if filter.AgentName != "" && ev.AgentName != filter.AgentName {
			continue
		}
		if filter.Team != "" && ev.Team != filter.Team {
			continue
		}
		if filter.SkillName != "" && ev.SkillName != filter.SkillName {
			continue
		}
		if filter.Kind != "" && ev.Kind != filter.Kind {
			continue
		}
		if !filter.Since.IsZero() && ev.TS.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && ev.TS.After(filter.Until) {
			continue
		}
		result = append(result, ev)
	}
	return result, nil
}

// Retention 删除早于 olderThan 的计量事件与混杂登记，返回各自删除数。
// 由 daemon 定时调用（默认 90 天滚动——白皮书 8.3 成本边界）。
func (s *FileMetricStore) Retention(_ context.Context, olderThan time.Time) (eventsRemoved int, confoundingsRemoved int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.loadEvents()
	if err != nil {
		return 0, 0, err
	}
	kept := events[:0]
	for _, ev := range events {
		if ev.TS.Before(olderThan) {
			eventsRemoved++
			continue
		}
		kept = append(kept, ev)
	}
	if eventsRemoved > 0 {
		if err := s.saveEvents(kept); err != nil {
			return eventsRemoved, 0, err
		}
	}

	confoundings, err := s.loadConfoundings()
	if err != nil {
		return eventsRemoved, 0, err
	}
	keptC := confoundings[:0]
	for _, ev := range confoundings {
		if ev.TS.Before(olderThan) {
			confoundingsRemoved++
			continue
		}
		keptC = append(keptC, ev)
	}
	if confoundingsRemoved > 0 {
		if err := s.saveConfoundings(keptC); err != nil {
			return eventsRemoved, confoundingsRemoved, err
		}
	}
	return eventsRemoved, confoundingsRemoved, nil
}

// LogConfounding 写入一条混杂登记事件。
func (s *FileMetricStore) LogConfounding(_ context.Context, ev ConfoundingEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	confoundings, err := s.loadConfoundings()
	if err != nil {
		return err
	}
	if ev.TS.IsZero() {
		ev.TS = time.Now()
	}
	confoundings = append(confoundings, ev)
	return s.saveConfoundings(confoundings)
}

// QueryConfoundings 返回窗口 [since, until] 内的混杂登记（compare 输出附带）。
func (s *FileMetricStore) QueryConfoundings(_ context.Context, since, until time.Time) ([]ConfoundingEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	confoundings, err := s.loadConfoundings()
	if err != nil {
		return nil, err
	}
	var result []ConfoundingEvent
	for _, ev := range confoundings {
		if !since.IsZero() && ev.TS.Before(since) {
			continue
		}
		if !until.IsZero() && ev.TS.After(until) {
			continue
		}
		result = append(result, ev)
	}
	return result, nil
}

// newEventID 生成事件 ID：纳秒时间戳 + 随机后缀，无需外部依赖。
func newEventID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(buf))
}
