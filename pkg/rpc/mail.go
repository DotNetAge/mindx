package rpc

import "encoding/json"

// M9 组织运行面：组织信箱 RPC 参数（CLI 与前端共用冻结协议）。
// 信箱是松散载体：参数只有信封级字段，信体由收方智能体自行解读。

// MailSendParams 发信参数（mail.send：信落服务端组织信箱，对方上线自取）。
type MailSendParams struct {
	// OrgSlug 目标组织（空 = 自动取我在册的第一个正常组织）
	OrgSlug string `json:"org_slug,omitempty"`
	// To 收件成员显示名（组织成员名单精确匹配）
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	InReplyTo string `json:"in_reply_to,omitempty"`
}

// MailReplyParams 回信参数（mail.reply：按原信编号回给来信人）。
type MailReplyParams struct {
	UID  string `json:"uid"`
	Body string `json:"body"`
}

// MailListParams 信箱查询参数（mail.list 发件对账 / mail.inbox 收件箱）。
type MailListParams struct {
	OrgSlug string `json:"org_slug,omitempty"`
}

// MailSend 发信（组织信箱）。
func (c *Client) MailSend(p MailSendParams) (json.RawMessage, error) {
	return c.CallWithTimeout("mail.send", p)
}

// MailReply 按原信编号回信。
func (c *Client) MailReply(p MailReplyParams) (json.RawMessage, error) {
	return c.CallWithTimeout("mail.reply", p)
}

// MailInbox 本机收件箱（已送达落档的信）。
func (c *Client) MailInbox(p MailListParams) (json.RawMessage, error) {
	return c.CallWithTimeout("mail.inbox", p)
}

// MailSent 发件清单（对账：对方是否已取、回信数）。
func (c *Client) MailSent(p MailListParams) (json.RawMessage, error) {
	return c.CallWithTimeout("mail.list", p)
}
