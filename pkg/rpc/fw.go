package rpc

import "encoding/json"

// FilewatchRemoveParams are the params for filewatch.remove.
type FilewatchRemoveParams struct {
	Dir string `json:"dir"`
}

// FilewatchRetryFailedParams are the params for filewatch.retry-failed.
type FilewatchRetryFailedParams struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files,omitempty"`
}

// FilewatchIgnoreFailedParams are the params for filewatch.ignore-failed.
type FilewatchIgnoreFailedParams struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files,omitempty"`
}

func (c *Client) FilewatchStart() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.start", nil)
}

func (c *Client) FilewatchStop() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.stop", nil)
}

func (c *Client) FilewatchStatus() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.status", nil)
}
