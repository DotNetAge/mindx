package rpc

import "encoding/json"

// M9 组织运行面：组织信箱 RPC 参数（CLI 与前端共用冻结协议）。
// 信箱是松散载体：参数只有信封级字段，信体由收方智能体自行解读。

// MailDeliverParams 跨机器直连投递的入站参数（mail.deliver）。
// Token 是发信方的资源社区访问令牌：收方 daemon 持本机登录态向服务端内省，
// 校验「发信人确实是我的直属上级/下级」后才收信（不信任来自客户端的任何声明）。
type MailDeliverParams struct {
	OrgSlug   string `json:"org_slug"`
	Token     string `json:"token"`
	UID       string `json:"uid"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	InReplyTo string `json:"in_reply_to,omitempty"`
	// Direction 投递方向声明（仅决定校验期望，无授权效力）：
	// "down" = 上级发给我（发信人须内省为 parent）；"up" = 下级回给我（发信人须内省为 subordinate）。
	Direction string `json:"direction"`
	// FromName 发信方显示名（渲染用；真实身份以服务端内省结果为准）
	FromName string `json:"from_name,omitempty"`
}

// MailSendParams 发信参数（mail.send；经本机 daemon 转发到组织信箱并接力直连投递）。
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

// MailSend 发信（组织信箱 + 直连投递接力）。
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
