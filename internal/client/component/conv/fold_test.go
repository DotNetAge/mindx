package conv

import (
	"strings"
	"testing"
	"time"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

// 构造一条完整的执行流：提问 → 两个工具调用 → 流式输出 → 结论。
func buildExecutedStream(t *testing.T) Stream {
	t.Helper()
	s := NewStream("s1", "dev", "帮我查一下")
	s, _ = UpdateStream(s, clientmsg.ToolExecStartMsg{SessionID: "s1", ToolName: "Bash", ToolCallID: "t1"})
	s, _ = UpdateStream(s, clientmsg.ToolExecEndMsg{
		SessionID: "s1", ToolCallID: "t1", Success: true, Result: "ok",
		Duration: 2 * time.Second, PromptTokens: 100, CompletionTokens: 50,
	})
	s, _ = UpdateStream(s, clientmsg.ToolExecStartMsg{SessionID: "s1", ToolName: "Read", ToolCallID: "t2"})
	s, _ = UpdateStream(s, clientmsg.ToolExecEndMsg{
		SessionID: "s1", ToolCallID: "t2", Success: true, Result: "file content",
		Duration: time.Second, PromptTokens: 80, CompletionTokens: 20,
	})
	s, _ = UpdateStream(s, clientmsg.ContentDeltaMsg{SessionID: "s1", Content: "答案部分"})
	s, _ = UpdateStream(s, clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "最终结论"})
	return s
}

// 结论到达仅置位折叠标记；终态门控保证执行中工具始终可见。
func TestFoldGatingByTerminalState(t *testing.T) {
	s := buildExecutedStream(t)

	if !s.ToolsFolded {
		t.Fatal("FinalAnswer should arm ToolsFolded")
	}
	if s.Status != StatusResponding {
		t.Fatalf("status after final answer = %v", s.Status)
	}
	// 终态未到：工具仍完整显示。
	if out := ViewStream(&s, 80); !strings.Contains(out, "Bash") {
		t.Errorf("tools must stay visible before terminal state, got:\n%s", out)
	}

	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})
	out := ViewStream(&s, 80)
	if strings.Contains(out, "Bash") || strings.Contains(out, "Read") {
		t.Errorf("tool steps must be folded after done, got:\n%s", out)
	}
	if !strings.Contains(out, "ctrl+o") {
		t.Errorf("folded summary should carry expand hint, got:\n%s", out)
	}
	if !strings.Contains(out, "最终结论") {
		t.Errorf("content must follow the summary line, got:\n%s", out)
	}
}

// 折叠摘要携带工具数量与 token 总耗（数字不受 locale 影响）。
func TestFoldSummaryCountsAndTokens(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	out := ViewStream(&s, 80)
	// t1: 100+50=150；t2: 80+20=100；合计 250。
	if !strings.Contains(out, "250") {
		t.Errorf("summary should contain total tokens 250, got:\n%s", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("summary should contain tool count 2, got:\n%s", out)
	}
}

// ctrl+o 手动切换：折叠后可展开恢复全部工具步骤。
func TestToggleToolsFoldRestoresView(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	s, _ = UpdateStream(s, clientmsg.ToggleToolsFoldMsg{SessionID: "s1"})
	if out := ViewStream(&s, 80); !strings.Contains(out, "Bash") {
		t.Errorf("manual unfold should restore tool steps, got:\n%s", out)
	}

	s, _ = UpdateStream(s, clientmsg.ToggleToolsFoldMsg{SessionID: "s1"})
	if out := ViewStream(&s, 80); strings.Contains(out, "Bash") {
		t.Errorf("manual re-fold should hide tool steps again, got:\n%s", out)
	}
}

// 同流追问的真实路径：handleSend 为每次发送创建新流。
// 旧流保持折叠终态，新流从零开始（未折叠），互不干扰。
func TestFollowUpCreatesNewStreamUnfolded(t *testing.T) {
	l := NewStreamList()
	l.AppendUserMessage("s1", "dev", "第一个问题")
	st := &l.Streams[len(l.Streams)-1]
	*st, _ = UpdateStream(*st, clientmsg.ToolExecStartMsg{SessionID: "s1", ToolName: "Bash", ToolCallID: "t1"})
	*st, _ = UpdateStream(*st, clientmsg.ToolExecEndMsg{
		SessionID: "s1", ToolCallID: "t1", Success: true, Result: "ok",
		PromptTokens: 100, CompletionTokens: 50,
	})
	*st, _ = UpdateStream(*st, clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "结论一"})
	*st, _ = UpdateStream(*st, clientmsg.SessionDoneMsg{SessionID: "s1"})

	// 追问：新流。
	l.AppendUserMessage("s1", "dev", "第二个问题")
	if len(l.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(l.Streams))
	}

	oldOut := ViewStream(&l.Streams[0], 80)
	if strings.Contains(oldOut, "Bash") {
		t.Errorf("previous stream must stay folded, got:\n%s", oldOut)
	}
	newOut := ViewStream(&l.Streams[1], 80)
	if l.Streams[1].ToolsFolded {
		t.Error("new stream must start unfolded")
	}
	_ = newOut
}

// 折叠窗口边界：最后一个用户提问之后的工具才参与折叠，
// 之前轮次的工具步骤保持原样。
func TestFoldWindowBoundByLastQuestion(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})
	// 追问开启新一轮。
	s.append(Item{Kind: itemQuestion, Text: "第二个问题"})
	s, _ = UpdateStream(s, clientmsg.ToolExecStartMsg{SessionID: "s1", ToolName: "Grep", ToolCallID: "t3"})
	s, _ = UpdateStream(s, clientmsg.ToolExecEndMsg{
		SessionID: "s1", ToolCallID: "t3", Success: true, Result: "match",
	})
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	out := ViewStream(&s, 80)
	if !strings.Contains(out, "Bash") || !strings.Contains(out, "Read") {
		t.Errorf("previous round tools must stay outside the fold window, got:\n%s", out)
	}
	if strings.Contains(out, "Grep") {
		t.Errorf("current round tools must be folded, got:\n%s", out)
	}
}

// 历史还原的流默认折叠，与运行时终态一致。
func TestHistoryRestoreDefaultsFolded(t *testing.T) {
	streams := StreamsFromMessages("s1", "dev", nil)
	for _, st := range streams {
		if !st.ToolsFolded {
			t.Error("restored streams should default to folded")
		}
	}
}
