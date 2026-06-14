package rpc

import "encoding/json"

func (c *Client) FilewatchStart() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.start", nil)
}

func (c *Client) FilewatchStop() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.stop", nil)
}

func (c *Client) FilewatchStatus() (json.RawMessage, error) {
	return c.CallWithTimeout("filewatch.status", nil)
}
