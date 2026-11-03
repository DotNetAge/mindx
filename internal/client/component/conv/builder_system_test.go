package conv

import (
	"testing"
	"time"
)

// TestBuildSystemNodeDispatch NoticeSub 细类精确分派：各细类落对节点类型与
// 关键字段（max_turns 计数透传、超时→错误卡、子任务失败态）；无细类条目走
// 文本启发兜底，未命中按原文直渲 content，不再误标 max_turns。
func TestBuildSystemNodeDispatch(t *testing.T) {
	offset := time.Second
	build := func(it Item) TreeNode {
		return buildSystemNode(&BuildState{}, it, offset)
	}

	// max_turns：轮次计数透传（避免 0/0 轮）+ 建议文案去前缀
	n := build(Item{Kind: itemNotice, Text: "💡 请发送\"继续\"", NoticeSub: "max_turns",
		TurnsCompleted: 12, MaxTurns: 20})
	mt, ok := n.(*MaxTurnsNode)
	if !ok {
		t.Fatalf("max_turns: got %T, want *MaxTurnsNode", n)
	}
	if mt.TurnsCompleted != 12 || mt.MaxTurns != 20 {
		t.Errorf("max_turns counts = %d/%d, want 12/20", mt.TurnsCompleted, mt.MaxTurns)
	}
	if mt.Suggestion != "请发送\"继续\"" {
		t.Errorf("max_turns suggestion = %q", mt.Suggestion)
	}

	// timeout：错误卡（llm_timeout）
	n = build(Item{Kind: itemNotice, Text: "请求超时：60s", NoticeSub: "timeout"})
	en, ok := n.(*ErrorNode)
	if !ok {
		t.Fatalf("timeout: got %T, want *ErrorNode", n)
	}
	if en.Source != "llm_timeout" || en.Message != "请求超时：60s" {
		t.Errorf("timeout node = source %q message %q", en.Source, en.Message)
	}

	// cancelled
	n = build(Item{Kind: itemNotice, Text: "已中断", NoticeSub: "cancelled"})
	if _, ok := n.(*CancelledNode); !ok {
		t.Fatalf("cancelled: got %T, want *CancelledNode", n)
	}

	// subtask_spawn：成功态 content 直渲
	n = build(Item{Kind: itemNotice, Text: "🚀 worker · 开始", NoticeSub: "subtask_spawn"})
	cn, ok := n.(*ContentNode)
	if !ok {
		t.Fatalf("subtask_spawn: got %T, want *ContentNode", n)
	}
	if cn.Status != NodeSuccess || cn.Text != "🚀 worker · 开始" {
		t.Errorf("subtask_spawn node = status %v text %q", cn.Status, cn.Text)
	}

	// subtask_fail：失败态落在节点 Status 上
	n = build(Item{Kind: itemNotice, Text: "❌ worker · boom", NoticeSub: "subtask_fail"})
	cn, ok = n.(*ContentNode)
	if !ok {
		t.Fatalf("subtask_fail: got %T, want *ContentNode", n)
	}
	if cn.Status != NodeFailed || cn.Text != "❌ worker · boom" {
		t.Errorf("subtask_fail node = status %v text %q", cn.Status, cn.Text)
	}

	// 文本启发兜底（历史还原的无细类条目）
	n = build(Item{Kind: itemNotice, Text: "max_turns reached"})
	if _, ok := n.(*MaxTurnsNode); !ok {
		t.Fatalf("legacy max_turns text: got %T, want *MaxTurnsNode", n)
	}
	n = build(Item{Kind: itemNotice, Text: "任务已中断"})
	if _, ok := n.(*CancelledNode); !ok {
		t.Fatalf("legacy cancelled text: got %T, want *CancelledNode", n)
	}
	n = build(Item{Kind: itemNotice, Text: "普通系统通知"})
	cn, ok = n.(*ContentNode)
	if !ok {
		t.Fatalf("legacy unknown text: got %T, want *ContentNode", n)
	}
	if cn.Text != "普通系统通知" {
		t.Errorf("legacy unknown text = %q", cn.Text)
	}
}
