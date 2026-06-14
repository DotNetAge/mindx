package rpc

import "encoding/json"

// EntityTagDef is a single entity tag definition.
type EntityTagDef struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Category string `json:"category,omitempty"`
}

// EntityTagsSaveParams are the params for entity_tags.save.
type EntityTagsSaveParams struct {
	Types []EntityTagDef `json:"types"`
}

func (c *Client) EntityTagsGet() (json.RawMessage, error) {
	return c.CallWithTimeout("entity_tags.get", nil)
}

func (c *Client) EntityTagsSave(types []EntityTagDef) (json.RawMessage, error) {
	return c.CallWithTimeout("entity_tags.save", EntityTagsSaveParams{Types: types})
}
