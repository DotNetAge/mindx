package svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// MindX 账号与评分 RPC（市场生态 §2）：
//
//   - market.rate：包级评分（1-5 星，评分挂包不区分版本）；
//   - market.rating.get：查询当前身份对包的评分（modal 回显）；
//   - market.device.login / market.device.status：设备授权登录（device flow，
//     编排逻辑在 handler_market_device.go，绑定收口走本文件的 finishMarketBind）；
//   - market.account.get：账号状态（匿名设备身份 / 已绑定用户信息）。

// handleMarketRate 对市场包提交评分。评分数据分享授权（metric.consent）是出网闸门，
// daemon 与服务端不做二次校验（计划书 §2.2：开关管用户意愿表达，评分请求本身无害）。
func (d *Daemon) handleMarketRate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketRateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" {
		return nil, fmt.Errorf("kind 与 name 均为必填")
	}
	// 回路分派：org_slug 非空 = 评组织包（M8；企业内实名口径）；空 = 公共货架包
	var agg *bundle.RatingAggregate
	var err error
	if orgSlug := strings.TrimSpace(p.OrgSlug); orgSlug != "" {
		agg, err = d.app.Market().RateInOrg(orgSlug, p.Kind, p.Name, p.Score)
	} else {
		agg, err = d.app.Market().Rate(p.Kind, p.Name, p.Score)
	}
	if err != nil {
		return nil, err
	}
	d.logger.Info("市场评分提交完成", "kind", p.Kind, "name", p.Name, "score", p.Score, "org", p.OrgSlug)
	return map[string]any{
		"score":        p.Score,
		"rating_avg":   agg.Avg,
		"rating_count": agg.Count,
	}, nil
}

// handleMarketRatingGet 查询当前身份（AuthToken 用户或匿名设备）对包的评分。
// 未评分时返回 rated=false（服务端 404 语义归一，前端直接回显空态）。
func (d *Daemon) handleMarketRatingGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketRatingGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" {
		return nil, fmt.Errorf("kind 与 name 均为必填")
	}
	// 回路分派：org_slug 非空 = 查组织包的我的评分（M8）；空 = 公共货架包
	var score int64
	var err error
	if orgSlug := strings.TrimSpace(p.OrgSlug); orgSlug != "" {
		score, err = d.app.Market().MyRatingInOrg(orgSlug, p.Kind, p.Name)
	} else {
		score, err = d.app.Market().MyRating(p.Kind, p.Name)
	}
	if err == bundle.ErrRatingNotFound {
		return map[string]any{"rated": false, "score": 0}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"rated": true, "score": score}, nil
}

// finishMarketBind 市场登录收口（拿到 AuthToken 后调用）：
// 先持久化并校验 AuthToken 归属（GET /me，失败回滚旧令牌），通过后执行
// clients/bind 单事务（user_id 落值 + 匿名评分迁移 + 设备元数据上报）。
// 幂等、支持改绑（服务端 2026-10-20 定案：换号登录即改绑）。
// 收口成功即写持久化登录史标记（孤儿账号守门的判据，见 autoProvisionMarketAccount）。
func (d *Daemon) finishMarketBind(authToken string) (user *bundle.MarketUser, migrated int64, err error) {
	// 先落库再校验：校验失败回滚旧令牌（credStore 与内存同通道恢复，不漂移）
	oldToken := d.app.MarketAuthToken()
	if err = d.app.SetMarketAuthToken(authToken); err != nil {
		return nil, 0, err
	}
	user, err = d.app.Market().Me()
	if err != nil {
		if rollbackErr := d.app.SetMarketAuthToken(oldToken); rollbackErr != nil {
			d.logger.Warn("市场登录令牌回滚失败", "error", rollbackErr)
		}
		return nil, 0, fmt.Errorf("令牌校验失败：%w", err)
	}
	deviceName, platform := core.MarketDeviceMeta()
	migrated, err = d.app.Market().Bind(deviceName, platform)
	if err != nil {
		return nil, 0, fmt.Errorf("设备绑定失败：%w", err)
	}
	if kvErr := d.kvSetInternal("market_ever_logged_in", "1"); kvErr != nil {
		d.logger.Warn("登录史标记写入失败（不影响本次登录）", "error", kvErr)
	}
	d.logger.Info("MindX 账号绑定完成", "login", user.Login, "migrated", migrated)
	return user, migrated, nil
}

// silentMarketRelogin 启动静默登录（账号机制定案 2026-10-10：新设备零输入即有账号）：
//   - 无凭据 → 自动建号：生成随机凭据注册（称呼=设备名，可后续改），注册即登录并
//     绑定设备（匿名评分迁移），凭据入密钥存储（重装恢复用）；仅注册成功才持久化，
//     失败丢弃下次启动重试；
//   - 有凭据 → 重装恢复路径：凭 username+password 换新 token 并绑定设备（last_seen
//     即刷新）；已有有效令牌时不动作。
//   - 全程静默（网络不可达等失败匿名态继续，只记日志），严禁凭据落日志。
func (d *Daemon) silentMarketRelogin() {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Warn("市场静默登录 panic", "recover", r)
		}
	}()
	// 用户本会话已主动退出：整个静默通道静默（含自动建号），重启后才恢复
	if d.marketLoggedOut {
		d.logger.Info("用户已主动退出，本次运行期不再静默登录")
		return
	}
	if !d.app.HasMarketCredentials() {
		d.autoProvisionMarketAccount()
		return
	}
	// 已有令牌且服务端校验通过 = 在线态，无需重登
	if d.app.MarketAuthToken() != "" {
		if _, err := d.app.Market().Me(); err == nil {
			return
		}
	}
	// Market() 懒创建在此触发（设备身份首启生成也在此收口）
	token, err := d.app.Market().Login(d.app.MarketUsername(), d.app.MarketPassword())
	if err != nil {
		d.logger.Warn("市场静默重登失败（匿名态继续）", "error", err)
		return
	}
	if _, _, err := d.finishMarketBind(token); err != nil {
		d.logger.Warn("市场静默重登绑定失败（匿名态继续）", "error", err)
		return
	}
	d.logger.Info("市场静默重登完成")
}

// autoProvisionMarketAccount 首次自动建号并登录（无凭据分支，见 silentMarketRelogin）。
// 称呼取设备名（截断 32 字符，服务端上限），仅注册成功才持久化凭据。
func (d *Daemon) autoProvisionMarketAccount() {
	// 孤儿账号守门（2026-10-20 定案）：曾登录过的设备不再自动建号。OAuth 账号
	// 退出后本地无凭据，若无此守门，每次「退出 + 重启」都会静默生成一个新账号
	// 并把设备从原账号改绑走（用户今早「注销后变 iMac 账号」的根源）。
	if d.kvGetStringInternal("market_ever_logged_in") == "1" {
		d.logger.Info("本设备曾登录过社区账号，保持匿名等待手动登录")
		return
	}
	username, password, err := core.GenerateMarketCredentials()
	if err != nil {
		d.logger.Warn("市场自动建号凭据生成失败（匿名态继续）", "error", err)
		return
	}
	deviceName, _ := core.MarketDeviceMeta()
	if len(deviceName) > 32 {
		deviceName = deviceName[:32]
	}
	token, err := d.app.Market().Register(username, password, deviceName)
	if err != nil {
		d.logger.Warn("MindX 账号自动创建失败（匿名态继续）", "error", err)
		return
	}
	if credErr := d.app.SetMarketCredentials(username, password); credErr != nil {
		d.logger.Warn("自动账号凭据持久化失败（本次会话内仍可用）", "error", credErr)
	}
	if _, _, err := d.finishMarketBind(token); err != nil {
		d.logger.Warn("市场自动账号绑定失败（匿名态继续）", "error", err)
		return
	}
	d.logger.Info("MindX 账号已自动创建并登录")
}

// handleMarketAccountRegister 用户名密码注册（账号机制定案 2026-10-10）：
// 注册 → 收口绑定；凭据入密钥存储（重装恢复用）。昵称缺省由服务端取用户名。
func (d *Daemon) handleMarketAccountRegister(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketAccountCredentialsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Username == "" || p.Password == "" {
		return nil, fmt.Errorf("username 与 password 均为必填")
	}
	token, err := d.app.Market().Register(p.Username, p.Password, p.DisplayName)
	if err != nil {
		return nil, err
	}
	if credErr := d.app.SetMarketCredentials(p.Username, p.Password); credErr != nil {
		d.logger.Warn("登录凭据持久化失败（本次会话内仍可用）", "error", credErr)
	}
	user, migrated, err := d.finishMarketBind(token)
	if err != nil {
		return nil, err
	}
	d.marketLoggedOut = false // 主动登录成功即解除退出标记
	return map[string]any{"user": user, "migrated": migrated}, nil
}

// handleMarketAccountLogin 用户名密码登录：登录 → 收口绑定；凭据入密钥存储。
func (d *Daemon) handleMarketAccountLogin(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketAccountCredentialsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Username == "" || p.Password == "" {
		return nil, fmt.Errorf("username 与 password 均为必填")
	}
	token, err := d.app.Market().Login(p.Username, p.Password)
	if err != nil {
		return nil, err
	}
	if credErr := d.app.SetMarketCredentials(p.Username, p.Password); credErr != nil {
		d.logger.Warn("登录凭据持久化失败（本次会话内仍可用）", "error", credErr)
	}
	user, migrated, err := d.finishMarketBind(token)
	if err != nil {
		return nil, err
	}
	d.marketLoggedOut = false // 主动登录成功即解除退出标记
	return map[string]any{"user": user, "migrated": migrated}, nil
}

// handleMarketAccountLogout 退出登录：服务端吊销令牌 + 本地清除令牌与凭据。
// 设备归属不变（服务端支持改绑，下次登录凭同一设备身份继续）；本会话内置
// 主动退出标记，静默登录通道（含自动建号）不再动作，重启后恢复自动登录。
func (d *Daemon) handleMarketAccountLogout(_ context.Context, _ json.RawMessage) (any, error) {
	if err := d.app.Market().Logout(); err != nil {
		// 服务端吊销失败不阻断本地登出（令牌留在服务端过期即可，本地清除更安全）
		d.logger.Warn("服务端令牌吊销失败，仅本地登出", "error", err)
	}
	if err := d.app.SetMarketAuthToken(""); err != nil {
		return nil, err
	}
	if err := d.app.SetMarketCredentials("", ""); err != nil {
		return nil, err
	}
	d.marketLoggedOut = true
	d.logger.Info("MindX 账号已退出登录，本次运行期不再自动登录")
	return map[string]any{"logged_out": true}, nil
}

// handleMarketAccountProfileSet Profile 字段编辑（nil = 不更新，前端整表单提交；
// 2026-10-20 定案：github 由 GitHub 身份绑定自动获取，移出手填通道；
// 2026-10-24 定案：email 加入编辑白名单）。
func (d *Daemon) handleMarketAccountProfileSet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketAccountProfileSetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	user, err := d.app.Market().UpdateProfileFields(p.NickName, p.JobTitle, p.Phone, p.Email)
	if err != nil {
		return nil, err
	}
	d.logger.Info("MindX 账号 Profile 更新完成")
	return map[string]any{"user": user}, nil
}

// handleMarketAccountDevices 用户名下设备列表（AccountSheet 设备分组渲染源）。
func (d *Daemon) handleMarketAccountDevices(_ context.Context, _ json.RawMessage) (any, error) {
	devices, err := d.app.Market().Devices()
	if err != nil {
		return nil, err
	}
	current := d.app.MarketClientID()
	out := make([]map[string]any, 0, len(devices))
	for _, dev := range devices {
		out = append(out, map[string]any{
			"client_id":    dev.ClientID,
			"device_name":  dev.DeviceName,
			"platform":     dev.Platform,
			"bound_at":     dev.BoundAt,
			"last_seen_at": dev.LastSeenAt,
			"is_current":   dev.ClientID == current,
		})
	}
	return map[string]any{"devices": out}, nil
}

// handleMarketUpdateProfile 更新MindX 账号显示名（§2.5 账户设置页：服务端白名单可编辑字段，
// ≤32 字符校验在服务端；仅 AuthToken 用户可用）。
func (d *Daemon) handleMarketUpdateProfile(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketUpdateProfileParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.NickName == "" {
		return nil, fmt.Errorf("nick_name 为必填")
	}
	nick, err := d.app.Market().UpdateProfile(p.NickName)
	if err != nil {
		return nil, err
	}
	d.logger.Info("市场显示名更新完成", "nick_name", nick)
	return map[string]any{"nick_name": nick}, nil
}

// handleMarketUploadAvatar 上传MindX 账号头像（base64 承载图片二进制；PNG/JPEG ≤1MB，
// 服务端统一缩放 128×128 并返回档案 URL）。
func (d *Daemon) handleMarketUploadAvatar(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketUploadAvatarParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(p.DataBase64)
	if err != nil {
		return nil, fmt.Errorf("头像数据 base64 非法")
	}
	filename := p.Filename
	if filename == "" {
		filename = "avatar.png"
	}
	avatarURL, err := d.app.Market().UploadAvatar(data, filename)
	if err != nil {
		return nil, err
	}
	d.logger.Info("市场头像上传完成", "avatar_url", avatarURL)
	return map[string]any{"avatar_url": avatarURL}, nil
}

// handleMarketAccountGet 账号状态查询（MindX 账号 Sheet / 设置账户页的渲染依据）：
//   - user 非空 = 已绑定，展示用户档案；
//   - has_token=true 且 user 空 = 令牌失效（token_error 带原因），引导重新录入；
//   - has_token=false = 匿名期，展示设备身份（client_id 短码由前端截取）。
func (d *Daemon) handleMarketAccountGet(_ context.Context, _ json.RawMessage) (any, error) {
	hasToken := d.app.MarketAuthToken() != ""
	resp := map[string]any{
		"has_token": hasToken,
		"client_id": d.app.MarketClientID(),
		"user":      nil,
	}
	if hasToken {
		user, err := d.app.Market().Me()
		if err != nil {
			resp["token_error"] = err.Error()
		} else {
			resp["user"] = user
		}
	}
	return resp, nil
}
