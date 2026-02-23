package memory

import (
	"mindx/internal/config"
	"mindx/internal/core"
	apperrors "mindx/internal/errors"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type Memory struct {
	store            core.Store
	llmClient        *openai.Client
	summaryModel     string
	keywordModel     string
	config           *config.VectorStoreConfig
	logger           logging.Logger
	embeddingService *embedding.EmbeddingService
}

func NewMemory(
	cfg *config.GlobalConfig,
	llmClient *openai.Client,
	logger logging.Logger,
	store core.Store,
	embeddingService *embedding.EmbeddingService,
) (*Memory, error) {
	if cfg.VectorStore.Type == "" {
		cfg.VectorStore.Type = "badger"
	}
	if cfg.VectorStore.DataPath == "" {
		cfg.VectorStore.DataPath = filepath.Join("data", "memory")
	}

	if err := os.MkdirAll(cfg.VectorStore.DataPath, 0755); err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTypeMemory, "创建记忆存储目录失败")
	}
	memory := &Memory{
		store:            store,
		llmClient:        llmClient,
		summaryModel:     cfg.Memory.SummaryModel,
		keywordModel:     cfg.Memory.KeywordModel,
		config:           &cfg.VectorStore,
		logger:           logger,
		embeddingService: embeddingService,
	}

	logger.Info(i18n.T("memory.init_success"),
		logging.String(i18n.T("memory.type"), cfg.VectorStore.Type),
		logging.String(i18n.T("memory.path"), cfg.VectorStore.DataPath))

	return memory, nil
}

func (m *Memory) Record(point core.MemoryPoint) error {
	m.logger.Debug(i18n.T("memory.start_record"), logging.Int(i18n.T("memory.keywords_count"), len(point.Keywords)))

	if point.CreatedAt.IsZero() {
		point.CreatedAt = time.Now()
	}
	point.UpdatedAt = time.Now()

	if len(point.Vector) == 0 {
		if m.embeddingService != nil {
			combinedText := strings.Join(point.Keywords, " ") + " " + point.Summary + " " + point.Content
			vector, err := m.embeddingService.GenerateEmbedding(combinedText)
			if err != nil {
				m.logger.Warn(i18n.T("memory.gen_vector_failed"), logging.Err(err))
				vector = []float64{}
			}
			point.Vector = vector
		} else {
			point.Vector = []float64{}
		}
	}

	// 语义去重：检查是否有高度相似的已有记忆
	mergedPoint, wasMerged := m.DeduplicateMemory(&point)
	if wasMerged {
		point = *mergedPoint
	}

	if err := m.storeMemory(point); err != nil {
		m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
		return apperrors.Wrap(err, apperrors.ErrTypeMemory, "存储记忆失败")
	}

	m.logger.Info(i18n.T("memory.record_success"),
		logging.Int(i18n.T("memory.id"), point.ID),
		logging.Int(i18n.T("memory.keywords_count"), len(point.Keywords)),
		logging.Float64(i18n.T("memory.total_weight"), point.TotalWeight))

	go func() {
		if err := m.CleanupExpiredMemories(); err != nil {
			m.logger.Error(i18n.T("memory.cleanup_failed"), logging.Err(err))
		}
	}()

	return nil
}

func (m *Memory) Close() error {
	return m.store.Close()
}
