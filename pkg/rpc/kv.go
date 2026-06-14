package rpc

import "encoding/json"

// KVGetParams are the params for kvstore.get.
type KVGetParams struct {
	Key string `json:"key"`
}

// KVSetParams are the params for kvstore.set.
type KVSetParams struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   int         `json:"ttl,omitempty"`
}

// KVDeleteParams are the params for kvstore.delete.
type KVDeleteParams struct {
	Key string `json:"key"`
}

// KVListParams are the params for kvstore.list.
type KVListParams struct {
	Prefix     string `json:"prefix,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	WithValues bool   `json:"with_values,omitempty"`
}

// KVBatchSetEntry is a single entry in a batch set operation.
type KVBatchSetEntry struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   int         `json:"ttl,omitempty"`
}

// KVBatchSetParams are the params for kvstore.batch_set.
type KVBatchSetParams struct {
	Entries []KVBatchSetEntry `json:"entries"`
}

// KVClearParams are the params for kvstore.clear.
type KVClearParams struct {
	Prefix string `json:"prefix"`
}

func (c *Client) KVGet(key string) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.get", KVGetParams{Key: key})
}

func (c *Client) KVSet(key string, value interface{}, ttl int) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.set", KVSetParams{Key: key, Value: value, TTL: ttl})
}

func (c *Client) KVDelete(key string) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.delete", KVDeleteParams{Key: key})
}

func (c *Client) KVList(prefix string, limit int, withValues bool) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.list", KVListParams{
		Prefix:     prefix,
		Limit:      limit,
		WithValues: withValues,
	})
}

func (c *Client) KVBatchSet(entries []KVBatchSetEntry) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.batch_set", KVBatchSetParams{Entries: entries})
}

func (c *Client) KVClear(prefix string) (json.RawMessage, error) {
	return c.CallWithTimeout("kvstore.clear", KVClearParams{Prefix: prefix})
}
