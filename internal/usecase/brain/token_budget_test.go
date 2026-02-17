package brain

import (
	"testing"
	"time"

	"mindx/pkg/logging"
	"github.com/stretchr/testify/assert"
)

func TestTokenBudgetManager_New(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(
		40960, // maxTokens
		8192,  // reservedOutputTokens
		5,      // minHistoryRounds
		200,    // avgTokensPerRound
		logger,
	)

	assert.NotNil(t, mgr)
	assert.Equal(t, 40960, mgr.modelMaxTokens)
	assert.Equal(t, 8192, mgr.reservedOutputTokens)
	assert.Equal(t, 5, mgr.minHistoryRounds)
	assert.Equal(t, 200, mgr.avgTokensPerRound)
}

func TestTokenBudgetManager_CalculateDynamicMaxHistoryCount(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(40960, 8192, 5, 200, logger)

	// 测试1: 初始状态（无统计数据）
	maxRounds := mgr.CalculateDynamicMaxHistoryCount()
	// (40960 - 8192) / 200 = 163
	assert.Equal(t, 163, maxRounds)
}

func TestTokenBudgetManager_RecordUsage(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(40960, 8192, 5, 200, logger)

	// 模拟多轮对话的 Token 消耗
	testCases := []struct {
		inputTokens  int
		outputTokens int
	}{
		{150, 100},  // 简单对话
		{120, 80},   // 简短对话
		{200, 150},  // 中等对话
		{180, 120},  // 中等对话
		{300, 200},  // 长对话
		{250, 180},  // 长对话
		{140, 90},   // 简单对话
		{160, 110},  // 简单对话
		{190, 140},  // 中等对话
		{170, 130},  // 中等对话
	}

	for _, tc := range testCases {
		mgr.RecordUsage(tc.inputTokens, tc.outputTokens)
		time.Sleep(1 * time.Millisecond) // 避免时间戳冲突
	}

	// 验证统计数据
	stats := mgr.GetStatistics()
	assert.Equal(t, int64(10), stats.TotalRounds)
	assert.True(t, stats.TotalInputTokens > 0)
	assert.True(t, stats.TotalOutputTokens > 0)

	// 平均输入 Token 应该在合理范围内
	avgInput := stats.AvgInputTokensPerRound
	assert.True(t, avgInput >= 100 && avgInput <= 400, "平均输入 Token 应该在 100-400 之间")
}

func TestTokenBudgetManager_DynamicAdjustment(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(40960, 8192, 5, 200, logger)

	// 初始估算
	initialMaxRounds := mgr.CalculateDynamicMaxHistoryCount()
	assert.Equal(t, 163, initialMaxRounds) // (40960 - 8192) / 200

	// 模拟小量对话（节省 Token）
	// 实际平均只有 150 Token，比估算的 200 少
	for i := 0; i < 20; i++ {
		mgr.RecordUsage(120, 80) // 输入120，输出80，总共200
	}

	// 获取调整后的最大轮数
	adjustedMaxRounds := mgr.CalculateDynamicMaxHistoryCount()

	// 由于实际 Token 消耗比估算少，应该能支持更多轮数
	assert.True(t, adjustedMaxRounds >= initialMaxRounds, "调整后应该支持更多或相同的轮数")

	stats := mgr.GetStatistics()
	t.Logf("初始估算: %d 轮", initialMaxRounds)
	t.Logf("实际平均每轮: %d Token", stats.AvgInputTokensPerRound)
	t.Logf("调整后: %d 轮", adjustedMaxRounds)
	t.Logf("增加轮数: %d", adjustedMaxRounds-initialMaxRounds)
}

func TestTokenBudgetManager_EstimateSavings(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(40960, 8192, 5, 200, logger)

	// 无数据时
	savings := mgr.EstimateSavings(200)
	assert.False(t, savings.HasData)

	// 添加数据（实际平均 150 Token）
	for i := 0; i < 20; i++ {
		mgr.RecordUsage(120, 80) // 总共 200 Token
	}

	// 检查节省
	savings = mgr.EstimateSavings(200)
	assert.True(t, savings.HasData)
	assert.Equal(t, 200, savings.InitialAvgTokens)
	assert.True(t, savings.CurrentAvgTokens > 0)
	assert.True(t, savings.AdditionalRounds >= 0)

	t.Logf("静态估算: %d 轮 (%d Token/轮)", savings.StaticMaxRounds, savings.InitialAvgTokens)
	t.Logf("动态计算: %d 轮 (%d Token/轮)", savings.DynamicMaxRounds, savings.CurrentAvgTokens)
	t.Logf("额外增加: %d 轮 (%.2f%%)", savings.AdditionalRounds, savings.ImprovementRatio*100)
}

func TestTokenBudgetManager_Reset(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(40960, 8192, 5, 200, logger)

	// 记录一些使用
	mgr.RecordUsage(150, 100)
	mgr.RecordUsage(200, 150)
	mgr.RecordUsage(180, 120)

	// 验证有数据
	assert.True(t, mgr.GetTotalRounds() > 0)

	// 重置
	mgr.Reset()

	// 验证数据已清空
	assert.Equal(t, int64(0), mgr.GetTotalRounds())
	stats := mgr.GetStatistics()
	assert.Equal(t, int64(0), stats.TotalRounds)
	assert.Equal(t, int64(0), stats.TotalInputTokens)
	assert.Equal(t, int64(0), stats.TotalOutputTokens)
}

func TestTokenBudgetManager_MinHistoryRounds(t *testing.T) {
	logger := logging.GetSystemLogger()
	mgr := NewTokenBudgetManager(1000, 900, 10, 100, logger)

	// 模型容量很小，但最小轮数是 10
	// (1000 - 900) / 100 = 1，但应该返回最小值 10
	maxRounds := mgr.CalculateDynamicMaxHistoryCount()
	assert.Equal(t, 10, maxRounds)
}
