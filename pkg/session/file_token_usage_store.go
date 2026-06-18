package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"gopkg.in/yaml.v3"
)

// TokenUsageRecordWithSource extends goharness's TokenUsageRecord with a source field
// that identifies where the token consumption originated (chat, indexing, translation, etc.).
type TokenUsageRecordWithSource struct {
	goharnesssession.TokenUsageRecord
	Source UsageSource `json:"source"`
}

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
	Source           string    `yaml:"source"`
}

func toYamlRecord(r goharnesssession.TokenUsageRecord, source UsageSource) yamlTokenUsageRecord {
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
		Source:           string(source),
	}
}

func fromYamlRecord(yr yamlTokenUsageRecord) TokenUsageRecordWithSource {
	source := UsageSource(yr.Source)
	if source == "" {
		source = UsageSourceChat
	}
	return TokenUsageRecordWithSource{
		TokenUsageRecord: goharnesssession.TokenUsageRecord{
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
		},
		Source: source,
	}
}

// FileTokenUsageStore implements goharness/session.TokenUsageStore with unified
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

// Append writes a single TokenUsageRecord with default source "chat".
func (s *FileTokenUsageStore) Append(_ context.Context, record goharnesssession.TokenUsageRecord) error {
	return s.appendWithSource(record, UsageSourceChat)
}

// AppendWithSource writes a single TokenUsageRecord with a specific source identifier
// (e.g. "indexing"). This lets consumers distinguish indexing consumption
// from chat consumption.
func (s *FileTokenUsageStore) AppendWithSource(_ context.Context, record goharnesssession.TokenUsageRecord, source string) error {
	return s.appendWithSource(record, UsageSource(source))
}

func (s *FileTokenUsageStore) appendWithSource(record goharnesssession.TokenUsageRecord, source UsageSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.loadAll()
	if err != nil {
		return err
	}
	records = append(records, toYamlRecord(record, source))
	return s.saveAll(records)
}

// Query retrieves TokenUsageRecords matching the given filter from the unified store.
// All filtering is done in-memory on the flat record collection.
func (s *FileTokenUsageStore) Query(_ context.Context, filter goharnesssession.TokenUsageFilter) ([]goharnesssession.TokenUsageRecord, error) {
	extRecords, err := s.QueryWithSource(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	result := make([]goharnesssession.TokenUsageRecord, len(extRecords))
	for i, r := range extRecords {
		result[i] = r.TokenUsageRecord
	}
	return result, nil
}

// QueryWithSource retrieves TokenUsageRecordWithSource entries matching the given filter.
// Unlike Query, this returns the source field so callers can distinguish indexing
// consumption from chat consumption.
func (s *FileTokenUsageStore) QueryWithSource(_ context.Context, filter goharnesssession.TokenUsageFilter) ([]TokenUsageRecordWithSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	yamlRecs, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	if len(yamlRecs) == 0 {
		return nil, nil
	}

	var result []TokenUsageRecordWithSource
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

	out := make([]TokenUsageRecordWithSource, len(result))
	copy(out, result)
	return out, nil
}

// Close is a no-op for the file store.
func (s *FileTokenUsageStore) Close() error {
	return nil
}
