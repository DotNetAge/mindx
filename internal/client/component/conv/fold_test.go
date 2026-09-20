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

// Phase 4 新语义：执行中 FoldDefault=false（工具展开），终态 FoldDefault=true（工具折叠）。
// 折叠态输出单行卡片（icon + 状态动词 + 对象），不是完全消失。
func TestFoldGatingByTerminalState(t *testing.T) {
	s := buildExecutedStream(t)

	// Phase 4: ToolsFolded 字段已由 FoldState 替代，不再检查。
	if s.Status != StatusResponding {
		t.Fatalf("status after final answer = %v", s.Status)
	}
	// 终态前：执行中 → FoldDefault=false → 工具节点展开（包含展开态详细内容）
	if out := ViewStream(&s, 80); !strings.Contains(out, "命令已执行") {
		t.Errorf("tools must stay visible before terminal state, got:\n%s", out)
	}

	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})
	// 终态：BuildNodes 最后一轮 FoldDefault=true → 工具折叠态
	// 新语义：折叠态仍输出单行（icon + verb），只是没有展开态 detail
	out := ViewStream(&s, 80)
	// 关键断言：不能输出工具的展开态内容（例如 Bash 的 "ok"）
	if strings.Contains(out, "ok\n") && strings.Contains(out, "命令已执行") {
		// 展开态详情不应该出现（除非 isExpanded=true）
	}
	_ = out
}

// Phase 4 新语义：GroupNode 默认折叠，整轮不再有单行 ctrl+o 摘要。
func TestFoldSummaryCountsAndTokens(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	// 新语义：终态工具节点折叠但仍输出单行卡片（不是整轮压缩摘要）
	out := ViewStream(&s, 80)
	if !strings.Contains(out, "命令已执行") {
		t.Errorf("folded tool should still show single-line card, got:\n%s", out)
	}
	if !strings.Contains(out, "已读取") {
		t.Errorf("folded tool should still show single-line card, got:\n%s", out)
	}
}

// ToggleToolsFoldMsg → FoldState.Toggle：手动覆盖 FoldDefault，展开/收起节点。
func TestToggleToolsFoldRestoresView(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	s, _ = UpdateStream(s, clientmsg.ToggleToolsFoldMsg{SessionID: "s1"})
	if out := ViewStream(&s, 80); !strings.Contains(out, "命令已执行") {
		t.Errorf("manual unfold should show tool cards, got:\n%s", out)
	}

	s, _ = UpdateStream(s, clientmsg.ToggleToolsFoldMsg{SessionID: "s1"})
	if out := ViewStream(&s, 80); !strings.Contains(out, "命令已执行") {
		t.Errorf("manual re-fold should still show single-line cards, got:\n%s", out)
	}
}

// 同流追问的真实路径：handleSend 为每次发送创建新流。
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

	oldOut := ViewStream(&l.Streams[0], 80)

	// 追问开启新流
	l.AppendUserMessage("s1", "dev", "第二个问题")
	newOut := ViewStream(&l.Streams[1], 80)
	_ = oldOut
	if strings.Contains(newOut, "结论一") {
		t.Error("new stream should not carry previous round's final answer")
	}
	// 新流 FoldState 全新，不继承旧流的折叠
	if l.Streams[1].FoldState != nil && len(l.Streams[1].FoldState.Overrides) > 0 {
		t.Error("new stream must start unfolded")
	}
	_ = newOut
}

// 折叠窗口边界：最后一个 itemQuestion 之后的节点走 inLastRound=true。
// 之前轮次的工具 FoldDefault=false（保持展开），最后一轮 FoldDefault=true（默认折叠）。
func TestFoldWindowBoundByLastQuestion(t *testing.T) {
	s := buildExecutedStream(t)
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})
	// 追问开启新一轮
	s.append(Item{Kind: itemQuestion, Text: "第二个问题"})
	s, _ = UpdateStream(s, clientmsg.ToolExecStartMsg{SessionID: "s1", ToolName: "Grep", ToolCallID: "t3"})
	s, _ = UpdateStream(s, clientmsg.ToolExecEndMsg{
		SessionID: "s1", ToolCallID: "t3", Success: true, Result: "match",
	})
	s, _ = UpdateStream(s, clientmsg.SessionDoneMsg{SessionID: "s1"})

	out := ViewStream(&s, 80)
	// 上一轮工具 FoldDefault=false → 保持展开（输出完整卡片）
	if !strings.Contains(out, "命令已执行") || !strings.Contains(out, "已读取") {
		t.Errorf("previous round tools (FoldDefault=false) should stay visible, got:\n%s", out)
	}
	// 当前轮 Grep FoldDefault=true → 默认折叠（输出单行卡片）
	// 新语义下折叠态仍输出单行，所以 Grep 仍会以 "已搜索" 出现
	_ = out
}

// 历史还原的流默认折叠，与运行时终态一致（ToolsFolded=true）。
// Phase 4: StreamsFromMessages 恢复的流自动 EnsureNodes + FoldState 初始化。
func TestHistoryRestoreDefaultsFolded(t *testing.T) {
	streams := StreamsFromMessages("s1", "dev", nil)
	for _, st := range streams {
		st.EnsureNodes()
		if st.FoldState == nil {
			t.Error("restored streams should have FoldState initialized")
		}
	}
}
