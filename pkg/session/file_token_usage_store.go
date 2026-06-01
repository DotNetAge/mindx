package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	goreactsession "github.com/DotNetAge/goreact/session"
	"gopkg.in/yaml.v3"
)

// yamlTokenUsageRecord is the YAML-serializable form of TokenUsageRecord.
type yamlTokenUsageRecord struct {
	ID               string    `yaml:"id"`
	SessionID        string    `yaml:"session_id"`
	ConversationID   string    `yaml:"conversation_id"`
	ModelName        string    `yaml:"model_name"`
	ProviderName     string    `yaml:"provider_name"`
	AgentName        string    `yaml:"agent_name"`
	PromptTokens     int       `yaml:"prompt_tokens"`
	CompletionTokens int       `yaml:"completion_tokens"`
	CachedTokens     int       `yaml:"cached_tokens"`
	ReasoningTokens  int       `yaml:"reasoning_tokens"`
	TotalTokens      int       `yaml:"total_tokens"`
	Timestamp        time.Time `yaml:"timestamp"`
}

func toYamlRecord(r goreactsession.TokenUsageRecord) yamlTokenUsageRecord {
	return yamlTokenUsageRecord{
		ID:               r.ID,
		SessionID:        r.SessionID,
		ConversationID:   r.ConversationID,
		ModelName:        r.ModelName,
		ProviderName:     r.ProviderName,
		AgentName:        r.AgentName,
		PromptTokens:     r.PromptTokens,
		CompletionTokens: r.CompletionTokens,
		CachedTokens:     r.CachedTokens,
		ReasoningTokens:  r.ReasoningTokens,
		TotalTokens:      r.TotalTokens,
		Timestamp:        r.Timestamp,
	}
}

func fromYamlRecord(yr yamlTokenUsageRecord) goreactsession.TokenUsageRecord {
	return goreactsession.TokenUsageRecord{
		ID:               yr.ID,
		SessionID:        yr.SessionID,
		ConversationID:   yr.ConversationID,
		ModelName:        yr.ModelName,
		ProviderName:     yr.ProviderName,
		AgentName:        yr.AgentName,
		PromptTokens:     yr.PromptTokens,
		CompletionTokens: yr.CompletionTokens,
		CachedTokens:     yr.CachedTokens,
		ReasoningTokens:  yr.ReasoningTokens,
		TotalTokens:      yr.TotalTokens,
		Timestamp:        yr.Timestamp,
	}
}

// FileTokenUsageStore implements goreact/session.TokenUsageStore with unified
// file-backed persistence. ALL records from every agent, session, and model
// are stored in a single YAML file, differentiated by the Provider / Model /
// Agent / Session dimension fields on TokenUsageRecord.
//
// File layout:
//
//	<dataDir>/token_usages.yml
//
// Thread-safe for concurrent read/write access.
type FileTokenUsageStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewFileTokenUsageStore creates a FileTokenUsageStore rooted at the given
// data directory. All token usage records are stored in a single file at
// <dataDir>/token_usages.yml.
func NewFileTokenUsageStore(dataDir string) *FileTokenUsageStore {
	return &FileTokenUsageStore{dataDir: dataDir}
}

// filePath returns the single unified path for all token usage records.
func (s *FileTokenUsageStore) filePath() string {
	return filepath.Join(s.dataDir, "token_usages.yml")
}

// loadAll reads all records from the single unified file.
// Returns empty slice if file doesn't exist.
func (s *FileTokenUsageStore) loadAll() ([]yamlTokenUsageRecord, error) {
	path := s.filePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []yamlTokenUsageRecord
	if err := yaml.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return records, nil
}

// saveAll writes all records to the single unified file atomically.
func (s *FileTokenUsageStore) saveAll(records []yamlTokenUsageRecord) error {
	path := s.filePath()
	data, err := yaml.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal token_usages: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Append writes a single TokenUsageRecord to the unified store.
func (s *FileTokenUsageStore) Append(_ context.Context, record goreactsession.TokenUsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.loadAll()
	if err != nil {
		return err
	}
	records = append(records, toYamlRecord(record))
	return s.saveAll(records)
}

// Query retrieves TokenUsageRecords matching the given filter from the unified store.
// All filtering is done in-memory on the flat record collection.
func (s *FileTokenUsageStore) Query(_ context.Context, filter goreactsession.TokenUsageFilter) ([]goreactsession.TokenUsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	yamlRecs, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	if len(yamlRecs) == 0 {
		return nil, nil
	}

	var result []goreactsession.TokenUsageRecord
	for _, yr := range yamlRecs {
		r := fromYamlRecord(yr)
		if filter.SessionID != "" && r.SessionID != filter.SessionID {
			continue
		}
		if filter.ConversationID != "" && r.ConversationID != filter.ConversationID {
			continue
		}
		if filter.ModelName != "" && r.ModelName != filter.ModelName {
			continue
		}
		if filter.ProviderName != "" && r.ProviderName != filter.ProviderName {
			continue
		}
		if filter.AgentName != "" && r.AgentName != filter.AgentName {
			continue
		}
		if !filter.Since.IsZero() && r.Timestamp.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && r.Timestamp.After(filter.Until) {
			continue
		}
		result = append(result, r)
	}

	out := make([]goreactsession.TokenUsageRecord, len(result))
	copy(out, result)
	return out, nil
}

// Close is a no-op for the file store.
func (s *FileTokenUsageStore) Close() error {
	return nil
}
