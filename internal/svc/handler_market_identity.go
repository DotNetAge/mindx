package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core/bundle"
)

// ---------------------------------------------------------------------------
// market.account.identity.* / market.org.create RPC —— 社区账号绑定与组织创建
// （2026-10-20 多社区身份定案）。
//
// 绑定编排与设备登录（handler_market_device.go）同构：link 受理即返回——调服务端
// 发起绑定拿一次性授权入口 URL，经既有 link_open 广播通道下发客户端（一律系统
// 浏览器打开，可复用用户既有第三方会话），后台 goroutine 2s 轮询服务端绑定进度；
// 用户在浏览器完成第三方授权后服务端落库，daemon 轮询到 done 收口。
// status 供前端定时查询渲染进度。解绑为同步转发（服务端把关主身份与最后一条）。
// ---------------------------------------------------------------------------

// marketIdentityPollInterval 绑定进度轮询间隔。
const marketIdentityPollInterval = 2 * time.Second

// marketIdentityLinkState 绑定编排状态（单飞：同一时刻至多一条进行中流程）。
type marketIdentityLinkState struct {
	mu     sync.Mutex
	phase  string // idle | pending | done | error
	errMsg string
}

func (s *marketIdentityLinkState) set(phase, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase, s.errMsg = phase, errMsg
}

func (s *marketIdentityLinkState) snapshot() (phase, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == "" {
		return "idle", ""
	}
	return s.phase, s.errMsg
}

// handleMarketIdentityLink 发起社区账号绑定（link_open 广播授权页；单飞幂等）。
func (d *Daemon) handleMarketIdentityLink(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Provider string `json:"provider"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(p.Provider)
	if provider == "" {
		return nil, fmt.Errorf("provider 为必填")
	}
	// 单飞闸：进行中直接回当前状态（前端轮询 status 会继续跟踪）
	if phase, _ := d.marketIdentity.snapshot(); phase == "pending" {
		return map[string]any{"status": "pending"}, nil
	}
	linkID, authURL, err := d.app.Market().IdentityLinkStart(provider)
	if err != nil {
		d.marketIdentity.set("error", err.Error())
		return nil, err
	}
	d.marketIdentity.set("pending", "")

	// 复用既有 link_open 广播通道：系统浏览器打开授权页（复用用户既有第三方会话）
	d.broadcastUI("link_open", map[string]any{
		"url": authURL,
	})
	d.logger.Info("社区账号绑定已发起", "provider", provider)

	go d.pollMarketIdentityLink(linkID)
	return map[string]any{"status": "pending"}, nil
}

// pollMarketIdentityLink 后台轮询绑定进度：pending 继续直至 done/failed/超时。
// done 后账户信息由前端重新 market.account.get 拉取（绑定结果在 /me identities）。
func (d *Daemon) pollMarketIdentityLink(linkID string) {
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(marketIdentityPollInterval)
		status, errMsg, err := d.app.Market().IdentityLinkStatus(linkID)
		if err != nil {
			// 单次网络抖动不终止：继续轮询到超时
			d.logger.Warn("绑定进度轮询失败", "error", err)
			continue
		}
		switch status {
		case "done":
			d.marketIdentity.set("done", "")
			d.logger.Info("社区账号绑定完成")
			return
		case "failed":
			if errMsg == "" {
				errMsg = "绑定失败，请稍后重试"
			}
			d.marketIdentity.set("error", errMsg)
			return
		default:
			// pending，继续
		}
	}
	d.marketIdentity.set("error", "绑定已超时，请重新发起")
}

// handleMarketIdentityStatus 查询绑定编排状态（前端 1.5s 轮询渲染进度）。
func (d *Daemon) handleMarketIdentityStatus(_ context.Context, _ json.RawMessage) (any, error) {
	phase, errMsg := d.marketIdentity.snapshot()
	resp := map[string]any{"status": phase}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	return resp, nil
}

// handleMarketIdentityUnlink 解绑社区账号（同步转发；服务端拒绝主身份与最后一条）。
func (d *Daemon) handleMarketIdentityUnlink(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Provider string `json:"provider"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(p.Provider)
	if provider == "" {
		return nil, fmt.Errorf("provider 为必填")
	}
	if err := d.app.Market().IdentityUnlink(provider); err != nil {
		return nil, err
	}
	d.logger.Info("社区账号已解绑", "provider", provider)
	return map[string]any{"unlinked": provider}, nil
}

// handleMarketOrgCreate 创建组织（POST /api/v1/orgs：名称 + 别名；创建者即所有者）。
// 组织面板「建立入口」人机通道（与 Agent 自建组织能力互补）。
func (d *Daemon) handleMarketOrgCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	name, slug := strings.TrimSpace(p.Name), strings.TrimSpace(p.Slug)
	if name == "" || slug == "" {
		return nil, fmt.Errorf("组织名称与别名均为必填")
	}
	orgID, err := d.app.Market().OrgCreate(name, slug)
	if err != nil {
		return nil, err
	}
	d.logger.Info("组织创建完成", "name", name, "slug", slug)
	return map[string]any{"org_id": orgID, "slug": slug, "name": name}, nil
}

// handleMarketOrgDelete 删除组织（DELETE /orgs/{slug}；仅所有者，组织须为空：
// 无其他成员且无已发布包，服务端把关并在拒绝时给出人话原因）。
func (d *Daemon) handleMarketOrgDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Slug string `json:"slug"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return nil, fmt.Errorf("组织别名为必填")
	}
	if err := d.app.Market().OrgDelete(slug); err != nil {
		return nil, err
	}
	d.logger.Info("组织已删除", "slug", slug)
	return map[string]any{"deleted": true, "slug": slug}, nil
}

// handleMarketOrgMembers 组织成员名片清单（GET /orgs/{slug}/members；组织中心详情用）。
func (d *Daemon) handleMarketOrgMembers(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Slug string `json:"slug"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return nil, fmt.Errorf("组织别名为必填")
	}
	members, err := d.app.Market().ListOrgMemberCards(slug)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []bundle.OrgMemberCard{}
	}
	return map[string]any{"members": members}, nil
}

// handleMarketOrgSubscription 组织订阅状态（GET /orgs/{slug}/subscription）。
// 免费开放期 status = none，组织中心渲染「免费开放中」；收费期渲染订阅明细。
func (d *Daemon) handleMarketOrgSubscription(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Slug string `json:"slug"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return nil, fmt.Errorf("组织别名为必填")
	}
	sub, err := d.app.Market().OrgSubscription(slug)
	if err != nil {
		return nil, err
	}
	return map[string]any{"subscription": sub}, nil
}
