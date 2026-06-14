package rpc

import "encoding/json"

// TranslateParams are the params for translate.rpc.
type TranslateParams struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

func (c *Client) Translate(text, lang string) (json.RawMessage, error) {
	return c.CallWithTimeout("translate.rpc", TranslateParams{Text: text, Lang: lang})
}
