package svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 市场账号与评分 RPC（市场生态 §2）：
//
//   - market.rate：包级评分（1-5 星，评分挂包不区分版本）；
//   - market.rating.get：查询当前身份对包的评分（modal 回显）；
//   - market.device.login / market.device.status：设备授权登录（device flow，
//     编排逻辑在 handler_market_device.go，绑定收口走本文件的 finishMarketBind）；
//   - market.account.get：账号状态（匿名设备身份 / 已绑定用户信息）。

// handleMarketRate 对市场包提交评分。评分开关（market.shareRatings）是 UI 层门槛，
// daemon 与服务端不做二次校验（计划书 §2.2：开关管用户意愿表达，评分请求本身无害）。
func (d *Daemon) handleMarketRate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketRateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" {
		return nil, fmt.Errorf("kind 与 name 均为必填")
	}
	agg, err := d.app.Market().Rate(p.Kind, p.Name, p.Score)
	if err != nil {
		return nil, err
	}
	d.logger.Info("市场评分提交完成", "kind", p.Kind, "name", p.Name, "score", p.Score)
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
	score, err := d.app.Market().MyRating(p.Kind, p.Name)
	if err == bundle.ErrRatingNotFound {
		return map[string]any{"rated": false, "score": 0}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"rated": true, "score": score}, nil
}

// finishMarketBind 市场登录收口（device flow 轮询拿到 AuthToken 后调用）：
// 先持久化并校验 AuthToken 归属（GET /me，失败回滚旧令牌），通过后执行
// clients/bind 单事务（user_id 落值 + 匿名评分迁移）。幂等、单向。
func (d *Daemon) finishMarketBind(authToken string) (user *bundle.MarketUser, migrated int64, err error) {
	// 先落库再校验：校验失败回滚旧令牌（credStore 与内存同通道恢复，不漂移）
	oldToken := d.app.MarketAuthToken()
	if err := d.app.SetMarketAuthToken(authToken); err != nil {
		return nil, 0, err
	}
	user, err = d.app.Market().Me()
	if err != nil {
		if rollbackErr := d.app.SetMarketAuthToken(oldToken); rollbackErr != nil {
			d.logger.Warn("市场登录令牌回滚失败", "error", rollbackErr)
		}
		return nil, 0, fmt.Errorf("令牌校验失败：%w", err)
	}
	migrated, err = d.app.Market().Bind()
	if err != nil {
		return nil, 0, fmt.Errorf("设备绑定失败：%w", err)
	}
	d.logger.Info("市场账号绑定完成", "login", user.Login, "migrated", migrated)
	return user, migrated, nil
}

// handleMarketUpdateProfile 更新市场账号显示名（§2.5 账户设置页：服务端白名单可编辑字段，
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

// handleMarketUploadAvatar 上传市场账号头像（base64 承载图片二进制；PNG/JPEG ≤1MB，
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

// handleMarketAccountGet 账号状态查询（市场账号 Sheet / 设置账户页的渲染依据）：
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
