package data

// ModelPricing holds per-model pricing per 1M tokens.
type ModelPricing struct {
	InputPrice   float64
	OutputPrice  float64
	CachedPrice  float64
}

var builtinPricing = map[string]ModelPricing{
	"claude-sonnet-4":          {InputPrice: 3.0, OutputPrice: 15.0, CachedPrice: 0.30},
	"claude-sonnet-4-20250514": {InputPrice: 3.0, OutputPrice: 15.0, CachedPrice: 0.30},
	"claude-opus-4":            {InputPrice: 15.0, OutputPrice: 75.0, CachedPrice: 1.50},
	"claude-haiku-3.5":         {InputPrice: 0.8, OutputPrice: 4.0, CachedPrice: 0.08},
	"gpt-4o":                   {InputPrice: 2.5, OutputPrice: 10.0, CachedPrice: 1.25},
	"gpt-4o-mini":              {InputPrice: 0.15, OutputPrice: 0.6, CachedPrice: 0.075},
	"deepseek-v3":              {InputPrice: 0.27, OutputPrice: 1.1, CachedPrice: 0.07},
	"deepseek-r1":              {InputPrice: 0.55, OutputPrice: 2.19, CachedPrice: 0.14},
	"qwen-plus":                {InputPrice: 0.8, OutputPrice: 2.0, CachedPrice: 0},
	"qwen-max":                 {InputPrice: 2.0, OutputPrice: 6.0, CachedPrice: 1.0},
}

func DefaultPricing() ModelPricing {
	return ModelPricing{InputPrice: 3.0, OutputPrice: 15.0, CachedPrice: 0.30}
}

// GetPricing returns pricing for the given model name, falling back to a
// built-in table. Unknown models get DefaultPricing.
func GetPricing(modelName string) ModelPricing {
	if p, ok := builtinPricing[modelName]; ok {
		return p
	}
	return DefaultPricing()
}

// CalculateCost computes the total cost from token counts and pricing.
func CalculateCost(p ModelPricing, inputTokens, outputTokens, cachedTokens int) float64 {
	inputCost := float64(inputTokens) / 1_000_000 * p.InputPrice
	outputCost := float64(outputTokens) / 1_000_000 * p.OutputPrice
	cachedCost := float64(cachedTokens) / 1_000_000 * p.CachedPrice
	return inputCost + outputCost + cachedCost
}
