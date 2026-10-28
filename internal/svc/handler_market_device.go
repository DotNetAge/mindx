package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// market.device.* RPC —— MindX 账号设备授权登录（device flow，市场生态 §2.4）。
//
// 编排：login 受理即返回——调服务端 start 拿授权页 URL（带 provider 时为直达
// 第三方授权页的直通链路：回调后浏览器自动完成设备授权，免确认页点击），经既有
// link_open 广播通道下发客户端（一律系统浏览器打开：支付宝/小米授权页对内嵌
// WebView 有风控，系统浏览器还能复用用户既有社区会话免重复登录），后台
// goroutine 按 interval 轮询；授权落定后 poll 下发 AuthToken，走
// finishMarketBind 收口（校验归属 + 绑定迁移）。
// status 供前端定时查询渲染进度。全程无短码、无粘贴。
// ---------------------------------------------------------------------------

// marketDevicePollInterval 轮询间隔（服务端 start 响应的 interval 语义：秒）。
const marketDevicePollInterval = 2 * time.Second

// marketDeviceLoginState 设备登录编排状态（单飞：同一时刻至多一条进行中流程）。
// provider 记录进行中流程的登录方式：换 provider 再次发起时作废旧流程而非幂等重开
// （2026-10-20 实锤：用户点 GitHub 后改点 Gitee，单飞闸把旧 pending 当进行中，
// 只会重开 GitHub 链接——「点 A 跳 B」假故障）；gen 为流程代次，旧轮询 goroutine
// 收口/报错前对号，防止晚到的旧流程覆盖新流程状态。
type marketDeviceLoginState struct {
	mu       sync.Mutex
	phase    string // idle | pending | done | error
	errMsg   string
	url      string // pending 时的授权页 URL（重复点「重新打开」时重新广播）
	provider string // pending 流程的登录方式（空 = 官网确认页）
	gen      uint64 // 流程代次（每次发起 +1）
	user     any    // done 时的用户档案（finishMarketBind 返回）
}

// set 置状态（写锁内联小方法，避免散落的锁纪律；仅 pending 阶段携带授权页 URL）。
func (s *marketDeviceLoginState) set(phase, errMsg, authURL, provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase, s.errMsg, s.url, s.provider = phase, errMsg, authURL, provider
	if phase != "done" {
		s.user = nil
	}
}

// snapshot 读状态快照（零值 phase 归一为 idle：daemon 零初始化即匿名待登录）。
func (s *marketDeviceLoginState) snapshot() (phase, errMsg, authURL, provider string, user any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == "" {
		return "idle", "", "", "", nil
	}
	return s.phase, s.errMsg, s.url, s.provider, s.user
}

// stale 判定 myGen 代次的流程是否已被新发起取代（收口/报错前对号，防旧覆盖新）。
func (s *marketDeviceLoginState) stale(myGen uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen != myGen
}

// handleMarketDeviceLogin 发起设备授权登录。受理即返回（pending）；
// 进行中且同一登录方式（或不带 provider 的泛点击）幂等返回当前状态，并把已缓存
// 的授权页 URL 重新广播一次（浏览器窗口关了可再开，不必重新发起流程）；
// 换登录方式则作废旧流程重新发起（旧轮询 goroutine 凭代次自行退场）。
func (d *Daemon) handleMarketDeviceLogin(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Provider string `json:"provider"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &p) // 参数解析失败按无 provider 处理
	}
	// 单飞闸：进行中且未换登录方式 → 重开浏览器 + 回当前状态（前端轮询 status 继续跟踪）
	if phase, _, authURL, pendingProvider, _ := d.marketDevice.snapshot(); phase == "pending" && (p.Provider == "" || p.Provider == pendingProvider) {
		if authURL != "" {
			d.broadcastUI("link_open", map[string]any{"url": authURL})
		}
		return map[string]any{"status": "pending"}, nil
	}
	// 换登录方式发起：作废旧流程（gen+1），旧轮询 goroutine 在收口/报错前对号退场
	d.marketDevice.mu.Lock()
	d.marketDevice.gen++
	myGen := d.marketDevice.gen
	d.marketDevice.mu.Unlock()
	start, err := d.app.Market().DeviceStart(p.Provider)
	if err != nil {
		// 收口失败的排障入口：只回前端不留痕会变成「日志全静默」（2026-10-20 教训）
		d.logger.Error("发起设备登录失败", err)
		d.marketDevice.set("error", err.Error(), "", p.Provider)
		return nil, fmt.Errorf("发起设备登录失败：%w", err)
	}
	// 直达授权 URL 优先（带 provider 的直通链路）；无 provider 时为官网确认页
	authURL := start.VerificationURL
	if start.LoginURL != "" {
		authURL = start.LoginURL
	}
	d.marketDevice.set("pending", "", authURL, p.Provider)

	// 复用既有 link_open 广播通道下发授权页 URL：不带 mode（客户端默认走系统
	// 浏览器）；不再发 mode=in_app——支付宝/小米授权页拒绝内嵌 WebView，系统
	// 浏览器最稳且可复用用户既有社区会话。
	d.broadcastUI("link_open", map[string]any{"url": authURL})
	d.logger.Info("设备授权登录已发起", "provider", p.Provider, "url", authURL)

	go d.pollMarketDeviceLogin(myGen, start.DeviceCode, time.Duration(start.ExpiresIn)*time.Second)
	return map[string]any{"status": "pending", "expires_in": start.ExpiresIn}, nil
}

// pollMarketDeviceLogin 后台轮询：pending 继续直至 authorized/超时；
// authorized 取 AuthToken 走 finishMarketBind 收口；会话过期/网络错误记 error。
// myGen 为发起时的流程代次：被新发起取代后所有状态写入作废（防旧覆盖新）。
func (d *Daemon) pollMarketDeviceLogin(myGen uint64, deviceCode string, expiresIn time.Duration) {
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
			if d.marketDevice.stale(myGen) {
				d.logger.Info("设备登录流程已被新发起取代，丢弃本次收口")
				return
			}
			user, _, bindErr := d.finishMarketBind(res.AuthToken)
			if bindErr != nil {
				// 收口失败的唯一留痕点（前端只显示 error 文案，排障靠这行日志）
				d.logger.Error("市场设备登录收口失败", bindErr)
				if !d.marketDevice.stale(myGen) {
					d.marketDevice.set("error", bindErr.Error(), "", "")
				}
				return
			}
			if d.marketDevice.stale(myGen) {
				d.logger.Info("设备登录流程已被新发起取代，丢弃本次收口")
				return
			}
			// done 状态由前端轮询 status 感知（不另设完成广播通道）
			d.marketDevice.mu.Lock()
			d.marketDevice.phase, d.marketDevice.errMsg, d.marketDevice.url, d.marketDevice.provider, d.marketDevice.user = "done", "", "", "", user
			d.marketDevice.mu.Unlock()
			d.marketLoggedOut = false // 主动登录成功即解除退出标记
			d.logger.Info("市场设备登录完成", "login", user.Login)
			return
		case "pending":
			// 用户尚未确认，继续轮询
		default:
			if !d.marketDevice.stale(myGen) {
				d.marketDevice.set("error", "设备登录返回未知状态："+res.Status, "", "")
			}
			return
		}
	}
	if !d.marketDevice.stale(myGen) {
		d.marketDevice.set("error", "设备登录已超时，请重新发起", "", "")
	}
}

// handleMarketDeviceStatus 查询设备登录编排状态（前端 1.5s 轮询渲染进度）。
func (d *Daemon) handleMarketDeviceStatus(_ context.Context, _ json.RawMessage) (any, error) {
	phase, errMsg, _, _, user := d.marketDevice.snapshot()
	resp := map[string]any{"status": phase, "user": user}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	return resp, nil
}

// handleMarketDeviceProviders 当前部署可用的登录方式清单（匿名透传服务端
// GET /auth/providers；登录页据此渲染第三方登录图标按钮）。
func (d *Daemon) handleMarketDeviceProviders(_ context.Context, _ json.RawMessage) (any, error) {
	providers, err := d.app.Market().LoginProviders()
	if err != nil {
		return nil, err
	}
	if providers == nil {
		providers = []string{}
	}
	return map[string]any{"providers": providers}, nil
}

// handleMarketExperienceMove 挪挂载点 + 兄弟重排（树状挂载写通道；组织树限文章作者
// 本人或组织管理员，公共树会员不可挪——权限与防环由市场服务端裁决）。
// siblings 为挪动完成后目标层级的完整有序文章 ID 列表（含被挪文自身）。
func (d *Daemon) handleMarketExperienceMove(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID       int64   `json:"id"`
		ParentID *int64  `json:"parent_id"`
		Siblings []int64 `json:"siblings"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID <= 0 {
		return nil, fmt.Errorf("缺少文章编号")
	}
	if err := d.app.Market().MoveExperience(p.ID, p.ParentID, p.Siblings); err != nil {
		return nil, err
	}
	return map[string]any{"id": p.ID, "parent_id": p.ParentID}, nil
}
