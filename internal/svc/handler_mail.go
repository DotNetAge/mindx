package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bbolt "go.etcd.io/bbolt"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// M9 组织运行面：组织信箱（跨机器派工与汇报的载体）。
//
// 信箱是松散载体：信内是工作安排、请求帮忙还是知会事项，daemon 不解析不理会——
// 收信后叫醒本机智能体读信，由它分辨处置（自己干 / 转告人类伙伴 / 仅知会）。
//
// 收发单一通道：信一律落服务端组织信箱，收方 daemon 轮询拉取（已送达落档去重）。
// 直连投递口 mail.deliver 一期停用（原实现无身份核验，注释所声称的核验未落地）；
// 二期重接线时必须带发信方令牌经服务端内省核验直属上下级。
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
	DeadLine    string `json:"dead_line"` // 截止时刻（RFC3339；空 = 普通信）
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

// mailboxKeeper 信箱负责人（大秘）：固定默认名 assistant，注册表验证存在性；
// 注册表里没有 assistant 时回退取注册表首员代为收信。daemon 只把通知送到人，
// 不再隐式挑「第一个已雇佣」——收信职责落在谁头上是明确的、可从注册表追溯的。
func (d *Daemon) mailboxKeeper() string {
	agents := d.app.Agents()
	if agents == nil {
		return ""
	}
	list := agents.List()
	if len(list) == 0 {
		return ""
	}
	for _, a := range list {
		if a.Meta.Name == "assistant" {
			return "assistant"
		}
	}
	return list[0].Meta.Name
}

// wakeForMail 来信通知入队：信落本机档后只做一件事——给信箱负责人追加一条
// 下一秒执行的通知任务（固定模板，不含信件语义）。谁收、何时办、怎么办由
// 负责人读信后自行判断（组织架构、用户职责、个性化规则都在它的会话侧），
// 机制不预判处置方式；截止时刻等信封字段原样留档，由负责人读信自查。
func (d *Daemon) wakeForMail(uid string) {
	agent := d.mailboxKeeper()
	if agent == "" {
		d.logger.Warn("收到组织来信但智能体注册表为空，暂存待读", "uid", uid)
		return
	}
	if err := d.enqueueOneJobNow(agent, fmt.Sprintf("你有新的企业来信，id 为：%s，请查收。", uid)); err != nil {
		d.logger.Warn("来信通知入队失败", "uid", uid, "agent", agent, "error", err.Error())
	} else {
		d.logger.Info("来信已通知信箱负责人", "uid", uid, "agent", agent)
	}
}

// handleMailDeliver 直连投递入站（mail.deliver）：一期停用（已移出免握手白名单，
// 且原实现无发信方身份核验）。handler 与落档逻辑保留，待二期带令牌核验重接线。
func (d *Daemon) handleMailDeliver(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		OrgSlug     string `json:"org_slug"`
		UID         string `json:"uid"`
		FromPersona int64  `json:"from_persona"`
		Subject     string `json:"subject"`
		Body        string `json:"body"`
		InReplyTo   string `json:"in_reply_to"`
		DeadLine    string `json:"dead_line"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.UID == "" || strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("信封不完整：缺少信件编号或正文")
	}
	if d.mailSeen(p.UID) {
		return map[string]string{"status": "ok", "dedup": "1"}, nil
	}
	fromName := d.memberNameOf(p.OrgSlug, p.FromPersona)
	if err := d.mailSave(&LocalMail{
		UID: p.UID, OrgSlug: p.OrgSlug, FromPersona: p.FromPersona, FromName: fromName,
		Subject: p.Subject, Body: p.Body, InReplyTo: p.InReplyTo, DeadLine: p.DeadLine,
		ReceivedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		return nil, fmt.Errorf("信件留档失败: %w", err)
	}
	d.wakeForMail(p.UID)
	return map[string]string{"status": "ok"}, nil
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

// handleMailSend 处理 mail.send：发信 = 入服务端信箱（拿全局信件编号、留底），
// 对方上线轮询自取（收发单一通道）。
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
	// dead_line 截止时刻（可选）：给了就必须是合法 RFC3339，收方按它排期开工
	p.DeadLine = strings.TrimSpace(p.DeadLine)
	if p.DeadLine != "" {
		if _, err := time.Parse(time.RFC3339, p.DeadLine); err != nil {
			return nil, fmt.Errorf("截止时刻格式不正确，请使用如 2026-10-23T18:00:00+08:00 的写法")
		}
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
	mail, err := d.app.Market().SendOrgMail(orgSlug, card.PersonaID, p.Subject, p.Body, p.InReplyTo, p.DeadLine)
	if err != nil {
		return nil, err
	}
	// 信已落服务端信箱（收发单一通道）：对方上线轮询自取，对账视图看「对方已取」。
	return map[string]any{
		"uid": mail.UID, "org_slug": orgSlug, "to": card.DisplayName,
	}, nil
}

// handleMailReply 处理 mail.reply：按原信编号回给寄件人（回信也是一封信，先入服务端信箱）。
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
		"Re: "+origin.Subject, strings.TrimSpace(p.Body), origin.UID, "")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"uid": mail.UID, "in_reply_to": origin.UID,
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
