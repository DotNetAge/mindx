package rpc

import "encoding/json"

// TerminalStartParams are the params for terminal.start.
type TerminalStartParams struct {
	Cwd string `json:"cwd"`
}

// TerminalInputParams are the params for terminal.input.
type TerminalInputParams struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

// TerminalResizeParams are the params for terminal.resize.
type TerminalResizeParams struct {
	SessionID string `json:"session_id"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
	X         uint16 `json:"x,omitempty"`
	Y         uint16 `json:"y,omitempty"`
}

// TerminalKillParams are the params for terminal.kill.
type TerminalKillParams struct {
	SessionID string `json:"session_id"`
}

func (c *Client) TerminalStart(cwd string) (json.RawMessage, error) {
	return c.CallWithTimeout("terminal.start", TerminalStartParams{Cwd: cwd})
}

func (c *Client) TerminalInput(sessionID, data string) (json.RawMessage, error) {
	return c.CallWithTimeout("terminal.input", TerminalInputParams{
		SessionID: sessionID, Data: data,
	})
}

func (c *Client) TerminalResize(sessionID string, rows, cols, x, y uint16) (json.RawMessage, error) {
	return c.CallWithTimeout("terminal.resize", TerminalResizeParams{
		SessionID: sessionID, Rows: rows, Cols: cols, X: x, Y: y,
	})
}

func (c *Client) TerminalKill(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("terminal.kill", TerminalKillParams{SessionID: sessionID})
}

func (c *Client) TerminalList() (json.RawMessage, error) {
	return c.CallWithTimeout("terminal.list", nil)
}
