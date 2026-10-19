package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core/bundle"
)

// ---------------------------------------------------------------------------
// M9 组织运行面：信件派发器（daemon 运行期后台循环）。
//
// 两项职责：
//  1. 地址上报：周期性向各组织登记本机私网可达地址，供直属上级/下级直连投递
//     （秒达路径）；掉线后地址由服务端按更新时间判定过期，信件自然走信箱兜底。
//  2. 收件箱轮询：周期性拉取服务端信箱里的信（兜底路径），去重落档后叫醒
//     本机智能体处置。直连已送达的信在本地留档去重，不会重复叫醒。
//
// 入站直投口 mail.deliver 与出站投递接力在 handler_mail.go；本文件只管后台节奏。
// ---------------------------------------------------------------------------

// mailPollInterval 收件箱轮询间隔（服务端压力与时效的折衷；直连投递不依赖本循环）。
const mailPollInterval = 60 * time.Second

// subscriptionWarned 已提示过「组织服务未开通」的组织（每进程一次，避免轮询刷屏）。
var subscriptionWarned sync.Map

// startMailLoop 启动信件后台循环（非阻塞；循环随 ctx 取消退出）。
func (d *Daemon) startMailLoop(ctx context.Context) {
	go func() {
		// 首轮延迟 30 秒：避开 daemon 启动尖峰，等网关、调度器等就绪后再开工
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		ticker := time.NewTicker(mailPollInterval)
		defer ticker.Stop()
		d.mailLoopOnce()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.mailLoopOnce()
			}
		}
	}()
}

// mailLoopOnce 一轮派发：逐个正常组织上报地址、拉取收件箱并分发。
func (d *Daemon) mailLoopOnce() {
	if d.app == nil || d.app.MarketAuthToken() == "" {
		return // 未登录资源社区：无信可拉，也无地址可报
	}
	orgs, err := d.app.Market().MeOrgs()
	if err != nil {
		d.logger.Debug("mail: 本轮跳过，组织清单暂时拿不到", "error", err.Error())
		return
	}
	for _, org := range orgs {
		if org.Status != "active" {
			continue
		}
		// 地址上报（幂等覆盖）：供上级/下级直连投递前寻址
		if err := d.app.Market().ReportOrgEndpoint(org.Slug, detectLANIP(), d.daemonPort()); err != nil {
			d.warnOrgBlocked(org.Slug, "地址上报", err)
		}
		// 收件箱轮询（取走即回执：服务端标记已投递）
		mails, err := d.app.Market().TakeOrgMailInbox(org.Slug)
		if err != nil {
			d.warnOrgBlocked(org.Slug, "收件箱拉取", err)
			continue
		}
		for _, m := range mails {
			d.receivePolledMail(org.Slug, m)
		}
	}
}

// warnOrgBlocked 组织信箱请求被挡下时的日志分级：服务未开通（403）用 Warn 明说原因
// （每组织每进程一次——基础流程建完了、订阅没开通，信箱会一直静默失效，必须让用户看得见）；
// 其余瞬时失败保持 Debug 不刷屏。
func (d *Daemon) warnOrgBlocked(orgSlug, action string, err error) {
	if strings.Contains(err.Error(), "HTTP 403") {
		if _, loaded := subscriptionWarned.LoadOrStore(orgSlug, true); loaded {
			return
		}
		d.logger.Warn("组织信箱暂不可用：该组织的服务未开通或已到期，开通后收发信会自动恢复",
			"org", orgSlug, "action", action)
		return
	}
	d.logger.Debug("mail: "+action+"未完成", "org", orgSlug, "error", err.Error())
}

// receivePolledMail 接收一封轮询拉到的信：去重（直连已收）→ 落档 → 叫醒智能体。
func (d *Daemon) receivePolledMail(orgSlug string, m *bundle.OrgMail) {
	if m == nil || d.mailSeen(m.UID) {
		return
	}
	fromName := d.memberNameOf(orgSlug, m.FromPersona)
	if err := d.mailSave(&LocalMail{
		UID: m.UID, OrgSlug: orgSlug, FromPersona: m.FromPersona, FromName: fromName,
		Subject: m.Subject, Body: m.Body, InReplyTo: m.InReplyTo,
		ReceivedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		d.logger.Warn("mail: 信件留档失败", "uid", m.UID, "error", err.Error())
		return
	}
	d.wakeForMail(fromName, m.Subject, m.Body, m.UID, m.InReplyTo)
}

// memberNameOf 按编号反查成员显示名（寄件人名字展示用；查不到回退编号写法）。
func (d *Daemon) memberNameOf(orgSlug string, personaID int64) string {
	fallback := fmt.Sprintf("成员 #%d", personaID)
	if d.app == nil {
		return fallback
	}
	cards, err := d.app.Market().ListOrgMemberCards(orgSlug)
	if err != nil {
		return fallback
	}
	for _, c := range cards {
		if c.PersonaID == personaID {
			return c.DisplayName
		}
	}
	return fallback
}
