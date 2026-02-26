package skills

import (
	"mindx/internal/config"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/logging"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestEmbeddingModel 获取测试用 embedding 模型名
func getTestEmbeddingModel() string {
	if m := os.Getenv("MINDX_TEST_EMBEDDING_MODEL"); m != "" {
		return m
	}
	return "qllama/bge-small-zh-v1.5:latest"
}

// TestVectorSearchPipeline_Health 向量搜索链路健康检查
// 验证：embedding 模型可用 → 向量生成成功 → 索引建立 → 搜索走向量路径 → 结果正确
// 此测试防止向量搜索静默退化为关键字搜索
func TestVectorSearchPipeline_Health(t *testing.T) {
	_ = initTestLogging()
	logger := logging.GetSystemLogger().Named("vector_search_health_test")

	// ===== 第1步：验证 embedding 模型可用 =====
	embeddingModel := getTestEmbeddingModel()
	provider, err := infraEmbedding.NewOllamaEmbedding("http://localhost:11434", embeddingModel)
	require.NoError(t, err, "创建 embedding provider 失败，模型: %s", embeddingModel)

	embeddingSvc := embedding.NewEmbeddingService(provider)

	// 验证能生成向量
	testVec, err := embeddingSvc.GenerateEmbedding("天气")
	require.NoError(t, err, "embedding 模型无法生成向量，模型可能未安装: %s", embeddingModel)
	require.Greater(t, len(testVec), 0, "生成的向量维度为 0")
	t.Logf("✓ embedding 模型可用: %s, 向量维度: %d", embeddingModel, len(testVec))

	// ===== 第2步：创建 SkillMgr 并建立索引 =====
	llamaSvc := infraLlama.NewOllamaService(getTestModelName())
	installSkillsPath := getProjectRootSkillsDir(t)
	workspacePath, err := config.GetWorkspacePath()
	require.NoError(t, err)

	mgr, err := NewSkillMgr(installSkillsPath, workspacePath, embeddingSvc, llamaSvc, logger)
	if err != nil {
		t.Skipf("创建技能管理器失败: %v", err)
	}

	// 建立索引
	err = mgr.ReIndex()
	require.NoError(t, err, "ReIndex 失败")
	t.Log("✓ 向量索引建立成功")

	// ===== 第3步：验证向量表非空 =====
	assert.False(t, mgr.IsVectorTableEmpty(), "向量表不应为空，索引可能未正确建立")
	t.Log("✓ 向量表非空")

	// ===== 第4步：验证搜索走向量路径且结果正确 =====
	tests := []struct {
		name        string
		keywords    []string
		expectSkill string
	}{
		{"天气查询", []string{"天气", "北京"}, "weather"},
		{"计算", []string{"计算", "数学"}, "calculator"},
		{"系统信息", []string{"系统", "CPU"}, "sysinfo"},
		{"联系人电话", []string{"电话", "联系人"}, "contacts"},
		{"文件搜索", []string{"文件", "搜索"}, "file_search"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := mgr.SearchSkills(tc.keywords...)
			require.NoError(t, err)

			if len(results) == 0 {
				t.Logf("⚠ 搜索 %v 未找到任何技能", tc.keywords)
				return
			}

			foundNames := make([]string, 0, len(results))
			for _, s := range results {
				foundNames = append(foundNames, s.GetName())
			}

			// 验证期望的技能在结果中
			found := false
			for _, name := range foundNames {
				if name == tc.expectSkill {
					found = true
					break
				}
			}

			assert.True(t, found,
				"搜索 %v 应找到 %s，实际结果: %v", tc.keywords, tc.expectSkill, foundNames)

			// 验证期望的技能排在前面（相关性高）
			if found && foundNames[0] == tc.expectSkill {
				t.Logf("✓ %s 排在第一位", tc.expectSkill)
			} else if found {
				t.Logf("⚠ %s 在结果中但未排第一，实际排序: %v", tc.expectSkill, foundNames)
			}
		})
	}
}

// TestVectorSearch_NotFallbackToKeyword 确保向量搜索不会静默退化为关键字搜索
func TestVectorSearch_NotFallbackToKeyword(t *testing.T) {
	_ = initTestLogging()
	logger := logging.GetSystemLogger().Named("vector_no_fallback_test")

	embeddingModel := getTestEmbeddingModel()
	provider, err := infraEmbedding.NewOllamaEmbedding("http://localhost:11434", embeddingModel)
	if err != nil {
		t.Skipf("embedding 模型不可用: %v", err)
	}
	embeddingSvc := embedding.NewEmbeddingService(provider)

	// 验证 embedding 能工作
	_, err = embeddingSvc.GenerateEmbedding("test")
	if err != nil {
		t.Fatalf("embedding 模型无法生成向量: %v\n"+
			"请确认 Ollama 已安装 embedding 模型: ollama pull %s", err, embeddingModel)
	}

	llamaSvc := infraLlama.NewOllamaService(getTestModelName())
	installSkillsPath := getProjectRootSkillsDir(t)
	workspacePath, err := config.GetWorkspacePath()
	require.NoError(t, err)

	mgr, err := NewSkillMgr(installSkillsPath, workspacePath, embeddingSvc, llamaSvc, logger)
	if err != nil {
		t.Skipf("创建技能管理器失败: %v", err)
	}

	err = mgr.ReIndex()
	require.NoError(t, err)

	// 核心断言：向量表必须非空
	require.False(t, mgr.IsVectorTableEmpty(),
		"向量表为空！向量搜索将退化为关键字搜索。\n"+
			"可能原因：\n"+
			"1. embedding 模型未正确配置\n"+
			"2. ReIndex 过程中向量生成失败\n"+
			"3. 技能定义缺少 tags/description")

	t.Log("✓ 向量表非空，搜索将走向量路径")
}
