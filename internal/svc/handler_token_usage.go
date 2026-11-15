package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

func (d *Daemon) handleTokenUsageDaily(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TokenUsageDailyParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Year == 0 || p.Month < 1 || p.Month > 12 || p.Day < 1 {
		return nil, fmt.Errorf("valid year, month (1-12) and day are required")
	}

	stats, err := d.buildDailyStats(p.Year, p.Month, p.Day)
	if err != nil {
		return nil, fmt.Errorf("build daily stats: %w", err)
	}
	return stats, nil
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
		// Use chargeable tokens (prompt + completion - cached) to match the
		// billing口径 used by monthly stats and ContextUsage.TotalActualTokens.
		totalTokens += chargeableTokens(r)
		if r.ConversationID != "" {
			key := r.SessionID + ":" + r.ConversationID
			convSet[key] = struct{}{}
		}
		mcIn, mcOut, mcCache := d.modelCost(r.ModelName, r.ProviderName)
		totalCost += core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
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
		// Use chargeable tokens (prompt + completion - cached) to match the
		// billing口径 used by monthly stats and ContextUsage.TotalActualTokens.
		totalTokens += chargeableTokens(r)
		mcIn, mcOut, mcCache := d.modelCost(r.ModelName, r.ProviderName)
		totalCost += core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
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

	// 轮次线索：conversationID 在 goharness 每轮 Run 生成且记录已携带；问题文本
	// 经会话用户消息时间对齐补出（每条记录归到其之前最近的用户提问）。
	anchors := d.sessionQuestionAnchors(p.SessionID)

	details := make([]any, 0, len(records))
	for _, r := range records {
		mcIn, mcOut, mcCache := d.modelCost(r.ModelName, r.ProviderName)
		cost := core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
		details = append(details, map[string]any{
			"timestamp":     r.Timestamp,
			"input_tokens":  r.PromptTokens,
			"output_tokens": r.CompletionTokens,
			"cached_tokens": r.CachedTokens,
			// total_tokens 与会话/月度汇总保持一致：计费口径 prompt + completion - cached
			"total_tokens":    chargeableTokens(r),
			"cost":            roundCost(cost),
			"model_name":      r.ModelName,
			"provider_name":   r.ProviderName,
			"conversation_id": r.ConversationID,
			"question":        anchorQuestion(anchors, r.Timestamp),
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

	mcIn, mcOut, mcCache := d.modelCost(p.Model, "")

	totalTokens := 0
	totalInput := 0
	totalOutput := 0
	totalCached := 0
	totalCost := 0.0
	requestCount := len(records)

	for _, r := range records {
		// Use chargeable tokens to keep the by-model endpoint consistent with
		// monthly stats, total/session endpoints, and ContextUsage.TotalActualTokens.
		totalTokens += chargeableTokens(r)
		totalInput += r.PromptTokens
		totalOutput += r.CompletionTokens
		totalCached += r.CachedTokens
		totalCost += core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
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

func (d *Daemon) buildDailyStats(year, month, day int) (map[string]any, error) {
	store := d.app.TokenUsageStore()
	if store == nil {
		return emptyDailyResult(year, month, day), nil
	}

	dayStart := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	// time.Date 会把越界 day 归一化进下月（如 2 月 30 日），回读校验拦掉这类参数
	if dayStart.Day() != day {
		return nil, fmt.Errorf("valid day of %d-%02d is required", year, month)
	}
	until := dayStart.AddDate(0, 0, 1)

	filter := goharnesssession.TokenUsageFilter{
		Since: dayStart,
		Until: until,
	}

	records, err := store.Query(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(records) == 0 {
		return emptyDailyResult(year, month, day), nil
	}

	hourlyMap := make(map[int]*dayAgg)
	modelMap := make(map[string]*modelAgg)
	totalCost := 0.0
	totalTokens := 0

	for _, r := range records {
		hour := r.Timestamp.Hour()
		hData := hourlyMap[hour]
		if hData == nil {
			hData = &dayAgg{}
			hourlyMap[hour] = hData
		}
		hData.inputTokens += r.PromptTokens
		hData.outputTokens += r.CompletionTokens
		hData.cachedTokens += r.CachedTokens
		hData.requestCount++
		hData.model = r.ModelName

		mKey := r.ModelName
		mData := modelMap[mKey]
		if mData == nil {
			mData = &modelAgg{
				model:    r.ModelName,
				provider: r.ProviderName,
			}
			modelMap[mKey] = mData
		}
		mData.inputTokens += r.PromptTokens
		mData.outputTokens += r.CompletionTokens
		mData.cachedTokens += r.CachedTokens
		mData.requestCount++
		mcIn, mcOut, mcCache := d.modelCost(r.ModelName, r.ProviderName)
		cost := core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
		if mData.series == nil {
			mData.series = make(map[string]*modelSeries)
		}
		sKey := fmt.Sprintf("%02d", hour)
		ms := mData.series[sKey]
		if ms == nil {
			ms = &modelSeries{}
			mData.series[sKey] = ms
		}
		ms.requestCount++
		ms.totalTokens += chargeableTokens(r)
		ms.cost += cost

		hData.cost += cost
		mData.totalCost += cost
		totalCost += cost
		totalTokens += chargeableTokens(r)
	}

	hourlyUsage := make([]any, 0, len(hourlyMap))
	for hour, ha := range hourlyMap {
		hourTotal := chargeableTokensOf(ha.inputTokens, ha.outputTokens, ha.cachedTokens)
		hourlyUsage = append(hourlyUsage, map[string]any{
			"hour":          hour,
			"input_tokens":  ha.inputTokens,
			"output_tokens": ha.outputTokens,
			"cached_tokens": ha.cachedTokens,
			"total_tokens":  hourTotal,
			"cost":          roundCost(ha.cost),
			"request_count": ha.requestCount,
			"model":         ha.model,
		})
	}

	modelBreakdown := make([]any, 0, len(modelMap))
	for _, ma := range modelMap {
		modelTotal := chargeableTokensOf(ma.inputTokens, ma.outputTokens, ma.cachedTokens)
		avgReq := 0
		if ma.requestCount > 0 {
			avgReq = modelTotal / ma.requestCount
		}
		hourly := make([]any, 0, len(ma.series))
		for k, ms := range ma.series {
			h, _ := strconv.Atoi(k)
			hourly = append(hourly, map[string]any{
				"hour":          h,
				"request_count": ms.requestCount,
				"total_tokens":  ms.totalTokens,
				"cost":          roundCost(ms.cost),
			})
		}
		modelBreakdown = append(modelBreakdown, map[string]any{
			"model":                  ma.model,
			"provider":               ma.provider,
			"total_tokens":           modelTotal,
			"input_tokens":           ma.inputTokens,
			"output_tokens":          ma.outputTokens,
			"cached_tokens":          ma.cachedTokens,
			"total_cost":             roundCost(ma.totalCost),
			"request_count":          ma.requestCount,
			"avg_tokens_per_request": avgReq,
			"hourly":                 hourly,
		})
	}

	return map[string]any{
		"date":            fmt.Sprintf("%d-%02d-%02d", year, month, day),
		"total_cost":      roundCost(totalCost),
		"total_tokens":    totalTokens,
		"total_requests":  len(records),
		"hourly_usage":    hourlyUsage,
		"model_breakdown": modelBreakdown,
	}, nil
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

	for _, r := range records {
		dateKey := r.Timestamp.Format("2006-01-02")
		dayData := dailyMap[dateKey]
		if dayData == nil {
			dayData = &dayAgg{}
			dailyMap[dateKey] = dayData
		}
		dayData.inputTokens += r.PromptTokens
		dayData.outputTokens += r.CompletionTokens
		dayData.cachedTokens += r.CachedTokens
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
		mData.inputTokens += r.PromptTokens
		mData.outputTokens += r.CompletionTokens
		mData.cachedTokens += r.CachedTokens
		mData.requestCount++
		mcIn, mcOut, mcCache := d.modelCost(r.ModelName, r.ProviderName)
		cost := core.CalculateCost(mcIn, mcOut, mcCache, int64(r.PromptTokens), int64(r.CompletionTokens), int64(r.CachedTokens))
		if mData.series == nil {
			mData.series = make(map[string]*modelSeries)
		}
		ms := mData.series[dateKey]
		if ms == nil {
			ms = &modelSeries{}
			mData.series[dateKey] = ms
		}
		ms.requestCount++
		ms.totalTokens += chargeableTokens(r)
		ms.cost += cost

		dayData.cost += cost
		mData.totalCost += cost
		totalCost += cost
	}

	// 总词元 = 输入 + 输出 - 缓存（chargeable tokens）
	totalTokens := 0
	for _, da := range dailyMap {
		totalTokens += da.inputTokens + da.outputTokens - da.cachedTokens
	}
	if totalTokens < 0 {
		totalTokens = 0
	}

	dailyUsage := make([]any, 0, len(dailyMap))
	for dateStr, da := range dailyMap {
		dayTotal := da.inputTokens + da.outputTokens - da.cachedTokens
		if dayTotal < 0 {
			dayTotal = 0
		}
		dailyUsage = append(dailyUsage, map[string]any{
			"date":          dateStr,
			"input_tokens":  da.inputTokens,
			"output_tokens": da.outputTokens,
			"cached_tokens": da.cachedTokens,
			"total_tokens":  dayTotal,
			"cost":          roundCost(da.cost),
			"request_count": da.requestCount,
			"model":         da.model,
		})
	}

	modelBreakdown := make([]any, 0, len(modelMap))
	for _, ma := range modelMap {
		modelTotal := ma.inputTokens + ma.outputTokens - ma.cachedTokens
		if modelTotal < 0 {
			modelTotal = 0
		}
		avgReq := 0
		if ma.requestCount > 0 {
			avgReq = modelTotal / ma.requestCount
		}
		daily := make([]any, 0, len(ma.series))
		for k, ms := range ma.series {
			daily = append(daily, map[string]any{
				"date":          k,
				"request_count": ms.requestCount,
				"total_tokens":  ms.totalTokens,
				"cost":          roundCost(ms.cost),
			})
		}
		modelBreakdown = append(modelBreakdown, map[string]any{
			"model":                  ma.model,
			"provider":               ma.provider,
			"total_tokens":           modelTotal,
			"input_tokens":           ma.inputTokens,
			"output_tokens":          ma.outputTokens,
			"cached_tokens":          ma.cachedTokens,
			"total_cost":             roundCost(ma.totalCost),
			"request_count":          ma.requestCount,
			"avg_tokens_per_request": avgReq,
			"daily":                  daily,
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

// sessionQuestionAnchors 取会话内全部用户提问（role=user），按时间升序，作为轮次归组锚点。
func (d *Daemon) sessionQuestionAnchors(sessionID string) []goharnesssession.Message {
	sess, err := d.getOrLoadSession(sessionID)
	if err != nil || sess == nil {
		return nil
	}
	msgs := sess.All()
	anchors := make([]goharnesssession.Message, 0, len(msgs)/4)
	for _, m := range msgs {
		if m.Role == "user" {
			anchors = append(anchors, m)
		}
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].Timestamp < anchors[j].Timestamp })
	return anchors
}

// anchorQuestion 返回时间戳之前最近的用户提问（Message.Timestamp 为秒级 Unix）。
// 同轮多次 LLM 调用归到同一提问；首条提问之前的记录返回空串。
func anchorQuestion(anchors []goharnesssession.Message, ts time.Time) string {
	q := ""
	sec := ts.Unix()
	for _, m := range anchors {
		if m.Timestamp > sec {
			break
		}
		if m.Content != "" {
			q = m.Content
		}
	}
	return q
}

func (d *Daemon) listAvailableModels() []string {
	result := make([]string, 0)
	models := d.app.Models()
	if models == nil {
		return result
	}
	for _, m := range models.List() {
		if m != nil {
			result = append(result, m.Name)
		}
	}
	return result
}

// modelCost 根据模型名称与供应商获取输入/输出价格，未找到时返回默认值。
// 传入 provider 走组合键寻址，跨供应商同名模型也能取到对应价格；provider 为空时退化为裸名匹配。
func (d *Daemon) modelCost(modelName, provider string) (per1MIn, per1MOut, per1MInCache float64) {
	model := d.app.Models().Get(modelLookupKey(modelName, provider))
	if model == nil {
		return core.DefaultInputCost, core.DefaultOutputCost, 0
	}
	return model.CostPer1MIn, model.CostPer1MOut, model.CostPer1MInCache
}

type dayAgg struct {
	inputTokens  int
	outputTokens int
	cachedTokens int
	requestCount int
	cost         float64
	model        string
}

type modelAgg struct {
	model        string
	provider     string
	inputTokens  int
	outputTokens int
	cachedTokens int
	requestCount int
	totalCost    float64
	// series 时序桶：day 维度键为 "HH"（小时），month 维度键为 "YYYY-MM-DD"（日期）
	series map[string]*modelSeries
}

// modelSeries 单模型单时序桶的请求/token/费用计数
type modelSeries struct {
	requestCount int
	totalTokens  int
	cost         float64
}

func emptyDailyResult(year, month, day int) map[string]any {
	return map[string]any{
		"date":            fmt.Sprintf("%d-%02d-%02d", year, month, day),
		"total_cost":      0.0,
		"total_tokens":    0,
		"total_requests":  0,
		"hourly_usage":    []any{},
		"model_breakdown": []any{},
	}
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

// chargeableTokensOf 计费口径三数合成：prompt + completion - cached，负值归零。
func chargeableTokensOf(input, output, cached int) int {
	n := input + output - cached
	if n < 0 {
		return 0
	}
	return n
}

// chargeableTokens returns the billable token count for a record:
// prompt + completion - cached. This matches the canonical billing口径 used by
// TokenUsage.ActualTokens() and buildMonthlyStats.
func chargeableTokens(r goharnesssession.TokenUsageRecord) int {
	n := r.PromptTokens + r.CompletionTokens - r.CachedTokens
	if n < 0 {
		return 0
	}
	return n
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
