package core

// DefaultInputCost 是未知模型默认的每百万输入 token 费用（¥）。
const DefaultInputCost = 3.0

// DefaultOutputCost 是未知模型默认的每百万输出 token 费用（¥）。
const DefaultOutputCost = 15.0

// CalculateCost 根据输入/输出 token 数和价格计算总费用。
// costPer1MIn 为每百万输入 token 的费用，costPer1MOut 为每百万输出 token 的费用。
func CalculateCost(costPer1MIn, costPer1MOut float64, promptTokens, completionTokens, cachedPromptTokens int64) float64 {
	cost := 0.0

	// Input tokens: cached portion is excluded (already paid in a prior call)
	chargeableInput := promptTokens - cachedPromptTokens
	if chargeableInput < 0 {
		chargeableInput = 0
	}
	if costPer1MIn > 0 {
		cost += costPer1MIn / 1_000_000 * float64(chargeableInput)
	}

	// Output tokens
	if costPer1MOut > 0 {
		cost += costPer1MOut / 1_000_000 * float64(completionTokens)
	}

	return cost
}
