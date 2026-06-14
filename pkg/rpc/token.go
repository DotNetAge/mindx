package rpc

import "encoding/json"

// TokenUsageMonthlyParams are the params for token.usage.monthly.
type TokenUsageMonthlyParams struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

// TokenUsageByModelParams are the params for token.usage.by_model.
type TokenUsageByModelParams struct {
	Model string `json:"model"`
	Year  int    `json:"year,omitempty"`
	Month int    `json:"month,omitempty"`
}

// TokenUsageSessionParams are the params for token.usage.session.
type TokenUsageSessionParams struct {
	SessionID string `json:"session_id"`
}

func (c *Client) TokenUsageOverview() (json.RawMessage, error) {
	return c.CallWithTimeout("token.usage.overview", nil)
}

func (c *Client) TokenUsageMonthly(year, month int) (json.RawMessage, error) {
	return c.CallWithTimeout("token.usage.monthly", TokenUsageMonthlyParams{Year: year, Month: month})
}

func (c *Client) TokenUsageByModel(model string, year, month int) (json.RawMessage, error) {
	return c.CallWithTimeout("token.usage.by_model", TokenUsageByModelParams{
		Model: model, Year: year, Month: month,
	})
}

func (c *Client) TokenUsageTotal() (json.RawMessage, error) {
	return c.CallWithTimeout("token.usage.total", nil)
}

func (c *Client) TokenUsageSession(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("token.usage.session", TokenUsageSessionParams{SessionID: sessionID})
}
