package svc

import (
	"context"
	"testing"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// appendUsage 注入一条使用记录（默认：今天、未知模型走默认价）。
func appendUsage(t *testing.T, d *Daemon, rec goharnesssession.TokenUsageRecord) {
	t.Helper()
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	if rec.ID == "" {
		rec.ID = "rec-" + rec.SessionID + "-" + rec.ConversationID + "-" + time.Now().Format("150405.000000000")
	}
	if err := d.app.TokenUsageStore().Append(context.Background(), rec); err != nil {
		t.Fatalf("Append usage record: %v", err)
	}
}

// TestChargeableTokensBillingGuards 计费口径守卫：prompt+completion-cached，
// 负值归零（对齐 ContextUsage.TotalActualTokens 口径）。
func TestChargeableTokensBillingGuards(t *testing.T) {
	r := goharnesssession.TokenUsageRecord{PromptTokens: 100, CompletionTokens: 50, CachedTokens: 30}
	if got := chargeableTokens(r); got != 120 {
		t.Errorf("chargeableTokens = %d, want 120", got)
	}
	r.CachedTokens = 500
	if got := chargeableTokens(r); got != 0 {
		t.Errorf("缓存超量应归零, got %d", got)
	}
	if got := chargeableTokensOf(10, 5, 100); got != 0 {
		t.Errorf("chargeableTokensOf 负值应归零, got %d", got)
	}
	if got := roundCost(1.23456789); got != 1.2345 {
		t.Errorf("roundCost 截断四位小数, got %v", got)
	}
}

// TestTokenUsageTotal_UnknownModelDefaultCost 汇总全量：chargeable 口径 token、
// 去重会话轮次、未知模型默认单价（输入 ¥3/输出 ¥15 每百万）。
func TestTokenUsageTotal_UnknownModelDefaultCost(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ConversationID: "c1",
		PromptTokens: 1_000_000, CompletionTokens: 0, // 成本 3.0，chargeable 1M
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ConversationID: "c2",
		PromptTokens: 2_000_000, CompletionTokens: 1_000_000, CachedTokens: 500_000,
		// 成本 = 3*(2M-0.5M)/1M + 15*1M/1M = 19.5；chargeable 2.5M
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s2", ConversationID: "c1",
		PromptTokens: 0, CompletionTokens: 0, // 空记录不进费用，但 s2:c1 计入轮次
	})

	out, err := d.handleTokenUsageTotal(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleTokenUsageTotal: %v", err)
	}
	m := out.(map[string]any)
	if m["total_tokens"].(int) != 3_500_000 {
		t.Errorf("total_tokens = %v, want 3500000", m["total_tokens"])
	}
	if m["total_conversations"].(int) != 3 {
		t.Errorf("total_conversations = %v, want 3", m["total_conversations"])
	}
	if m["total_cost"].(float64) != 22.5 {
		t.Errorf("total_cost = %v, want 22.5", m["total_cost"])
	}
}

// TestTokenUsageSession_FilterAndValidation 会话维度：按 session_id 过滤、缺参报错。
func TestTokenUsageSession_FilterAndValidation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleTokenUsageSession(context.Background(), []byte(`{}`)); err == nil {
		t.Error("缺 session_id 应报错")
	}

	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 1_000_000,
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "other", PromptTokens: 5_000_000, CompletionTokens: 1_000_000,
	})

	out, err := d.handleTokenUsageSession(context.Background(), []byte(`{"session_id":"s1"}`))
	if err != nil {
		t.Fatalf("handleTokenUsageSession: %v", err)
	}
	m := out.(map[string]any)
	if m["tokens_used"].(int) != 1_000_000 {
		t.Errorf("tokens_used = %v, want 1000000（他_session 记录不得混入）", m["tokens_used"])
	}
	if m["cost"].(float64) != 3.0 {
		t.Errorf("cost = %v, want 3.0", m["cost"])
	}
}

// TestTokenUsageSessionDetail_QuestionAnchors 轮次明细：每条记录归到其之前
// 最近的用户提问；首条提问之前的记录问题为空。
func TestTokenUsageSessionDetail_QuestionAnchors(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleTokenUsageSessionDetail(context.Background(), []byte(`{}`)); err == nil {
		t.Error("缺 session_id 应报错")
	}

	sid := mustCreateSession(t, d.app.SessDB(), "assistant")
	sess := loadTestSession(t, d, sid, "assistant")
	now := time.Now().Unix()
	// 两轮提问：t-120 与 t-60（Timestamp 按秒级 Unix，见 anchorQuestion 契约）
	if err := sess.Append(context.Background(), goharnesssession.Message{
		Role: "user", Content: "第一问", Timestamp: now - 120,
	}); err != nil {
		t.Fatalf("append msg: %v", err)
	}
	if err := sess.Append(context.Background(), goharnesssession.Message{
		Role: "user", Content: "第二问", Timestamp: now - 60,
	}); err != nil {
		t.Fatalf("append msg: %v", err)
	}

	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: sid, PromptTokens: 10, Timestamp: time.Unix(now-100, 0), // 归第一问
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: sid, PromptTokens: 20, Timestamp: time.Unix(now-60, 0), // 与第二问同秒，归第二问
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: sid, PromptTokens: 30, Timestamp: time.Unix(now-200, 0), // 首问之前
	})

	out, err := d.handleTokenUsageSessionDetail(context.Background(), []byte(`{"session_id":"`+sid+`"}`))
	if err != nil {
		t.Fatalf("handleTokenUsageSessionDetail: %v", err)
	}
	records := out.(map[string]any)["records"].([]any)
	if len(records) != 3 {
		t.Fatalf("应有 3 条明细, got %d", len(records))
	}
	wantQ := map[int]string{10: "第一问", 20: "第二问", 30: ""}
	for _, r := range records {
		m := r.(map[string]any)
		got := m["question"].(string)
		in := m["input_tokens"].(int)
		if got != wantQ[in] {
			t.Errorf("input=%d 的轮次问题 = %q, want %q", in, got, wantQ[in])
		}
	}
}

// TestTokenUsageDaily_ValidationAndBuckets 日粒度：参数校验、2月30日越界拒绝、
// 小时桶与模型分解聚合。
func TestTokenUsageDaily_ValidationAndBuckets(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	bad := []rpc.TokenUsageDailyParams{
		{Year: 0, Month: 11, Day: 1},
		{Year: 2026, Month: 13, Day: 1},
		{Year: 2026, Month: 0, Day: 1},
		{Year: 2026, Month: 11, Day: 0},
	}
	for _, p := range bad {
		if _, err := d.handleTokenUsageDaily(context.Background(), mustJSON(t, p)); err == nil {
			t.Errorf("非法参数 %+v 应报错", p)
		}
	}
	// 2月30日：time.Date 归一化进 3 月，必须被回读校验拦下
	if _, err := d.handleTokenUsageDaily(context.Background(), mustJSON(t, rpc.TokenUsageDailyParams{Year: 2026, Month: 2, Day: 30})); err == nil {
		t.Error("2月30日应报错而非静默归到 3 月")
	}

	now := time.Now()
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 500_000, CompletionTokens: 500_000, CachedTokens: 100_000,
		Timestamp: time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local),
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s2", PromptTokens: 200_000,
		Timestamp: time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, time.Local),
	})

	out, err := d.handleTokenUsageDaily(context.Background(), mustJSON(t, rpc.TokenUsageDailyParams{Year: now.Year(), Month: int(now.Month()), Day: now.Day()}))
	if err != nil {
		t.Fatalf("handleTokenUsageDaily: %v", err)
	}
	m := out.(map[string]any)
	if m["total_requests"].(int) != 2 {
		t.Errorf("total_requests = %v, want 2", m["total_requests"])
	}
	if m["total_tokens"].(int) != 1_100_000 {
		t.Errorf("total_tokens = %v, want 1100000", m["total_tokens"])
	}
	if len(m["hourly_usage"].([]any)) != 2 {
		t.Errorf("应聚合出 9/15 两个小时桶: %v", m["hourly_usage"])
	}
	breakdown := m["model_breakdown"].([]any)
	if len(breakdown) != 1 {
		t.Fatalf("未知模型应归入默认桶, got %d", len(breakdown))
	}
	bm := breakdown[0].(map[string]any)
	if bm["request_count"].(int) != 2 {
		t.Errorf("模型桶 request_count = %v, want 2", bm["request_count"])
	}
	if len(bm["hourly"].([]any)) != 2 {
		t.Errorf("模型桶应含两条小时序列: %v", bm["hourly"])
	}
}

// TestTokenUsageMonthly_ValidationAndAggregation 月粒度：参数校验、跨日聚合、
// chargeable 口径防负。
func TestTokenUsageMonthly_ValidationAndAggregation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleTokenUsageMonthly(context.Background(), []byte(`{"month":13}`)); err == nil {
		t.Error("月份 13 应报错")
	}
	if _, err := d.handleTokenUsageMonthly(context.Background(), []byte(`{}`)); err == nil {
		t.Error("缺年月应报错")
	}

	now := time.Now()
	day := func(offset int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+offset, 12, 0, 0, 0, time.Local)
	}
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 1_000_000, Timestamp: day(0),
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 2_000_000, CompletionTokens: 1_000_000, Timestamp: day(-1),
	})
	// 上一月记录不得混入本月
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 9_000_000, Timestamp: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -1, 0),
	})

	out, err := d.handleTokenUsageMonthly(context.Background(), mustJSON(t, rpc.TokenUsageMonthlyParams{Year: now.Year(), Month: int(now.Month())}))
	if err != nil {
		t.Fatalf("handleTokenUsageMonthly: %v", err)
	}
	m := out.(map[string]any)
	if m["total_requests"].(int) != 2 {
		t.Errorf("total_requests = %v, want 2", m["total_requests"])
	}
	if m["total_tokens"].(int) != 4_000_000 {
		t.Errorf("total_tokens = %v, want 4000000", m["total_tokens"])
	}
	if len(m["daily_usage"].([]any)) != 2 {
		t.Errorf("应聚合出两个日桶: %v", m["daily_usage"])
	}
	if len(m["model_breakdown"].([]any)) != 1 {
		t.Errorf("模型分解应归一桶: %v", m["model_breakdown"])
	}
}

// TestTokenUsageByModel_FilterAndAvg 模型维度：缺参报错、按月窗口过滤、均值。
func TestTokenUsageByModel_FilterAndAvg(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleTokenUsageByModel(context.Background(), []byte(`{}`)); err == nil {
		t.Error("缺 model 应报错")
	}

	now := time.Now()
	thisMonth := time.Date(now.Year(), now.Month(), 10, 0, 0, 0, 0, time.Local)
	nextMonth := thisMonth.AddDate(0, 1, 0)

	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ModelName: "m-a", ProviderName: "p-x", PromptTokens: 300_000, Timestamp: thisMonth,
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ModelName: "m-a", ProviderName: "p-x", PromptTokens: 500_000, Timestamp: thisMonth,
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ModelName: "m-b", PromptTokens: 1_000_000, Timestamp: thisMonth,
	})
	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", ModelName: "m-a", PromptTokens: 9_000_000, Timestamp: nextMonth, // 下月窗口外
	})

	out, err := d.handleTokenUsageByModel(context.Background(), mustJSON(t, rpc.TokenUsageByModelParams{Model: "m-a"}))
	if err != nil {
		t.Fatalf("handleTokenUsageByModel: %v", err)
	}
	list := out.([]any)
	if len(list) != 1 {
		t.Fatalf("应只返回 m-a 一个桶, got %d", len(list))
	}
	m := list[0].(map[string]any)
	if m["request_count"].(int) != 2 {
		t.Errorf("request_count = %v, want 2", m["request_count"])
	}
	if m["total_tokens"].(int) != 800_000 {
		t.Errorf("total_tokens = %v, want 800000", m["total_tokens"])
	}
	if m["avg_tokens_per_request"].(int) != 400_000 {
		t.Errorf("avg_tokens_per_request = %v, want 400000", m["avg_tokens_per_request"])
	}
	if m["provider"].(string) != "p-x" {
		t.Errorf("provider = %v, want p-x", m["provider"])
	}
}

// TestTokenUsageOverview_ShapeAndEmptyMonth 总览：返回当月/上月两块与非空模型清单来源。
func TestTokenUsageOverview_ShapeAndEmptyMonth(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	appendUsage(t, d, goharnesssession.TokenUsageRecord{
		SessionID: "s1", PromptTokens: 100_000,
	})

	out, err := d.handleTokenUsageOverview(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleTokenUsageOverview: %v", err)
	}
	m := out.(map[string]any)
	cur := m["current_month"].(map[string]any)
	if cur["total_requests"].(int) != 1 {
		t.Errorf("当月应有 1 条请求: %v", cur)
	}
	prev := m["previous_month"].(map[string]any)
	if prev["total_requests"].(int) != 0 {
		t.Errorf("上月应无请求: %v", prev)
	}
	if m["available_models"] == nil {
		t.Error("available_models 不应为 nil")
	}
}

// TestTokenUsageModelCost_FallbackAndLookup 价格寻址：未知模型回退默认价，
// 注册模型按组合键取价。
func TestTokenUsageModelCost_FallbackAndLookup(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	in, out, cache := d.modelCost("no-such-model", "")
	if in != core.DefaultInputCost || out != core.DefaultOutputCost || cache != 0 {
		t.Errorf("未知模型应回退默认价, got %v/%v/%v", in, out, cache)
	}
}
