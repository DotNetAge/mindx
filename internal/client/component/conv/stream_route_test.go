package conv

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	goharnesssession "github.com/DotNetAge/goharness/session"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

// AskUser 作答是当前轮的控制应答：必须追加到同一条流，不能切出新轮，
// 否则轮次显示与用量统计都会在回答处断开。
func TestAskUserReplyStaysInCurrentRound(t *testing.T) {
	l := NewStreamList()
	l.AppendUserMessage("s1", "dev", "需要你确认的问题")

	l.AppendReply("s1", "dev", "选 A")

	if len(l.Streams) != 1 {
		t.Fatalf("AskUser answer must not create a new stream, got %d streams", len(l.Streams))
	}
	if n := len(l.Streams[0].Items); n != 2 {
		t.Fatalf("answer not appended to current stream, items=%d", n)
	}
	if last := l.Streams[0].Items[1]; last.Kind != itemQuestion || last.Text != "选 A" {
		t.Errorf("answer item mismatch: %+v", last)
	}
}

// 历史里被旧数据落成 user 的授权魔术词，只是当前轮的控制载荷，
// 不能当作新提问切出一轮，也不能渲染成用户气泡。
func TestHistoryPermissionMagicDoesNotStartRound(t *testing.T) {
	msgs := []goharnesssession.Message{
		{Role: "user", Content: "帮我改文件"},
		{Role: "user", Content: "PermissionAllow"},
		{Role: "assistant", Content: "已修改", FinishReason: "stop"},
	}
	streams := StreamsFromMessages("s1", "dev", msgs)
	if len(streams) != 1 {
		t.Fatalf("permission magic word must not split history rounds, got %d streams", len(streams))
	}
	out := ViewStream(&streams[0], 80)
	if strings.Contains(out, "PermissionAllow") {
		t.Errorf("permission magic word should not be rendered as a user bubble:\n%s", out)
	}
}

// 外来会话事件（daemon 广播的定时任务、其他客户端流量）不得回发错误消息，
// 否则 AgentErrorMsg 携带同一 SessionID 再次路由，形成无限错误循环。
func TestForeignSessionEventDoesNotLoop(t *testing.T) {
	l := NewStreamList()
	l.AppendUserMessage("local-session", "dev", "hi")

	foreign := clientmsg.AgentErrorMsg{SessionID: "foreign-session", Error: assertErr("boom")}
	for i := 0; i < 100; i++ {
		newList, cmd := l.Update(foreign)
		l = newList
		if cmd != nil {
			msg := cmd()
			if _, isErr := msg.(clientmsg.AgentErrorMsg); isErr {
				t.Fatalf("iteration %d: foreign event must be dropped silently, got error cmd back (loop source)", i)
			}
		}
	}
	if len(l.Streams) != 1 {
		t.Fatalf("stream count changed: %d", len(l.Streams))
	}
	if n := len(l.Streams[0].Items); n != 1 {
		t.Errorf("local stream polluted with %d extra items", n-1)
	}
}

// 本地会话事件仍正常路由到对应流。
func TestLocalSessionEventStillRouted(t *testing.T) {
	l := NewStreamList()
	l.AppendUserMessage("s1", "dev", "q")

	newList, _ := l.Update(clientmsg.ContentDeltaMsg{SessionID: "s1", Content: "hello"})
	l = newList

	if len(l.Streams) != 1 || len(l.Streams[0].Items) != 2 {
		t.Fatalf("event not routed to local stream: %+v", l.Streams)
	}
	last := l.Streams[0].Items[len(l.Streams[0].Items)-1]
	if last.Kind != itemOutput || !strings.Contains(last.Text, "hello") {
		t.Errorf("output item not appended correctly: %+v", last)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

var _ tea.Msg = clientmsg.AgentErrorMsg{}
