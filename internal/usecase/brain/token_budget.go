package brain

import (
	"sync"

	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// TokenBudgetManager Token 预算管理器
// 职责: 动态管理 Token 预算，基于实际消耗调整历史对话轮数
type TokenBudgetManager struct {
	modelMaxTokens       int // 模型最大 Token 容量
	reservedOutputTokens int // 预留给输出的 Token 数
	minHistoryRounds     int // 最小历史对话轮数
	avgTokensPerRound    int // 单轮平均 Token 数（初始化估算值）
	systemPromptTokens   int // 系统提示词占用的 Token 数

	// 运行时统计
	totalInputTokens  int64        // 累计输入 Token（历史对话 + 系统提示）
	totalOutputTokens int64        // 累计输出 Token
	totalRounds       int64        // 累计对话轮数
	mu                sync.RWMutex // 保护统计数据的锁
	logger            logging.Logger
}

// NewTokenBudgetManager 创建 Token 预算管理器
func NewTokenBudgetManager(
	modelMaxTokens int,
	reservedOutputTokens int,
	minHistoryRounds int,
	avgTokensPerRound int,
	logger logging.Logger,
) *TokenBudgetManager {
	return &TokenBudgetManager{
		modelMaxTokens:       modelMaxTokens,
		reservedOutputTokens: reservedOutputTokens,
		minHistoryRounds:     minHistoryRounds,
		avgTokensPerRound:    avgTokensPerRound,
		logger:               logger.Named("token_budget"),
	}
}

// SetSystemPromptTokens 设置系统提示词占用的 Token 数
func (m *TokenBudgetManager) SetSystemPromptTokens(tokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemPromptTokens = tokens
}

// RecordUsage 记录 Token 使用情况
// inputTokens: 输入 Token 数（包含历史对话 + 当前问题）
// outputTokens: 输出 Token 数（生成的回答）
func (m *TokenBudgetManager) RecordUsage(inputTokens, outputTokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalInputTokens += int64(inputTokens)
	m.totalOutputTokens += int64(outputTokens)
	m.totalRounds++

	m.logger.Debug(i18n.T("brain.record_token"),
		logging.Int("input_tokens", inputTokens),
		logging.Int("output_tokens", outputTokens),
		logging.Int64("total_input_tokens", m.totalInputTokens),
		logging.Int64("total_output_tokens", m.totalOutputTokens),
		logging.Int64("total_rounds", m.totalRounds))

	// 每 10 轮更新一次平均 Token 数
	if m.totalRounds%10 == 0 {
		m.updateAvgTokens()
	}
}

// updateAvgTokens 更新平均 Token 数
func (m *TokenBudgetManager) updateAvgTokens() {
	if m.totalRounds == 0 {
		return
	}

	// 计算每轮实际消耗的平均 Token
	newAvg := int(m.totalInputTokens / m.totalRounds)

	// 平滑更新（避免剧烈波动）
	// 新平均值 = 旧平均值 * 0.8 + 新计算值 * 0.2
	m.avgTokensPerRound = (m.avgTokensPerRound*8 + newAvg*2) / 10

	m.logger.Info(i18n.T("brain.update_avg_token"),
		logging.Int("old_avg", m.avgTokensPerRound),
		logging.Int("new_calc_avg", newAvg),
		logging.Int("smoothed_avg", m.avgTokensPerRound),
		logging.Int64("total_rounds", m.totalRounds))
}

// CalculateDynamicMaxHistoryCount 动态计算最大历史对话轮数
// 使用实际运行时的平均 Token 消耗来计算，比静态估算更准确
func (m *TokenBudgetManager) CalculateDynamicMaxHistoryCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 防止除零错误
	if m.avgTokensPerRound <= 0 {
		return m.minHistoryRounds
	}

	// 如果还没有统计数据，使用初始估算值
	if m.totalRounds == 0 {
		maxRounds := (m.modelMaxTokens - m.reservedOutputTokens - m.systemPromptTokens) / m.avgTokensPerRound
		if maxRounds < m.minHistoryRounds {
			return m.minHistoryRounds
		}
		return maxRounds
	}

	// 计算可用 Token 预算
	availableTokens := m.modelMaxTokens - m.reservedOutputTokens - m.systemPromptTokens
	if availableTokens <= 0 {
		return m.minHistoryRounds
	}

	// 使用实际运行时的平均 Token 消耗计算
	maxRounds := availableTokens / m.avgTokensPerRound

	// 确保不低于最小轮数
	if maxRounds < m.minHistoryRounds {
		return m.minHistoryRounds
	}

	return maxRounds
}

// GetStatistics 获取统计信息
func (m *TokenBudgetManager) GetStatistics() *TokenBudgetStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgInput := int64(0)
	if m.totalRounds > 0 {
		avgInput = m.totalInputTokens / m.totalRounds
	}

	return &TokenBudgetStatistics{
		TotalInputTokens:       m.totalInputTokens,
		TotalOutputTokens:      m.totalOutputTokens,
		TotalRounds:            m.totalRounds,
		AvgInputTokensPerRound: avgInput,
		MaxHistoryRounds:       m.CalculateDynamicMaxHistoryCount(),
		AvgTokensPerRound:      m.avgTokensPerRound,
	}
}

// GetAvgTokensPerRound 获取当前平均每轮 Token 消耗
func (m *TokenBudgetManager) GetAvgTokensPerRound() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.avgTokensPerRound
}

// GetTotalRounds 获取累计对话轮数
func (m *TokenBudgetManager) GetTotalRounds() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalRounds
}

// Reset 重置统计数据（用于测试或重新训练）
func (m *TokenBudgetManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalInputTokens = 0
	m.totalOutputTokens = 0
	m.totalRounds = 0

	m.logger.Info(i18n.T("brain.reset_token_stats"))
}

// TokenBudgetStatistics Token 预算统计信息
type TokenBudgetStatistics struct {
	TotalInputTokens       int64 `json:"total_input_tokens"`         // 累计输入 Token
	TotalOutputTokens      int64 `json:"total_output_tokens"`        // 累计输出 Token
	TotalRounds            int64 `json:"total_rounds"`               // 累计对话轮数
	AvgInputTokensPerRound int64 `json:"avg_input_tokens_per_round"` // 平均每轮输入 Token
	MaxHistoryRounds       int   `json:"max_history_rounds"`         // 当前最大历史轮数
	AvgTokensPerRound      int   `json:"avg_tokens_per_round"`       // 平均每轮 Token 消耗
}

// EstimateSavings 估算节省的 Token 数
// 比较：静态估算 vs 实际运行
func (m *TokenBudgetManager) EstimateSavings(initialAvgTokens int) *TokenSavings {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalRounds == 0 {
		return &TokenSavings{
			HasData: false,
		}
	}

	currentAvg := m.avgTokensPerRound
	staticMaxRounds := (m.modelMaxTokens - m.reservedOutputTokens) / initialAvgTokens
	dynamicMaxRounds := (m.modelMaxTokens - m.reservedOutputTokens) / currentAvg
	additionalRounds := dynamicMaxRounds - staticMaxRounds

	return &TokenSavings{
		HasData:          true,
		InitialAvgTokens: initialAvgTokens,
		CurrentAvgTokens: currentAvg,
		StaticMaxRounds:  staticMaxRounds,
		DynamicMaxRounds: dynamicMaxRounds,
		AdditionalRounds: additionalRounds,
		ImprovementRatio: float64(additionalRounds) / float64(staticMaxRounds),
	}
}

// TokenSavings Token 节省统计
type TokenSavings struct {
	HasData          bool    `json:"has_data"`           // 是否有足够数据
	InitialAvgTokens int     `json:"initial_avg_tokens"` // 初始估算平均 Token
	CurrentAvgTokens int     `json:"current_avg_tokens"` // 当前实际平均 Token
	StaticMaxRounds  int     `json:"static_max_rounds"`  // 静态估算最大轮数
	DynamicMaxRounds int     `json:"dynamic_max_rounds"` // 动态计算最大轮数
	AdditionalRounds int     `json:"additional_rounds"`  // 额外增加的轮数
	ImprovementRatio float64 `json:"improvement_ratio"`  // 改善比例
}
