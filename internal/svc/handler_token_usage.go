package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

func (d *Daemon) handleTokenUsageOverview(_ context.Context, params json.RawMessage) (any, error) {
	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())

	currentMonthStats, err := d.buildMonthlyStats(currentYear, currentMonth)
	if err != nil {
		d.logger.Warn("failed to build current month stats", "error", err)
	}

	var prevMonthStats map[string]any
	prevYear, prevMonth := currentYear, currentMonth-1
	if prevMonth == 0 {
		prevYear--
		prevMonth = 12
	}
	pmStats, err := d.buildMonthlyStats(prevYear, prevMonth)
	if err == nil {
		prevMonthStats = pmStats
	}

	models := d.listAvailableModels()

	return map[string]any{
		"current_month":    currentMonthStats,
		"previous_month":   prevMonthStats,
		"available_models": models,
	}, nil
}

func (d *Daemon) handleTokenUsageMonthly(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TokenUsageMonthlyParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Year == 0 || p.Month == 0 || p.Month < 1 || p.Month > 12 {
		return nil, fmt.Errorf("valid year and month (1-12) are required")
	}

	stats, err := d.buildMonthlyStats(p.Year, p.Month)
	if err != nil {
		return nil, fmt.Errorf("build monthly stats: %w", err)
	}
	return stats, nil
}

func (d *Daemon) handleTokenUsageTotal(_ context.Context, _ json.RawMessage) (any, error) {
	store := d.app.TokenUsageStore()
	if store == nil {
		return map[string]any{
			"total_tokens":        0,
			"total_cost":          0.0,
			"total_conversations": 0,
		}, nil
	}

	// Query all records (no time/session filter)
	records, err := store.Query(context.Background(), goharnesssession.TokenUsageFilter{})
	if err != nil {
		return nil, fmt.Errorf("query all token usage: %w", err)
	}

	totalTokens := 0
	totalCost := 0.0
	convSet := make(map[string]struct{})

	for _, r := range records {
		totalTokens += r.TotalTokens
		if r.ConversationID != "" {
			key := r.SessionID + ":" + r.ConversationID
			convSet[key] = struct{}{}
		}
		mc, hasMC := d.app.Costs().Get(r.ModelName)
		if hasMC {
			totalCost += calculateRecordCost(mc, r)
		}
	}

	return map[string]any{
		"total_tokens":        totalTokens,
		"total_cost":          roundCost(totalCost),
		"total_conversations": len(convSet),
	}, nil
}

func (d *Daemon) handleTokenUsageSession(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TokenUsageSessionParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	store := d.app.TokenUsageStore()
	if store == nil {
		return map[string]any{
			"tokens_used": 0,
			"cost":        0.0,
		}, nil
	}

	filter := goharnesssession.TokenUsageFilter{
		SessionID: p.SessionID,
	}
	records, err := store.Query(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("query session token usage: %w", err)
	}

	totalTokens := 0
	totalCost := 0.0
	for _, r := range records {
		totalTokens += r.TotalTokens
		mc, hasMC := d.app.Costs().Get(r.ModelName)
		if hasMC {
			totalCost += calculateRecordCost(mc, r)
		}
	}

	return map[string]any{
		"tokens_used": totalTokens,
		"cost":        roundCost(totalCost),
	}, nil
}

func (d *Daemon) handleTokenUsageSessionDetail(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TokenUsageSessionDetailParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	store := d.app.TokenUsageStore()
	if store == nil {
		return []any{}, nil
	}

	filter := goharnesssession.TokenUsageFilter{
		SessionID: p.SessionID,
	}
	records, err := store.Query(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("query session token usage: %w", err)
	}

	details := make([]any, 0, len(records))
	for _, r := range records {
		mc, hasMC := d.app.Costs().Get(r.ModelName)
		cost := 0.0
		if hasMC {
			cost = calculateRecordCost(mc, r)
		}
		details = append(details, map[string]any{
			"timestamp":     r.Timestamp,
			"input_tokens":  r.PromptTokens,
			"output_tokens": r.CompletionTokens,
			"cached_tokens": r.CachedTokens,
			"total_tokens":  r.TotalTokens,
			"cost":          roundCost(cost),
			"model_name":    r.ModelName,
			"provider_name": r.ProviderName,
		})
	}

	return map[string]any{
		"session_id": p.SessionID,
		"records":    details,
	}, nil
}

func (d *Daemon) handleTokenUsageByModel(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TokenUsageByModelParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	now := time.Now()
	year := p.Year
	if year == 0 {
		year = now.Year()
	}
	month := p.Month
	if month == 0 {
		month = int(now.Month())
	}

	store := d.app.TokenUsageStore()
	if store == nil {
		return []any{}, nil
	}

	since := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	until := since.AddDate(0, 1, 0)

	filter := goharnesssession.TokenUsageFilter{
		ModelName: p.Model,
		Since:     since,
		Until:     until,
	}

	records, err := store.Query(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("query token usage: %w", err)
	}

	modelCost, hasCost := d.app.Costs().Get(p.Model)

	totalTokens := 0
	totalInput := 0
	totalOutput := 0
	totalCached := 0
	totalCost := 0.0
	requestCount := len(records)

	for _, r := range records {
		totalTokens += r.TotalTokens
		totalInput += r.PromptTokens
		totalOutput += r.CompletionTokens
		totalCached += r.CachedTokens
		if hasCost {
			totalCost += calculateRecordCost(modelCost, r)
		}
	}

	avgPerRequest := 0
	if requestCount > 0 {
		avgPerRequest = totalTokens / requestCount
	}

	return []any{map[string]any{
		"model":                  p.Model,
		"provider":               resolveProvider(records),
		"total_tokens":           totalTokens,
		"input_tokens":           totalInput,
		"output_tokens":          totalOutput,
		"total_cost":             roundCost(totalCost),
		"request_count":          requestCount,
		"avg_tokens_per_request": avgPerRequest,
	}}, nil
}

func (d *Daemon) buildMonthlyStats(year, month int) (map[string]any, error) {
	store := d.app.TokenUsageStore()
	d.logger.Debug("buildMonthlyStats called",
		"year", year, "month", month,
		"store_is_nil", store == nil,
	)
	if store == nil {
		d.logger.Warn("TokenUsageStore is nil", "year", year, "month", month)
		return emptyMonthlyResult(year, month), nil
	}

	since := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	until := since.AddDate(0, 1, 0)

	filter := goharnesssession.TokenUsageFilter{
		Since: since,
		Until: until,
	}

	records, err := store.Query(context.Background(), filter)
	d.logger.Debug("query result",
		"record_count", len(records),
		"query_err", err,
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(records) == 0 {
		return emptyMonthlyResult(year, month), nil
	}

	dailyMap := make(map[string]*dayAgg)
	modelMap := make(map[string]*modelAgg)
	totalCost := 0.0
	totalTokens := 0

	for _, r := range records {
		dateKey := r.Timestamp.Format("2006-01-02")
		dayData := dailyMap[dateKey]
		if dayData == nil {
			dayData = &dayAgg{}
			dailyMap[dateKey] = dayData
		}
		dayData.inputTokens += r.PromptTokens
		dayData.outputTokens += r.CompletionTokens
		dayData.totalTokens += r.TotalTokens
		dayData.requestCount++
		dayData.model = r.ModelName

		mKey := r.ModelName
		mData := modelMap[mKey]
		if mData == nil {
			mData = &modelAgg{
				model:    r.ModelName,
				provider: r.ProviderName,
			}
			modelMap[mKey] = mData
		}
		mData.totalTokens += r.TotalTokens
		mData.inputTokens += r.PromptTokens
		mData.outputTokens += r.CompletionTokens
		mData.requestCount++

		mc, hasMC := d.app.Costs().Get(r.ModelName)
		if hasMC {
			cost := calculateRecordCost(mc, r)
			dayData.cost += cost
			mData.totalCost += cost
			totalCost += cost
		}

		totalTokens += r.TotalTokens
	}

	dailyUsage := make([]any, 0, len(dailyMap))
	for dateStr, da := range dailyMap {
		dailyUsage = append(dailyUsage, map[string]any{
			"date":          dateStr,
			"input_tokens":  da.inputTokens,
			"output_tokens": da.outputTokens,
			"total_tokens":  da.totalTokens,
			"cost":          roundCost(da.cost),
			"request_count": da.requestCount,
			"model":         da.model,
		})
	}

	modelBreakdown := make([]any, 0, len(modelMap))
	for _, ma := range modelMap {
		avgReq := 0
		if ma.requestCount > 0 {
			avgReq = ma.totalTokens / ma.requestCount
		}
		modelBreakdown = append(modelBreakdown, map[string]any{
			"model":                  ma.model,
			"provider":               ma.provider,
			"total_tokens":           ma.totalTokens,
			"input_tokens":           ma.inputTokens,
			"output_tokens":          ma.outputTokens,
			"total_cost":             roundCost(ma.totalCost),
			"request_count":          ma.requestCount,
			"avg_tokens_per_request": avgReq,
		})
	}

	return map[string]any{
		"year":            year,
		"month":           month,
		"total_cost":      roundCost(totalCost),
		"total_tokens":    totalTokens,
		"total_requests":  len(records),
		"daily_usage":     dailyUsage,
		"model_breakdown": modelBreakdown,
	}, nil
}

func (d *Daemon) listAvailableModels() []string {
	costs := d.app.Costs()
	list := costs.List()
	result := make([]string, 0, len(list))
	for _, nc := range list {
		result = append(result, nc.Name)
	}
	return result
}

type dayAgg struct {
	inputTokens  int
	outputTokens int
	totalTokens  int
	requestCount int
	cost         float64
	model        string
}

type modelAgg struct {
	model        string
	provider     string
	totalTokens  int
	inputTokens  int
	outputTokens int
	requestCount int
	totalCost    float64
}

func emptyMonthlyResult(year, month int) map[string]any {
	return map[string]any{
		"year":            year,
		"month":           month,
		"total_cost":      0.0,
		"total_tokens":    0,
		"total_requests":  0,
		"daily_usage":     []any{},
		"model_breakdown": []any{},
	}
}

func calculateRecordCost(mc core.ModelCost, r goharnesssession.TokenUsageRecord) float64 {
	cost := 0.0

	// Input tokens: cached portion is excluded (already paid in a prior call)
	chargeableInput := r.PromptTokens - r.CachedTokens
	if chargeableInput < 0 {
		chargeableInput = 0
	}
	if mc.CostPer1MIn > 0 {
		cost += mc.CostPer1MIn / 1_000_000 * float64(chargeableInput)
	}

	// Output tokens
	if mc.CostPer1MOut > 0 {
		cost += mc.CostPer1MOut / 1_000_000 * float64(r.CompletionTokens)
	}

	return cost
}

func roundCost(v float64) float64 {
	return float64(int(v*10000)) / 10000
}

func resolveProvider(records []goharnesssession.TokenUsageRecord) string {
	for _, r := range records {
		if r.ProviderName != "" {
			return r.ProviderName
		}
	}
	return ""
}
