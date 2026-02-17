package skills

import (
	"mindx/internal/config"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/logging"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPrecomputeVectors 集成测试
func TestPrecomputeVectors(t *testing.T) {
	// 初始化日志
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/precompute_vectors_test.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   false,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable:     false,
			OutputPath: "/tmp/conversation.log",
		},
	}
	_ = logging.Init(logConfig)
	logger := logging.GetSystemLogger().Named("precompute_vectors_test")

	// 创建 embedding provider
	provider, err := infraEmbedding.NewOllamaEmbedding("http://localhost:11434", "nomic-embed-text")
	require.NoError(t, err)

	// 创建 embedding service
	embeddingSvc := embedding.NewEmbeddingService(provider)

	// 创建 llama service
	llamaSvc := infraLlama.NewOllamaService("qwen2.5:7b")

	// 创建 SkillMgr
	installSkillsPath, err := config.GetInstallSkillsPath()
	require.NoError(t, err)
	workspacePath, err := config.GetWorkspacePath()
	require.NoError(t, err)
	mgr, err := NewSkillMgr(installSkillsPath, workspacePath, embeddingSvc, llamaSvc, logger)
	require.NoError(t, err)

	// 执行预计算
	err = mgr.ReIndex()
	require.NoError(t, err, "ReIndex() 应该成功执行")
}
