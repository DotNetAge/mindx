package rpc

import "encoding/json"

// LogReadParams are the params for log.read.
type LogReadParams struct {
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Stream string `json:"stream,omitempty"`
}

// LogClearParams are the params for log.clear.
type LogClearParams struct {
	Confirmed bool `json:"confirmed"`
}

func (c *Client) LogRead(offset, limit int, stream string) (json.RawMessage, error) {
	return c.CallWithTimeout("log.read", LogReadParams{
		Offset: offset, Limit: limit, Stream: stream,
	})
}

func (c *Client) LogClear(confirmed bool) (json.RawMessage, error) {
	return c.CallWithTimeout("log.clear", LogClearParams{Confirmed: confirmed})
}

func (c *Client) LogCount() (json.RawMessage, error) {
	return c.CallWithTimeout("log.count", nil)
}
