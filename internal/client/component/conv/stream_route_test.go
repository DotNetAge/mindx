package conv

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

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
