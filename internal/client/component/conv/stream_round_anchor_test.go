package conv

import (
	"testing"

	"github.com/DotNetAge/goharness/session"
)

// 宿主注入的消息（工具读图附件、任务板提醒）以 user 角色进入历史，但它们不是
// 用户提问：历史还原时不得据此新开一条 Stream，否则 TUI 会凭空多出一轮，
// 并把宿主文案渲染成一张「用户提问卡」。
func TestStreamsFromMessagesSkipHostInjectedUser(t *testing.T) {
	msgs := []session.Message{
		{Role: "user", Content: "帮我看看这张图"},
		{Role: "assistant", ToolCalls: []session.ToolCall{{ID: "c1", Name: "Read"}}},
		{Role: "tool", ToolCallID: "c1", Content: "已读取"},
		{Role: "user", Content: session.ToolVisionPrefix + " 工具 Read 读取到的图像如下", Source: session.SourceToolVision},
		{Role: "user", Content: "【任务板提醒】距上次回写…", Source: session.SourceTaskBoardNudge},
		{Role: "assistant", Content: "图里有三个圆", FinishReason: "stop"},
	}

	streams := StreamsFromMessages("s1", "agent", msgs)
	if len(streams) != 1 {
		t.Fatalf("宿主注入消息不应切出新轮：期望 1 条 Stream，得到 %d", len(streams))
	}
	if len(streams[0].Items) == 0 || streams[0].Items[0].Kind != itemQuestion || streams[0].Items[0].Text != "帮我看看这张图" {
		t.Fatalf("轮次起点应是真实提问，得到 %+v", streams[0].Items)
	}
}

// 对照：真实的多轮提问仍然各自成流。
func TestStreamsFromMessagesKeepRealTurns(t *testing.T) {
	msgs := []session.Message{
		{Role: "user", Content: "第一问"},
		{Role: "assistant", Content: "第一答", FinishReason: "stop"},
		{Role: "user", Content: "第二问"},
		{Role: "assistant", Content: "第二答", FinishReason: "stop"},
	}
	streams := StreamsFromMessages("s1", "agent", msgs)
	if len(streams) != 2 {
		t.Fatalf("真实提问应各自成流：期望 2，得到 %d", len(streams))
	}
}
