package rpc

import "encoding/json"

// I18nSwitchParams are the params for i18n.switch.
type I18nSwitchParams struct {
	Lang string `json:"lang"`
}

func (c *Client) I18nGet() (json.RawMessage, error) {
	return c.CallWithTimeout("i18n.get", nil)
}

func (c *Client) I18nSwitch(lang string) (json.RawMessage, error) {
	return c.CallWithTimeout("i18n.switch", I18nSwitchParams{Lang: lang})
}

func (c *Client) I18nList() (json.RawMessage, error) {
	return c.CallWithTimeout("i18n.list", nil)
}
