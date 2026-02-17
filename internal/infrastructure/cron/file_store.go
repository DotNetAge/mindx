package cron

import (
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/usecase/cron"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type FileJobStore struct {
	filePath string
	mu       sync.RWMutex
	data     *JobStoreData
}

type JobStoreData struct {
	Jobs map[string]*cron.Job `json:"jobs"`
}

func NewFileJobStore() (*FileJobStore, error) {
	workspace, err := config.GetWorkspacePath()
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(workspace, "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, err
		}
	}

	filePath := filepath.Join(dataDir, "cron_jobs.json")

	store := &FileJobStore{
		filePath: filePath,
		data: &JobStoreData{
			Jobs: make(map[string]*cron.Job),
		},
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (s *FileJobStore) Add(job *cron.Job) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job.ID = uuid.New().String()
	job.CreatedAt = time.Now()
	job.Enabled = true
	job.LastStatus = cron.JobStatusPending

	s.data.Jobs[job.ID] = job
	return job.ID, s.save()
}

func (s *FileJobStore) Get(id string) (*cron.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.data.Jobs[id]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return job, nil
}

func (s *FileJobStore) List() ([]*cron.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*cron.Job, 0, len(s.data.Jobs))
	for _, job := range s.data.Jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *FileJobStore) Update(id string, job *cron.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.data.Jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	if job.Name != "" {
		existing.Name = job.Name
	}
	if job.Cron != "" {
		existing.Cron = job.Cron
	}
	if job.Skill != "" {
		existing.Skill = job.Skill
	}
	if job.Params != nil {
		existing.Params = job.Params
	}

	return s.save()
}

func (s *FileJobStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data.Jobs[id]; !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	delete(s.data.Jobs, id)
	return s.save()
}

func (s *FileJobStore) Pause(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.data.Jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Enabled = false
	return s.save()
}

func (s *FileJobStore) Resume(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.data.Jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Enabled = true
	return s.save()
}

func (s *FileJobStore) UpdateLastRun(id string, status cron.JobStatus, errMsg *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.data.Jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	now := time.Now()
	job.LastRun = &now
	job.LastStatus = status
	job.LastError = errMsg

	return s.save()
}

func (s *FileJobStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, s.data)
}

func (s *FileJobStore) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}
