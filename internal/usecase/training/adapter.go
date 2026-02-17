package training

import (
	"mindx/internal/core"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type MemoryAdapter struct {
	memory       MemoryProvider
	lastTrainLog string
}

type MemoryProvider interface {
	GetAllMemoryPoints() ([]core.MemoryPoint, error)
}

func NewMemoryAdapter(memory MemoryProvider) (*MemoryAdapter, error) {
	homeDir, _ := os.UserHomeDir()
	lastTrainLog := filepath.Join(homeDir, ".bot", "training", "last_training.json")

	if err := os.MkdirAll(filepath.Dir(lastTrainLog), 0755); err != nil {
		return nil, err
	}

	return &MemoryAdapter{
		memory:       memory,
		lastTrainLog: lastTrainLog,
	}, nil
}

func (a *MemoryAdapter) GetAllMemoryPoints() ([]core.MemoryPoint, error) {
	return a.memory.GetAllMemoryPoints()
}

func (a *MemoryAdapter) GetLastTrainTime() (time.Time, error) {
	if _, err := os.Stat(a.lastTrainLog); os.IsNotExist(err) {
		return time.Now().AddDate(0, 0, -7), nil
	}

	data, err := os.ReadFile(a.lastTrainLog)
	if err != nil {
		return time.Time{}, err
	}

	var trainLog struct {
		LastTrainTime time.Time `json:"last_train_time"`
	}
	if err := json.Unmarshal(data, &trainLog); err != nil {
		return time.Time{}, err
	}

	return trainLog.LastTrainTime, nil
}

func (a *MemoryAdapter) UpdateLastTrainTime(t time.Time) error {
	trainLog := struct {
		LastTrainTime time.Time `json:"last_train_time"`
	}{
		LastTrainTime: t,
	}

	data, err := json.MarshalIndent(trainLog, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.lastTrainLog, data, 0644)
}
