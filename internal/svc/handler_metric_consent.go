package svc

import (
	"context"
	"encoding/json"
	"time"
)

// 本文件是决策量表的出网授权链（白皮书《决策量表：Agent 生态的绩效系统》
// 第 8 章出网边界）：metric.consent.get/set 管理用户授权状态。
//
// 授权语义：granted=true 表示用户同意"聚合评分（仅计数与占比）"出网上传
// 市场；文本与轮次明细无论授权与否一律不出本地（8.4 双重校验的客户端侧）。
// 撤回即 set granted=false——状态幂等覆盖，市场侧以下次上报为准。
// 存储复用 kv.db 的 metric_ 前缀命名空间（kvPutRaw/kvGetRaw）。

// metricConsentKey 授权状态在 kv.db default bucket 的键。
const metricConsentKey = "metric_consent"

// metricConsentState 授权状态：granted + 最近一次变更时间（审计用；
// 从未设置时 UpdatedAt 为 nil，序列化省略）。
type metricConsentState struct {
	Granted   bool       `json:"granted"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func (d *Daemon) handleMetricConsentGet(_ context.Context, _ json.RawMessage) (any, error) {
	state := metricConsentState{} // 从未设置 = 未授权（默认拒绝）
	if raw, err := d.kvGetRaw(metricConsentKey); err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &state)
	}
	return state, nil
}

func (d *Daemon) handleMetricConsentSet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Granted bool `json:"granted"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	state := metricConsentState{Granted: p.Granted, UpdatedAt: nowPtr()}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if err := d.kvPutRaw(metricConsentKey, raw); err != nil {
		return nil, err
	}
	return state, nil
}

// nowPtr 返回当前时间的指针（UpdatedAt 需要区分"从未设置"与零值）。
func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
