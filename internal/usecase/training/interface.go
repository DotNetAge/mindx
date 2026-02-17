package training

import (
	"mindx/internal/core"
	"time"
)

type TrainingDataSource interface {
	GetAllMemoryPoints() ([]core.MemoryPoint, error)
	GetLastTrainTime() (time.Time, error)
	UpdateLastTrainTime(t time.Time) error
}

type MemoryPoint struct {
	Keywords  []string
	Content   string
	Summary   string
	CreatedAt time.Time
}

type TrainingPair struct {
	Prompt     string    `json:"prompt"`
	Completion string    `json:"completion"`
	Topic      string    `json:"topic,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func FromCoreMemoryPoint(p core.MemoryPoint) MemoryPoint {
	return MemoryPoint{
		Keywords:  p.Keywords,
		Content:   p.Content,
		Summary:   p.Summary,
		CreatedAt: p.CreatedAt,
	}
}
