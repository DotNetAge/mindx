package svc

import (
	"testing"
	"time"

	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
)

// AskUser 挂起段不是独立轮：暂存后由最终段合并，任务意图仍是本轮最初的提问。
func TestMetricAccumMergesSuspendedSegment(t *testing.T) {
	d := &Daemon{}

	d.stashMetricAccum("s1", "最初的问题", goharnessevents.ExecutionSummaryData{
		TotalIterations: 2,
		TotalDuration:   3 * time.Second,
		TokensUsed:      goharnesssession.TokenUsage{PromptTokens: 100, CompletionTokens: 50, CachedTokens: 40},
	}, 1, []string{"Skill:alpha"})

	final := goharnessevents.ExecutionSummaryData{
		TotalIterations: 1,
		TotalDuration:   2 * time.Second,
		TokensUsed:      goharnesssession.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
	}
	taskHint := "选 A"
	routeCorrection := 0
	skillsUsed := []string{"Skill:beta"}

	a := d.takeMetricAccum("s1")
	if a == nil {
		t.Fatal("suspended segment should be stashed")
	}
	mergeMetricAccum(&final, &taskHint, &routeCorrection, &skillsUsed, a)

	if final.TotalIterations != 3 {
		t.Errorf("iterations should accumulate, got %d", final.TotalIterations)
	}
	if final.TotalDuration != 5*time.Second {
		t.Errorf("duration should accumulate, got %v", final.TotalDuration)
	}
	// 前段净量 = 100+50-40 = 110，最终段净量 = 15，合计 125。
	if got := final.TokensUsed.ActualTokens(); got != 125 {
		t.Errorf("actual tokens should accumulate, got %d", got)
	}
	if taskHint != "最初的问题" {
		t.Errorf("task hint must keep the original question, got %q", taskHint)
	}
	if routeCorrection != 1 {
		t.Errorf("route correction should accumulate, got %d", routeCorrection)
	}
	if len(skillsUsed) != 2 || skillsUsed[0] != "Skill:alpha" || skillsUsed[1] != "Skill:beta" {
		t.Errorf("skill sequence should merge in order, got %v", skillsUsed)
	}
	if d.takeMetricAccum("s1") != nil {
		t.Error("accum should be cleared after take")
	}
}

// AskUser 与工具授权都以「等待用户输入」收尾，二者都必须并入同一轮计量。
func TestSuspendedSummaryReasons(t *testing.T) {
	cases := map[string]bool{
		"ask_user_pending":   true,
		"permission_pending": true,
		"completed":          false,
		"magicword_consumed": false,
		"cancelled":          false,
	}
	for reason, want := range cases {
		if got := isSuspendedSummaryReason(reason); got != want {
			t.Errorf("reason %q: isSuspendedSummaryReason = %v, want %v", reason, got, want)
		}
	}
}

// 真实新提问开始时要清掉上一轮残留的挂起段累计，避免串轮。
func TestMetricAccumClearOnNewRound(t *testing.T) {
	d := &Daemon{}
	d.stashMetricAccum("s1", "旧问题", goharnessevents.ExecutionSummaryData{}, 0, nil)
	d.clearMetricAccum("s1")
	if d.takeMetricAccum("s1") != nil {
		t.Error("new user round must start from a clean accumulator")
	}
}
