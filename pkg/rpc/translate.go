package rpc

import "encoding/json"

// TranslateParams are the params for translate.rpc.
type TranslateParams struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

// TranslateResult is the result of translate.rpc.
type TranslateResult struct {
	Text   string `json:"text"`
	Cached bool   `json:"cached"`
}

func (c *Client) Translate(text, lang string) (json.RawMessage, error) {
	return c.CallWithTimeout("translate.rpc", TranslateParams{Text: text, Lang: lang})
}
