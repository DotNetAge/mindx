package rpc

import "encoding/json"

// SchemaGetParams are params for schema.get
type SchemaGetParams struct {
	Category string `json:"category"`
	Name     string `json:"name"`
}

// SchemaSaveParams are params for schema.save
type SchemaSaveParams struct {
	Category string          `json:"category"`
	Name     string          `json:"name"`
	Schema   json.RawMessage `json:"schema"`
}

func (c *Client) SchemaGet(category, name string) (json.RawMessage, error) {
	return c.CallWithTimeout("schema.get", SchemaGetParams{
		Category: category,
		Name:     name,
	})
}

func (c *Client) SchemaSave(category, name string, schema json.RawMessage) (json.RawMessage, error) {
	return c.CallWithTimeout("schema.save", SchemaSaveParams{
		Category: category,
		Name:     name,
		Schema:   schema,
	})
}

func (c *Client) SchemaList() (json.RawMessage, error) {
	return c.CallWithTimeout("schema.list", nil)
}
