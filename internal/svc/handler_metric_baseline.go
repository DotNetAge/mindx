package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	bbolt "go.etcd.io/bbolt"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/metric"
)

// 本文件是决策量表的基线与比对（白皮书《决策量表：Agent 生态的绩效系统》
// 第 6 章进化对接）：复盘开工前 compare 验收上轮修订，收尾 save 存基线。
//
// "改善"判据为帕累托标准（白皮书 8.1 第五道防线外的补充）：任一可比因子
// 显著劣化，整体即判非改善——防"降 E 换 Q"式的局部优化被记为改善。
// compare 输出强制附带基线窗口以来的混杂清单（白皮书 2.1 局限声明）：
// 量表劣化触发审查是强承诺，量表改善宣称理论获证是弱声明。

const baselineKeyPrefix = "metric_baseline_"

// metricFactorDirection 各可比指标的提升方向：higher = 越大越好，lower = 越小越好。
var metricFactorDirection = map[string]string{
	"q_task_success":     "higher",
	"q_rework_rate":      "lower",
	"q_route_correction": "lower",
	"s_rounds":           "lower",
	"s_duration":         "lower",
	"e_tokens":           "lower",
	"e_cost":             "lower",
	"p_round_agg":        "higher",
	"p_overall":          "higher",
}

// metricFactorEpsilon 是"显著变化"的相对阈值（5%）：低于此差异视为持平。
const metricFactorEpsilon = 0.05

// ── metric.baseline.save ─────────────────────────────────────────────────

type metricBaselineSaveParams struct {
	Name       string `json:"name"`
	WindowDays int    `json:"window_days,omitempty"`
}

func (d *Daemon) handleMetricBaselineSave(_ context.Context, params json.RawMessage) (any, error) {
	var p metricBaselineSaveParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	windowDays := p.WindowDays
	if windowDays <= 0 {
		windowDays = 7
	}

	until := time.Now()
	since := until.AddDate(0, 0, -windowDays)
	events, err := d.app.MetricStore().Query(context.Background(), metric.MetricFilter{
		Kind: metric.KindRound, Since: since, Until: until,
	})
	if err != nil {
		return nil, err
	}
	report := core.AggregateMetricReport(events, since.Format(time.RFC3339), until.Format(time.RFC3339))

	payload, _ := json.Marshal(map[string]any{
		"name":     p.Name,
		"saved_at": until.Format(time.RFC3339),
		"report":   report,
	})
	if err := d.kvPutRaw(baselineKeyPrefix+p.Name, payload); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": p.Name, "samples": report.Samples}, nil
}

// ── metric.baseline.list ─────────────────────────────────────────────────

func (d *Daemon) handleMetricBaselineList(_ context.Context, _ json.RawMessage) (any, error) {
	names, err := d.kvListByPrefix(baselineKeyPrefix)
	if err != nil {
		return nil, err
	}
	type entry struct {
		Name    string `json:"name"`
		SavedAt string `json:"saved_at"`
	}
	out := make([]entry, 0, len(names))
	for _, key := range names {
		raw, err := d.kvGetRaw(key)
		if err != nil || raw == nil {
			continue
		}
		var payload struct {
			Name    string `json:"name"`
			SavedAt string `json:"saved_at"`
		}
		if json.Unmarshal(raw, &payload) == nil {
			out = append(out, entry{Name: payload.Name, SavedAt: payload.SavedAt})
		}
	}
	return map[string]any{"baselines": out}, nil
}

// ── metric.compare ───────────────────────────────────────────────────────

type metricCompareParams struct {
	Baseline   string `json:"baseline"`
	WindowDays int    `json:"window_days,omitempty"`
}

func (d *Daemon) handleMetricCompare(_ context.Context, params json.RawMessage) (any, error) {
	var p metricCompareParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Baseline == "" {
		return nil, fmt.Errorf("baseline name is required")
	}

	raw, err := d.kvGetRaw(baselineKeyPrefix + p.Baseline)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("baseline %q not found", p.Baseline)
	}
	var payload struct {
		SavedAt string             `json:"saved_at"`
		Report  *core.MetricReport `json:"report"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode baseline: %w", err)
	}
	if payload.Report == nil {
		return nil, fmt.Errorf("baseline %q has no report", p.Baseline)
	}

	windowDays := p.WindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	until := time.Now()
	since := until.AddDate(0, 0, -windowDays)
	events, err := d.app.MetricStore().Query(context.Background(), metric.MetricFilter{
		Kind: metric.KindRound, Since: since, Until: until,
	})
	if err != nil {
		return nil, err
	}
	current := core.AggregateMetricReport(events, since.Format(time.RFC3339), until.Format(time.RFC3339))

	// 混杂清单（强制附带，白皮书 2.1）：基线保存以来的全部登记事件。
	confoundings, err := d.app.MetricStore().QueryConfoundings(context.Background(),
		parseTimeOrZero(payload.SavedAt), until)
	if err != nil {
		return nil, err
	}

	factors := d.compareMetricFactors(payload.Report, current)
	verdict := "flat"
	improved, regressed := 0, 0
	for _, f := range factors {
		switch f.Change {
		case "improved":
			improved++
		case "regressed":
			regressed++
		}
	}
	switch {
	case regressed > 0:
		verdict = "regressed" // 帕累托：任一显著劣化即整体非改善
	case improved > 0:
		verdict = "improved"
	}

	return map[string]any{
		"baseline":       p.Baseline,
		"baseline_at":    payload.SavedAt,
		"window_days":    windowDays,
		"factors":        factors,
		"confoundings":   confoundings,
		"verdict":        verdict, // improved | flat | regressed
		"current_report": current,
	}, nil
}

// metricFactorDelta 是单因子比对结果。
type metricFactorDelta struct {
	Factor    string  `json:"factor"`
	Direction string  `json:"direction"` // higher | lower
	Baseline  float64 `json:"baseline"`
	Current   float64 `json:"current"`
	Change    string  `json:"change"` // improved | regressed | flat | null
	Note      string  `json:"note,omitempty"`
}

// compareMetricFactors 逐因子帕累托对比：基线与当前均为非 null 才比较；
// 任一侧 null 该因子记 null（不可比，不参与判定）。
func (d *Daemon) compareMetricFactors(baseline, current *core.MetricReport) []metricFactorDelta {
	names := []string{
		"q_task_success", "q_rework_rate", "q_route_correction",
		"s_rounds", "s_duration", "e_tokens", "e_cost",
		"p_round_agg", "p_overall",
	}
	out := make([]metricFactorDelta, 0, len(names))
	for _, name := range names {
		base := extractFactorStat(baseline, name)
		cur := extractFactorStat(current, name)
		dir := metricFactorDirection[name]
		delta := metricFactorDelta{Factor: name, Direction: dir, Change: "null"}
		if base == nil || cur == nil || base.Null || cur.Null {
			delta.Note = "样本不足或未覆盖，不可比"
			out = append(out, delta)
			continue
		}
		delta.Baseline = base.Mean
		delta.Current = cur.Mean
		if base.Mean == cur.Mean {
			delta.Change = "flat"
			out = append(out, delta)
			continue
		}
		// 相对变化幅度超过 epsilon 才判显著（绝对变化小于 5% 视为持平）。
		relative := (cur.Mean - base.Mean) / base.Mean
		if relative < 0 {
			relative = -relative
		}
		if relative <= metricFactorEpsilon {
			delta.Change = "flat"
			out = append(out, delta)
			continue
		}
		better := (dir == "higher" && cur.Mean > base.Mean) || (dir == "lower" && cur.Mean < base.Mean)
		if better {
			delta.Change = "improved"
		} else {
			delta.Change = "regressed"
		}
		out = append(out, delta)
	}
	return out
}

// extractFactorStat 按指标名从 report 提取 Stat；未知名返回 nil。
func extractFactorStat(r *core.MetricReport, name string) *core.Stat {
	switch name {
	case "q_task_success":
		return r.Quality.QTaskSuccess
	case "q_rework_rate":
		return r.Quality.QReworkRate
	case "q_route_correction":
		return r.Quality.QRouteCorrection
	case "s_rounds":
		return r.Speed.SRounds
	case "s_duration":
		return r.Speed.SDuration
	case "e_tokens":
		return r.Effort.ETokens
	case "e_cost":
		return r.Effort.ECost
	case "p_round_agg":
		return r.Perceived.PRoundAgg
	case "p_overall":
		return r.Perceived.POverall
	}
	return nil
}

func parseTimeOrZero(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// checkMetricVersionSnapshot daemon 启动时的版本快照对比（混杂登记自动接线）：
// 版本变化 → LogConfounding(daemon_reinstall)。快照存 kv.db。
func (d *Daemon) checkMetricVersionSnapshot(version string) {
	if d.kvStore == nil || version == "" {
		return
	}
	key := "metric_version_snapshot"
	raw, err := d.kvGetRaw(key)
	if err == nil && len(raw) > 0 {
		var prev string
		if json.Unmarshal(raw, &prev) == nil && prev != "" && prev != version {
			_ = d.app.MetricStore().LogConfounding(context.Background(), metric.ConfoundingEvent{
				Kind:   "daemon_reinstall",
				Detail: fmt.Sprintf("daemon: %s -> %s", prev, version),
			})
		}
	}
	_ = d.kvPutRaw(key, []byte(fmt.Sprintf("%q", version)))
}

// ── kv 原始读写辅助（基线快照存 kv.db default bucket，metric_ 前缀命名空间） ──
// 直接操作 bbolt：基线是聚合快照 JSON，不经 kvItem 的 TTL 语义。

func (d *Daemon) kvPutRaw(key string, value []byte) error {
	db := d.kvStore
	if db == nil {
		return fmt.Errorf("kvstore not initialized")
	}
	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), value)
	})
}

func (d *Daemon) kvGetRaw(key string) ([]byte, error) {
	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}
	var out []byte
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		if v := b.Get([]byte(key)); v != nil {
			out = append([]byte(nil), v...)
		}
		return nil
	})
	return out, err
}

func (d *Daemon) kvListByPrefix(prefix string) ([]string, error) {
	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}
	var keys []string
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == prefix; k, _ = c.Next() {
			keys = append(keys, string(k))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}
