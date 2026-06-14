package rpc

import "encoding/json"

func (c *Client) UserConfig() (json.RawMessage, error) {
	return c.CallWithTimeout("user.config", nil)
}
