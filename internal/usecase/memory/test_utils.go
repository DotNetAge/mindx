package memory

import (
	"mindx/internal/config"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/logging"
	"context"

	"go.uber.org/zap"
)

func newTestLogger() logging.Logger {
	return &zapLoggerWrapper{logger: zap.NewNop()}
}

type zapLoggerWrapper struct {
	logger *zap.Logger
}

func (l *zapLoggerWrapper) Debug(msg string, fields ...logging.Field)      {}
func (l *zapLoggerWrapper) Info(msg string, fields ...logging.Field)       {}
func (l *zapLoggerWrapper) Warn(msg string, fields ...logging.Field)       {}
func (l *zapLoggerWrapper) Error(msg string, fields ...logging.Field)      {}
func (l *zapLoggerWrapper) Fatal(msg string, fields ...logging.Field)      {}
func (l *zapLoggerWrapper) WithContext(ctx context.Context) logging.Logger { return l }
func (l *zapLoggerWrapper) With(fields ...logging.Field) logging.Logger    { return l }
func (l *zapLoggerWrapper) Named(name string) logging.Logger {
	return l
}

// NewTestMemory 创建测试用的 Memory 实例
func NewTestMemory(logger logging.Logger) *Memory {
	provider := infraEmbedding.NewTFIDFEmbedding()
	embeddingSvc := embedding.NewEmbeddingService(provider)
	store := persistence.NewMemoryStore(provider)

	return &Memory{
		logger:           logger,
		embeddingService: embeddingSvc,
		store:            store,
		llmClient:        nil,
		summaryModel:     "",
		keywordModel:     "",
		config:           &config.VectorStoreConfig{Type: "memory", DataPath: ""},
	}
}
