// Package rpc provides a thin JSON-RPC client for MindX daemon communication.
//
// It wraps the WebSocket-based gateway.Client and exposes typed methods for
// each daemon RPC method. This is the primary way CLI commands and external
// tools interact with the running daemon process.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
)

const (
	// DefaultAddr is the default daemon WebSocket endpoint.
	DefaultAddr = "ws://localhost:1314/ws"

	// DefaultTimeout is the default RPC call timeout.
	DefaultTimeout = 30 * time.Second
)

// Client is a thin JSON-RPC client for the MindX daemon.
type Client struct {
	gw *gateway.Client
}

// Dial connects to the MindX daemon at the given WebSocket address.
// If addr is empty, DefaultAddr is used.
func Dial(addr string) (*Client, error) {
	if addr == "" {
		addr = DefaultAddr
	}
	c := gateway.NewClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("cannot connect to daemon at %s: %w\nIs the daemon running? (mindx start)", addr, err)
	}
	return &Client{gw: c}, nil
}

// Close shuts down the client connection.
func (c *Client) Close() error {
	return c.gw.Close()
}

// Call invokes a JSON-RPC method and returns the raw result.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.gw.Call(ctx, method, params)
}

// CallWithTimeout calls a JSON-RPC method with a default timeout.
func (c *Client) CallWithTimeout(method string, params any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return c.Call(ctx, method, params)
}
