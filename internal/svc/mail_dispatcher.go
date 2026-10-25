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
// 职责：收件箱轮询——周期性拉取服务端信箱里的信，去重落档后通知信箱负责人。
// 直连秒达路径（地址上报 + mail.deliver 直投口）一期停用；二期重接线时恢复
// 地址上报，并给直投口补发信方令牌经服务端内省核验直属上下级。
// ---------------------------------------------------------------------------

// mailPollInterval 收件箱轮询间隔（服务端压力与时效的折衷；收信唯一通道）。
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

// mailLoopOnce 一轮派发：逐个正常组织拉取收件箱并分发。
func (d *Daemon) mailLoopOnce() {
	if d.app == nil || d.app.MarketAuthToken() == "" {
		return // 未登录资源社区：无信可拉
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

// receivePolledMail 接收一封轮询拉到的信：去重 → 落档 → 通知信箱负责人。
func (d *Daemon) receivePolledMail(orgSlug string, m *bundle.OrgMail) {
	if m == nil || d.mailSeen(m.UID) {
		return
	}
	fromName := d.memberNameOf(orgSlug, m.FromPersona)
	if err := d.mailSave(&LocalMail{
		UID: m.UID, OrgSlug: orgSlug, FromPersona: m.FromPersona, FromName: fromName,
		Subject: m.Subject, Body: m.Body, InReplyTo: m.InReplyTo, DeadLine: m.DeadLine,
		ReceivedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		d.logger.Warn("mail: 信件留档失败", "uid", m.UID, "error", err.Error())
		return
	}
	d.wakeForMail(m.UID)
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
