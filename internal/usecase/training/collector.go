package training

import (
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Collector struct {
	dataSource   TrainingDataSource
	lastTrainLog string
	logger       logging.Logger
}

func NewCollector(dataSource TrainingDataSource, logger logging.Logger) (*Collector, error) {
	if dataSource == nil {
		return nil, fmt.Errorf(i18n.T("collector.datasource_nil"))
	}

	homeDir, _ := os.UserHomeDir()
	lastTrainLog := filepath.Join(homeDir, ".bot", "training", "last_training.json")

	if err := os.MkdirAll(filepath.Dir(lastTrainLog), 0755); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("collector.create_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	return &Collector{
		dataSource:   dataSource,
		lastTrainLog: lastTrainLog,
		logger:       logger,
	}, nil
}

func (c *Collector) GetLastTrainTime() (time.Time, error) {
	if _, err := os.Stat(c.lastTrainLog); os.IsNotExist(err) {
		return time.Now().AddDate(0, 0, -7), nil
	}

	data, err := os.ReadFile(c.lastTrainLog)
	if err != nil {
		return time.Time{}, fmt.Errorf(i18n.TWithData("collector.read_log_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var trainLog struct {
		LastTrainTime time.Time `json:"last_train_time"`
	}
	if err := json.Unmarshal(data, &trainLog); err != nil {
		return time.Time{}, fmt.Errorf(i18n.TWithData("collector.parse_log_failed", map[string]interface{}{"Error": err.Error()}))
	}

	return trainLog.LastTrainTime, nil
}

func (c *Collector) UpdateLastTrainTime(t time.Time) error {
	trainLog := struct {
		LastTrainTime time.Time `json:"last_train_time"`
	}{
		LastTrainTime: t,
	}

	data, err := json.MarshalIndent(trainLog, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.TWithData("collector.marshal_log_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := os.WriteFile(c.lastTrainLog, data, 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("collector.write_log_failed", map[string]interface{}{"Error": err.Error()}))
	}

	c.logger.Info(i18n.T("collector.train_time_updated"), logging.String("time", t.Format("2006-01-02 15:04:05")))
	return nil
}

func (c *Collector) CollectMemoryPoints() ([]MemoryPoint, error) {
	lastTrainTime, err := c.GetLastTrainTime()
	if err != nil {
		return nil, err
	}

	allPoints, err := c.dataSource.GetAllMemoryPoints()
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("collector.get_points_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var recentPoints []MemoryPoint
	for _, p := range allPoints {
		if p.CreatedAt.After(lastTrainTime) {
			recentPoints = append(recentPoints, FromCoreMemoryPoint(p))
		}
	}

	c.logger.Info(i18n.T("collector.collect_complete"),
		logging.String("from", lastTrainTime.Format("2006-01-02")),
		logging.String("to", time.Now().Format("2006-01-02")),
		logging.Int("total_points", len(allPoints)),
		logging.Int("recent_points", len(recentPoints)))

	return recentPoints, nil
}

func (c *Collector) ExtractTrainingPairs(points []MemoryPoint) []TrainingPair {
	var pairs []TrainingPair

	for _, point := range points {
		if point.Content == "" {
			continue
		}

		content := point.Content
		var userMsg, assistantMsg string

		if strings.Contains(content, "用户:") || strings.Contains(content, "User:") {
			parts := c.splitConversation(content)
			if len(parts) >= 2 {
				userMsg = parts[0]
				assistantMsg = parts[1]
			}
		} else {
			if point.Summary != "" {
				userMsg = point.Summary
			} else if len(point.Keywords) > 0 {
				userMsg = strings.Join(point.Keywords, " ")
			} else {
				continue
			}
			assistantMsg = content
		}

		if userMsg != "" && assistantMsg != "" {
			pairs = append(pairs, TrainingPair{
				Prompt:     userMsg,
				Completion: assistantMsg,
				Topic:      strings.Join(point.Keywords, ", "),
				Timestamp:  point.CreatedAt,
			})
		}
	}

	c.logger.Info(i18n.T("collector.extract_pairs_complete"), logging.Int("pairs", len(pairs)))
	return pairs
}

func (c *Collector) splitConversation(content string) []string {
	var parts []string

	separators := []string{"用户:", "User:", "助手:", "Assistant:"}

	working := content
	for _, sep := range separators {
		if strings.Contains(working, sep) {
			working = strings.ReplaceAll(working, sep, "|||"+sep)
		}
	}

	segments := strings.Split(working, "|||")

	var userPart, assistantPart string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		if strings.HasPrefix(seg, "用户:") || strings.HasPrefix(seg, "User:") {
			userPart = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(seg, "用户:"), "User:"))
		} else if strings.HasPrefix(seg, "助手:") || strings.HasPrefix(seg, "Assistant:") {
			assistantPart = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(seg, "助手:"), "Assistant:"))
		}
	}

	if userPart != "" && assistantPart != "" {
		parts = []string{userPart, assistantPart}
	}

	return parts
}

func (c *Collector) GetMemoryStats(points []MemoryPoint) map[string]interface{} {
	if len(points) == 0 {
		return map[string]interface{}{
			"total": 0,
		}
	}

	var earliest, latest time.Time
	keywords := make(map[string]int)

	for i, p := range points {
		if i == 0 {
			earliest = p.CreatedAt
			latest = p.CreatedAt
		} else {
			if p.CreatedAt.Before(earliest) {
				earliest = p.CreatedAt
			}
			if p.CreatedAt.After(latest) {
				latest = p.CreatedAt
			}
		}

		for _, kw := range p.Keywords {
			keywords[kw]++
		}
	}

	return map[string]interface{}{
		"total_points":    len(points),
		"earliest":        earliest.Format("2006-01-02"),
		"latest":          latest.Format("2006-01-02"),
		"unique_keywords": len(keywords),
	}
}
