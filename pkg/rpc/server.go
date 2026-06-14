package rpc

import "encoding/json"

func (c *Client) ServerVersion() (json.RawMessage, error) {
	return c.CallWithTimeout("server.version", nil)
}

func (c *Client) ServerCheckUpdate() (json.RawMessage, error) {
	return c.CallWithTimeout("server.check_update", nil)
}

func (c *Client) ServerApplyUpdate() (json.RawMessage, error) {
	return c.CallWithTimeout("server.apply_update", nil)
}

func (c *Client) ServerRestartDaemon() (json.RawMessage, error) {
	return c.CallWithTimeout("server.restart_daemon", nil)
}
