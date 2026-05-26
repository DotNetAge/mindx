package session

// TokenEstimator estimates token counts for text content.
type TokenEstimator interface {
	Estimate(text string) int
}

// NewTokenEstimator creates a simple char-count-based token estimator.
func NewTokenEstimator() TokenEstimator {
	return &charCountEstimator{}
}

type charCountEstimator struct{}

func (e *charCountEstimator) Estimate(text string) int {
	// Rough estimate: ~4 characters per token for Chinese/English mixed text.
	return len(text) / 4
}
