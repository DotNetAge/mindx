package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bbolt "go.etcd.io/bbolt"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// M9 组织运行面：组织信箱（跨机器派工与汇报的载体）。
//
// 信箱是松散载体：信内是工作安排、请求帮忙还是知会事项，daemon 不解析不理会——
// 收信后叫醒本机智能体读信，由它分辨处置（自己干 / 转告人类伙伴 / 仅知会）。
//
// 两条投递路径：
//   直连（优先）——对方智能主机私网可达时，出站连接器携发信方访问令牌直投 mail.deliver；
//   信箱（兜底）——直连不通，信留在服务端信箱，收方 daemon 轮询拉取。
// 收方核验（不信任来自客户端的任何声明）：持本机登录态调服务端内省，
// 确认发信人确实是自己的直属上级/下级后才收信落档。
// ---------------------------------------------------------------------------

// mailInboxBucket 本地信件留档桶（kvstore bbolt 内独立 bucket；key=信件编号）。
var mailInboxBucket = []byte("mail_inbox")

// mailArchiveCap 本地留档上限（超出按收信时间淘汰最旧，防止长期膨胀）。
const mailArchiveCap = 200

// LocalMail 本地留档的信（服务端信封字段 + 投递上下文）。
type LocalMail struct {
	UID         string `json:"uid"`
	OrgSlug     string `json:"org_slug"`
	FromPersona int64  `json:"from_persona"`
	FromName    string `json:"from_name"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	InReplyTo   string `json:"in_reply_to"`
	ReceivedAt  string `json:"received_at"`
}

// mailSave 留档一封来信（重复编号覆盖 = 幂等）。
func (d *Daemon) mailSave(m *LocalMail) error {
	if d.kvStore == nil {
		return fmt.Errorf("本地存储未就绪")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return d.kvStore.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(mailInboxBucket)
		if err != nil {
			return err
		}
		return b.Put([]byte(m.UID), data)
	})
}

// mailSeen 是否已留档过该信（直连已收的信，轮询拉取时去重跳过）。
func (d *Daemon) mailSeen(uid string) bool {
	if d.kvStore == nil {
		return false
	}
	seen := false
	_ = d.kvStore.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(mailInboxBucket)
		if b != nil {
			seen = b.Get([]byte(uid)) != nil
		}
		return nil
	})
	return seen
}

// mailList 全部留档信件（按收信时间倒序；超出上限淘汰最旧）。
func (d *Daemon) mailList() []LocalMail {
	out := make([]LocalMail, 0, 8)
	if d.kvStore == nil {
		return out
	}
	_ = d.kvStore.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(mailInboxBucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var m LocalMail
			if json.Unmarshal(v, &m) == nil {
				out = append(out, m)
			}
			return nil
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ReceivedAt > out[j].ReceivedAt })
	if len(out) > mailArchiveCap {
		out = out[:mailArchiveCap]
	}
	return out
}

// mailFind 按编号取留档信（回信定位来信人用）。
func (d *Daemon) mailFind(uid string) *LocalMail {
	list := d.mailList()
	for i := range list {
		if list[i].UID == uid {
			return &list[i]
		}
	}
	return nil
}

// ackMailAsync 直连收信回执（异步尽力而为）：服务端把信标已投递，寄件人的
// 发件对账即时准确；失败不影响收信（对方轮询拉取路径仍会兜底标记）。
func (d *Daemon) ackMailAsync(orgSlug, uid string) {
	go func() {
		if err := d.app.Market().AckOrgMail(orgSlug, uid); err != nil {
			d.logger.Debug("mail: 收信回执未完成（不影响收信，轮询会兜底）", "uid", uid, "error", err.Error())
		}
	}()
}

// handleMailDeliver 处理 mail.deliver：跨机器直连投递的入站口（openMethods 白名单）。
// 防滥用依据：方法本身只做「核验 + 落档 + 叫醒」，核验必须凭发信方访问令牌经
// 服务端内省通过（组织信任替代设备握手闸），伪造者无法出示有效上级/下级令牌。
func (d *Daemon) handleMailDeliver(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MailDeliverParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.OrgSlug == "" || p.Token == "" || p.UID == "" || p.Body == "" {
		return nil, fmt.Errorf("信件不完整（缺少组织、凭证或正文）")
	}
	myToken := d.app.MarketAuthToken()
	if myToken == "" {
		return nil, fmt.Errorf("本机还没有登录资源社区，无法核验来信身份；请先登录后再收组织来信")
	}
	intro, err := d.app.Market().IntrospectOrg(p.OrgSlug, p.Token)
	if err != nil {
		return nil, fmt.Errorf("来信身份核验暂时不可用，请稍后再试：%w", err)
	}
	// 方向校验：down（上级发给我）要求发信人是我的直属上级；up（下级回给我）要求是我的直属下级
	want := "parent"
	if p.Direction != "down" {
		want = "subordinate"
	}
	if !intro.Active || intro.Relation != want {
		return nil, fmt.Errorf("这封信没有通过身份核验：只有你的直属上级寄来的信、或直属下级的回信才会接收")
	}
	if d.mailSeen(p.UID) {
		// 直连重复投递：上次可能收下了但回执没送达，补一次回执（服务端幂等）
		d.ackMailAsync(p.OrgSlug, p.UID)
		return map[string]any{"ok": true, "duplicate": true}, nil
	}
	if err := d.mailSave(&LocalMail{
		UID: p.UID, OrgSlug: p.OrgSlug, FromPersona: intro.PersonaID,
		FromName: intro.DisplayName, Subject: p.Subject, Body: p.Body,
		InReplyTo: p.InReplyTo, ReceivedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		return nil, err
	}
	// 直连已送达：立即回执服务端，寄件人的发件对账即刻显示「对方已取」
	d.ackMailAsync(p.OrgSlug, p.UID)
	d.wakeForMail(intro.DisplayName, p.Subject, p.Body, p.UID, p.InReplyTo)
	return map[string]any{"ok": true}, nil
}

// defaultHiredAgent 本机默认负责人（第一个已雇佣的智能体；来信处置与自动开工的执行者）。
func (d *Daemon) defaultHiredAgent() string {
	if agents := d.app.Agents(); agents != nil {
		if hired := agents.Hired(); len(hired) > 0 {
			return hired[0].Meta.Name
		}
	}
	return ""
}

// wakeForMail 来信叫醒：把信渲染为本机智能体的开工内容并即刻入队。
// 智能体是信的第一读者，由它判断来信意图并处置；信件编号贯穿回信对账。
func (d *Daemon) wakeForMail(fromName, subject, body, uid, inReplyTo string) {
	agent := d.defaultHiredAgent()
	if agent == "" {
		d.logger.Warn("收到组织来信但本机没有已雇佣的智能体，暂存待读", "uid", uid)
		return
	}
	var sb strings.Builder
	sb.WriteString("【组织来信】\n")
	fmt.Fprintf(&sb, "寄件人：%s\n主题：%s\n信件编号：%s\n", fromName, subject, uid)
	if inReplyTo != "" {
		fmt.Fprintf(&sb, "这是对信件 %s 的回信。\n", inReplyTo)
	}
	sb.WriteString("\n正文：\n")
	sb.WriteString(body)
	sb.WriteString("\n\n请判断这封信要你做什么：需要动手的事，现在着手做；需要本机主人知晓或裁决的，清楚转告并等指示；")
	fmt.Fprintf(&sb, "事情办完后务必回信汇报（命令：mindx mail reply %s \"汇报内容\"），让寄件人知道结果。", uid)
	if err := d.enqueueOneJob(agent, sb.String()); err != nil {
		d.logger.Warn("来信开工入队失败", "uid", uid, "error", err.Error())
	} else {
		d.logger.Info("来信已叫醒智能体处置", "uid", uid, "agent", agent)
	}
}

// resolveOrgSlug 发信组织解析：显式指定优先；缺省取在册的第一个正常组织。
func (d *Daemon) resolveOrgSlug(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	orgs, err := d.app.Market().MeOrgs()
	if err != nil {
		return "", err
	}
	for _, org := range orgs {
		if org.Status == "active" {
			return org.Slug, nil
		}
	}
	return "", fmt.Errorf("你还没有加入任何组织，无法发信")
}

// findMemberCard 成员名片按显示名精确匹配（收件人解析）。
func findMemberCard(cards []bundle.OrgMemberCard, name string) *bundle.OrgMemberCard {
	for i := range cards {
		if cards[i].DisplayName == name {
			return &cards[i]
		}
	}
	return nil
}

// handleMailSend 处理 mail.send：发信 = 入服务端信箱（拿全局信件编号、留底）→
// 直连投递接力（对方机器私网可达时秒达；不通则留信箱等对方上线拉取）。
func (d *Daemon) handleMailSend(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MailSendParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.To) == "" {
		return nil, fmt.Errorf("请指定收信人")
	}
	if strings.TrimSpace(p.Subject) == "" {
		return nil, fmt.Errorf("请填写信件主题")
	}
	if strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("信不能没有正文")
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	cards, err := d.app.Market().ListOrgMemberCards(orgSlug)
	if err != nil {
		return nil, err
	}
	card := findMemberCard(cards, strings.TrimSpace(p.To))
	if card == nil {
		return nil, fmt.Errorf("组织里没有叫 %q 的成员，请核对名字", strings.TrimSpace(p.To))
	}
	mail, err := d.app.Market().SendOrgMail(orgSlug, card.PersonaID, p.Subject, p.Body, p.InReplyTo)
	if err != nil {
		return nil, err
	}
	delivered := d.deliverMailDirectly(orgSlug, card.PersonaID, mail.UID, p.Subject, p.Body, p.InReplyTo, "down")
	return map[string]any{
		"uid": mail.UID, "org_slug": orgSlug, "to": card.DisplayName,
		"delivered_directly": delivered,
	}, nil
}

// handleMailReply 处理 mail.reply：按原信编号回给寄件人（回信也是一封信，同样先入信箱再接力直连）。
func (d *Daemon) handleMailReply(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MailReplyParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("回信不能没有内容")
	}
	origin := d.mailFind(strings.TrimSpace(p.UID))
	if origin == nil {
		return nil, fmt.Errorf("本地没有编号为 %s 的来信，无法回信（先 mindx mail inbox 看看收件箱）", strings.TrimSpace(p.UID))
	}
	mail, err := d.app.Market().SendOrgMail(origin.OrgSlug, origin.FromPersona,
		"Re: "+origin.Subject, strings.TrimSpace(p.Body), origin.UID)
	if err != nil {
		return nil, err
	}
	delivered := d.deliverMailDirectly(origin.OrgSlug, origin.FromPersona, mail.UID,
		mail.Subject, strings.TrimSpace(p.Body), origin.UID, "up")
	return map[string]any{
		"uid": mail.UID, "in_reply_to": origin.UID, "delivered_directly": delivered,
	}, nil
}

// handleMailInbox 处理 mail.inbox：本机收件箱（已送达落档的信，最新在前）。
func (d *Daemon) handleMailInbox(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]any{"mails": d.mailList()}, nil
}

// handleMailSent 处理 mail.list：发件清单（服务端对账视图：对方是否已取、已收回信数）。
func (d *Daemon) handleMailSent(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MailListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	mails, err := d.app.Market().ListOrgMailSent(orgSlug)
	if err != nil {
		return nil, err
	}
	return map[string]any{"org_slug": orgSlug, "mails": mails}, nil
}

// handleMailMembers 处理 mail.members：在册成员名片（写信选人与收件人解析的数据源）。
// me_persona 是本机在组织的成员编号：写信选人据此排除自己（服务端也不接受自寄）。
func (d *Daemon) handleMailMembers(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MailListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	cards, err := d.app.Market().ListOrgMemberCards(orgSlug)
	if err != nil {
		return nil, err
	}
	// 本机 persona（内省自己；失败置 0，前端不过滤降级为可选）
	mePersona := int64(0)
	if myToken := d.app.MarketAuthToken(); myToken != "" {
		if intro, err := d.app.Market().IntrospectOrg(orgSlug, myToken); err == nil {
			mePersona = intro.PersonaID
		}
	}
	return map[string]any{"org_slug": orgSlug, "members": cards, "me_persona": mePersona}, nil
}

// gatewayCaller 直连投递所需的最小 WS 客户端接口（便于测试替身）。
type gatewayCaller interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Close() error
}

// dialPeer 直连对方智能主机的拨号函数（变量化便于测试替换）。
var dialPeer = func(addr string) (gatewayCaller, error) {
	c := gateway.NewClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// deliverMailDirectly 直连投递接力（私网直连优先）：查对方可达地址 →
// 裸 WS 连对方智能主机调 mail.deliver（出示发信方访问令牌）→ 不通静默放弃（信已在服务端信箱，对方上线自会拉取）。
func (d *Daemon) deliverMailDirectly(orgSlug string, toPersona int64, uid, subject, body, inReplyTo, direction string) bool {
	ep, err := d.app.Market().OrgMemberEndpoint(orgSlug, toPersona)
	if err != nil || !ep.Reachable {
		return false
	}
	myToken := d.app.MarketAuthToken()
	if myToken == "" {
		return false
	}
	c, err := dialPeer(fmt.Sprintf("ws://%s:%d%s", ep.Addr, ep.Port, d.WSPath()))
	if err != nil {
		return false
	}
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = c.Call(ctx, "mail.deliver", rpc.MailDeliverParams{
		OrgSlug: orgSlug, Token: myToken, UID: uid, Subject: subject, Body: body,
		InReplyTo: inReplyTo, Direction: direction,
	})
	return err == nil
}
