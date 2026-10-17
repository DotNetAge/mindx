package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core/bundle"
)

// ---------------------------------------------------------------------------
// market.device.* RPC —— MindX 账号设备授权登录（device flow，市场生态 §2.4）。
//
// 编排：login 受理即返回——调服务端 start 拿确认页 URL，经既有 link_open 广播
// 通道下发客户端（一律系统浏览器打开：支付宝/小米授权页对内嵌 WebView 有风控，
// 系统浏览器还能复用用户既有社区会话免重复登录），后台 goroutine 按 interval
// 轮询；用户在官网确认页点「授权此设备」后 poll 下发 AuthToken，走
// finishMarketBind 收口（校验归属 + 绑定迁移）。
// status 供前端定时查询渲染进度。全程无短码、无粘贴，用户只点一次确认。
// ---------------------------------------------------------------------------

// marketDevicePollInterval 轮询间隔（服务端 start 响应的 interval 语义：秒）。
const marketDevicePollInterval = 2 * time.Second

// marketDeviceLoginState 设备登录编排状态（单飞：同一时刻至多一条进行中流程）。
type marketDeviceLoginState struct {
	mu     sync.Mutex
	phase  string // idle | pending | done | error
	errMsg string
	user   any // done 时的用户档案（finishMarketBind 返回）
}

// set 置状态（写锁内联小方法，避免散落的锁纪律）。
func (s *marketDeviceLoginState) set(phase, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase, s.errMsg = phase, errMsg
	if phase != "done" {
		s.user = nil
	}
}

// snapshot 读状态快照（零值 phase 归一为 idle：daemon 零初始化即匿名待登录）。
func (s *marketDeviceLoginState) snapshot() (phase, errMsg string, user any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == "" {
		return "idle", "", nil
	}
	return s.phase, s.errMsg, s.user
}

// handleMarketDeviceLogin 发起设备授权登录。受理即返回（pending）；
// 已有流程进行中时幂等返回当前状态，不重复发起。
func (d *Daemon) handleMarketDeviceLogin(_ context.Context, _ json.RawMessage) (any, error) {
	// 单飞闸：进行中直接回当前状态（前端轮询 status 会继续跟踪）
	if phase, _, _ := d.marketDevice.snapshot(); phase == "pending" {
		return map[string]any{"status": "pending"}, nil
	}
	start, err := d.app.Market().DeviceStart()
	if err != nil {
		d.marketDevice.set("error", err.Error())
		return nil, fmt.Errorf("发起设备登录失败：%w", err)
	}
	d.marketDevice.set("pending", "")

	// 复用既有 link_open 广播通道下发确认页 URL：不带 mode（客户端默认走系统
	// 浏览器）；不再发 mode=in_app——支付宝/小米授权页拒绝内嵌 WebView，系统
	// 浏览器最稳且可复用用户既有社区会话。
	d.broadcastUI("link_open", map[string]any{
		"url": start.VerificationURL,
	})
	d.logger.Info("设备授权登录已发起", "url", start.VerificationURL)

	go d.pollMarketDeviceLogin(start.DeviceCode, time.Duration(start.ExpiresIn)*time.Second)
	return map[string]any{"status": "pending", "expires_in": start.ExpiresIn}, nil
}

// pollMarketDeviceLogin 后台轮询：pending 继续直至 authorized/超时；
// authorized 取 AuthToken 走 finishMarketBind 收口；会话过期/网络错误记 error。
func (d *Daemon) pollMarketDeviceLogin(deviceCode string, expiresIn time.Duration) {
	deadline := time.Now().Add(expiresIn)
	for time.Now().Before(deadline) {
		time.Sleep(marketDevicePollInterval)
		res, err := d.app.Market().DevicePoll(deviceCode)
		if err != nil {
			// 单次网络抖动不终止：继续轮询到超时（服务端过期会持续报错）
			d.logger.Warn("设备登录轮询失败", "error", err)
			continue
		}
		switch res.Status {
		case "authorized":
			user, _, bindErr := d.finishMarketBind(res.AuthToken)
			if bindErr != nil {
				d.marketDevice.set("error", bindErr.Error())
				return
			}
			// done 状态由前端轮询 status 感知（不另设完成广播通道）
			d.marketDevice.mu.Lock()
			d.marketDevice.phase, d.marketDevice.errMsg, d.marketDevice.user = "done", "", user
			d.marketDevice.mu.Unlock()
			d.logger.Info("市场设备登录完成", "login", user.Login)
			return
		case "pending":
			// 用户尚未确认，继续轮询
		default:
			d.marketDevice.set("error", "设备登录返回未知状态："+res.Status)
			return
		}
	}
	d.marketDevice.set("error", "设备登录已超时，请重新发起")
}

// handleMarketDeviceStatus 查询设备登录编排状态（前端 1.5s 轮询渲染进度）。
func (d *Daemon) handleMarketDeviceStatus(_ context.Context, _ json.RawMessage) (any, error) {
	phase, errMsg, user := d.marketDevice.snapshot()
	resp := map[string]any{"status": phase, "user": user}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	return resp, nil
}

// handleMarketExperienceList 经验库清单（Agent 经验库全量库，会员身份；q 为关键词）。
// 前端不持有登录令牌：由 daemon 携带内置登录态代理访问市场 API。
func (d *Daemon) handleMarketExperienceList(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Q string `json:"q"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &p) // 参数解析失败按空参处理
	}
	items, err := d.app.Market().ListExperiences(p.Q)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []bundle.MarketExperience{}
	}
	return map[string]any{"experiences": items}, nil
}

// handleMarketExperienceGet 经验文章全文（会员身份）。
func (d *Daemon) handleMarketExperienceGet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID <= 0 {
		return nil, fmt.Errorf("缺少文章编号")
	}
	e, err := d.app.Market().GetExperience(p.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"experience": e}, nil
}
