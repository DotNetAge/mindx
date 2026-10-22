package bundle

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// M9 组织运行面：组织信箱客户端方法。
// 信箱是松散载体：这里只搬运信封（收发件人/主题/正文/回信引用）与投递状态，
// 信内是工作安排、请求帮忙还是知会事项，由收方机器上的智能体作为第一读者解读。

// OrgIntrospection 组织令牌内省结果：对方相对本机登录者（校验者）的三层关系。
type OrgIntrospection struct {
	Active      bool   `json:"active"`
	PersonaID   int64  `json:"persona_id"`
	DisplayName string `json:"display_name"`
	JobTitle    string `json:"job_title"`
	OrgRole     string `json:"org_role"`
	Relation    string `json:"relation"` // parent 对方是我的上级 / subordinate 下级 / peer / self / none
}

// IntrospectOrg 令牌内省（POST /orgs/{slug}/introspect）：出示对方 AT，
// 服务端实时解析其身份与关系。收方 daemon 收直来信件时以此核验发信方身份。
func (m *MarketClient) IntrospectOrg(orgSlug, otherToken string) (*OrgIntrospection, error) {
	if err := m.ensureIdentity(); err != nil {
		return nil, err
	}
	var out OrgIntrospection
	if err := m.postJSON(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/introspect",
		map[string]string{"token": otherToken}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrgMail 信件（信封级字段；与组织信箱服务端响应对齐）。
type OrgMail struct {
	ID          int64  `json:"id"`
	UID         string `json:"uid"`
	FromPersona int64  `json:"from_persona"`
	ToPersona   int64  `json:"to_persona"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	InReplyTo   string `json:"in_reply_to"`
	DeadLine    string `json:"dead_line"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	DeliveredAt string `json:"delivered_at"`
	ReplyCount  int    `json:"reply_count"`
}

// SendOrgMail 发信入组织信箱（拿全局信件 ID 并留底；直连投递由 daemon 层接力加速）。
func (m *MarketClient) SendOrgMail(orgSlug string, toPersona int64, subject, body, inReplyTo, deadLine string) (*OrgMail, error) {
	if err := m.ensureIdentity(); err != nil {
		return nil, err
	}
	var out OrgMail
	err := m.postJSON(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/mails", map[string]any{
		"to_persona": toPersona, "subject": subject, "body": body, "in_reply_to": inReplyTo, "dead_line": deadLine,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// TakeOrgMailInbox 拉取组织收件箱（取走即回执：服务端标记已投递）。
func (m *MarketClient) TakeOrgMailInbox(orgSlug string) ([]*OrgMail, error) {
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) + "/mails/inbox")
	if err != nil {
		return nil, fmt.Errorf("收件箱暂时打不开：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("拉取收件箱失败（HTTP %d）", resp.StatusCode)
	}
	var body struct {
		Mails []*OrgMail `json:"mails"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body.Mails, nil
}

// ListOrgMailSent 发件清单（对账视图：对方是否已取、已收到的回信数）。
func (m *MarketClient) ListOrgMailSent(orgSlug string) ([]*OrgMail, error) {
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) + "/mails/sent")
	if err != nil {
		return nil, fmt.Errorf("发件清单暂时打不开：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取发件清单失败（HTTP %d）", resp.StatusCode)
	}
	var body struct {
		Mails []*OrgMail `json:"mails"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body.Mails, nil
}

// OrgMemberCard 组织成员名片（写信收件人选择与成员名解析的数据源）。
type OrgMemberCard struct {
	PersonaID   int64  `json:"persona_id"`
	DisplayName string `json:"display_name"`
	JobTitle    string `json:"job_title"`
	OrgRole     string `json:"org_role"`
}

// ListOrgMemberCards 在册成员名片清单。
func (m *MarketClient) ListOrgMemberCards(orgSlug string) ([]OrgMemberCard, error) {
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) + "/members")
	if err != nil {
		return nil, fmt.Errorf("成员名单暂时打不开：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取成员名单失败（HTTP %d）", resp.StatusCode)
	}
	var body struct {
		Members []OrgMemberCard `json:"members"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body.Members, nil
}

// ReportOrgEndpoint 上报本机私网可达地址（daemon 上线与定时刷新；幂等覆盖）。
func (m *MarketClient) ReportOrgEndpoint(orgSlug, addr string, port int) error {
	if err := m.ensureIdentity(); err != nil {
		return err
	}
	return m.postJSON(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/endpoints",
		map[string]any{"addr": addr, "port": port}, &struct{}{})
}

// AckOrgMail 直连收信回执（收方机器确认收到后调用）：服务端即标已投递，
// 寄件人的发件对账即时准确，不必等收方下一轮轮询。重复回执幂等。
func (m *MarketClient) AckOrgMail(orgSlug, uid string) error {
	if err := m.ensureIdentity(); err != nil {
		return err
	}
	return m.postJSON(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/mails/"+
		url.PathEscape(uid)+"/ack", map[string]any{}, &struct{}{})
}

// PeerEndpoint 对方机器的可达地址（reachable=false 表示尚未上报，直连不可用走信箱兜底）。
type PeerEndpoint struct {
	Reachable bool   `json:"reachable"`
	Addr      string `json:"addr"`
	Port      int    `json:"port"`
	UpdatedAt string `json:"updated_at"`
}

// OrgMemberEndpoint 查直上下级的机器可达地址（直连投递前寻址）。
func (m *MarketClient) OrgMemberEndpoint(orgSlug string, personaID int64) (*PeerEndpoint, error) {
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) +
		"/members/" + strconv.FormatInt(personaID, 10) + "/endpoint")
	if err != nil {
		return nil, fmt.Errorf("寻址暂时不可用：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("寻址失败（HTTP %d）", resp.StatusCode)
	}
	var out PeerEndpoint
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
