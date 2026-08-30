package core

// DefaultInputCost 是未知模型默认的每百万输入 token 费用（¥）。
const DefaultInputCost = 3.0

// DefaultOutputCost 是未知模型默认的每百万输出 token 费用（¥）。
const DefaultOutputCost = 15.0

// CalculateCost 根据输入/输出/缓存 token 数和价格计算总费用。
// costPer1MIn 为每百万输入 token 的费用，costPer1MOut 为每百万输出 token 的费用，
// costPer1MInCache 为每百万缓存命中输入 token 的费用（0 表示缓存免费）。
// 未缓存输入按输入价计费，缓存命中输入按缓存价计费，输出按输出价计费。
func CalculateCost(costPer1MIn, costPer1MOut, costPer1MInCache float64, promptTokens, completionTokens, cachedPromptTokens int64) float64 {
	cost := 0.0

	// 未命中的输入 token 按输入价计费（缓存部分改按缓存价单独计费）。
	uncachedInput := promptTokens - cachedPromptTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}
	if costPer1MIn > 0 {
		cost += costPer1MIn / 1_000_000 * float64(uncachedInput)
	}

	// 缓存命中的输入 token 按缓存价计费（缓存价 >0 时收费，为 0 时免费）。
	if cachedPromptTokens > 0 && costPer1MInCache > 0 {
		cost += costPer1MInCache / 1_000_000 * float64(cachedPromptTokens)
	}

	// Output tokens
	if costPer1MOut > 0 {
		cost += costPer1MOut / 1_000_000 * float64(completionTokens)
	}

	return cost
}
