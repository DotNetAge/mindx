package conv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const tickInterval = 250 * time.Millisecond

// ─────────────────────────────────────────────────────────────
// 事件驱动的对话流模型
//
// Stream 是单个会话的线性时间线：每个 Item 由 JSON-RPC 事件按到达顺序
// 追加/扩展（ELM 单向数据流，Update 只做纯状态转移）。
//
//   thinking_delta → ThinkingBlock（全文小斜体流式追加）
//   tool_use_delta / tool_exec_start → ActionStep（工具名(参数) | 计时 | Token）
//   tool_exec_end   → 填充结果 / 错误与真实 token 消耗（in+out-cached）
//   markdown(content_delta/final_answer/task_summary) → OutputBlock
//   form(AskUser)   → QuestionItem + 内联 Choices（输入栏隐藏）
//   error / max_turns_reached / llm_cancelled / llm_timeout → Error/Notice
// ─────────────────────────────────────────────────────────────

type itemKind int

const (
	itemQuestion itemKind = iota // 用户提问 / AskUser 问题
	itemThinking                 // 思想流
	itemAction                   // 工具调用
	itemOutput                   // Content（markdown）
	itemError                    // 错误组件
	itemNotice                   // 提示框（max turns / cancelled / timeout / 子任务）
)

// Item 是时间线上的一个条目。按 Kind 只使用对应字段组。
type Item struct {
	Kind itemKind

	// itemQuestion / itemOutput / itemNotice 共用：文本内容。
	Text      string
	Streaming bool // output：流式接收中

	// itemThinking：思想流正文存于 Text。
	ThinkingActive bool
	ThinkingStart  time.Time
	ThinkingDur    time.Duration

	// itemAction
	Action ActionStep

	// itemError
	ErrorData ErrorMsg

	// 完成态输出块的渲染缓存（glamour 结果按宽度复用，见 ViewStream）。
	cachedView  string
	cachedWidth int
}

// Stream 是单会话的事件驱动对话流。
type Stream struct {
	SessionID string
	AgentName string
	Status    Status
	CreatedAt time.Time
	Items     []Item

	BlinkOn     bool // Tick 驱动的闪烁相位（thinking / executing 共用）
	ToolsFolded bool // 结论达成后把工具步骤折叠为单行摘要（ctrl+o 手动切换）
}

// NewStream 创建以用户提问开头的会话流。
func NewStream(sessionID, agentName, question string) Stream {
	s := Stream{
		SessionID: sessionID,
		AgentName: agentName,
		Status:    StatusThinking,
		CreatedAt: time.Now(),
	}
	if question != "" {
		s.append(Item{Kind: itemQuestion, Text: question})
	}
	return s
}

func (s *Stream) append(it Item) {
	s.Items = append(s.Items, it)
}

// last 返回满足匹配的最后一条 Item 下标；无则 -1。
func (s *Stream) last(match func(Item) bool) int {
	for i := len(s.Items) - 1; i >= 0; i-- {
		if match(s.Items[i]) {
			return i
		}
	}
	return -1
}

// ensureThinkingBlock 返回可续写的思考块下标：活动块续写；
// 已关闭但为空的块复用；否则新建。
func (s *Stream) ensureThinkingBlock() int {
	i := s.last(func(it Item) bool { return it.Kind == itemThinking })
	if i >= 0 && (s.Items[i].ThinkingActive || s.Items[i].Text == "") {
		return i
	}
	s.append(Item{Kind: itemThinking, ThinkingActive: true, ThinkingStart: time.Now()})
	return len(s.Items) - 1
}

// findStep 定位 tool_exec_end 的目标步骤：优先精确 ToolCallID，
// 其次同名且执行中的最后一条。
func (s *Stream) findStep(toolCallID, toolName string) int {
	if toolCallID != "" {
		if i := s.last(func(it Item) bool {
			return it.Kind == itemAction && it.Action.ToolCallID == toolCallID
		}); i >= 0 {
			return i
		}
	}
	if i := s.last(func(it Item) bool {
		return it.Kind == itemAction && it.Action.Status == ActionStepExecuting &&
			(toolName == "" || it.Action.ToolName == toolName)
	}); i >= 0 {
		return i
	}
	return -1
}

// UpdateStream 处理定向到该会话流的事件（纯函数：返回新状态）。
func UpdateStream(s Stream, e tea.Msg) (Stream, tea.Cmd) {
	done := s.Status == StatusDone || s.Status == StatusError

	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		if done || e.Content == "" {
			return s, nil
		}
		i := s.ensureThinkingBlock()
		it := &s.Items[i]
		if !it.ThinkingActive {
			it.ThinkingActive = true
			it.ThinkingStart = time.Now()
		}
		it.Text += e.Content
		if s.Status != StatusExecuting {
			s.Status = StatusThinking
		}
		return s, nil

	case msg.ThinkingDoneMsg:
		i := s.last(func(it Item) bool { return it.Kind == itemThinking && it.ThinkingActive })
		if i < 0 {
			return s, nil
		}
		it := &s.Items[i]
		it.ThinkingActive = false
		if !it.ThinkingStart.IsZero() {
			it.ThinkingDur = time.Since(it.ThinkingStart).Round(10 * time.Millisecond)
		}
		if e.Content != "" && it.Text == "" {
			it.Text = e.Content
		}
		return s, nil

	case msg.ToolUseDeltaMsg:
		if done {
			return s, nil
		}
		i := s.last(func(it Item) bool {
			return it.Kind == itemAction && it.Action.Status == ActionStepExecuting &&
				it.Action.ToolCallID == "" && it.Action.ArgIndex == e.Index
		})
		if i < 0 {
			s.append(Item{Kind: itemAction, Action: ActionStep{
				ToolName: e.Name, ArgIndex: e.Index, StreamingArgs: e.Arguments,
				Status: ActionStepExecuting,
			}})
		} else {
			it := &s.Items[i]
			if e.Name != "" {
				it.Action.ToolName = e.Name
			}
			it.Action.StreamingArgs += e.Arguments
		}
		s.Status = StatusExecuting
		return s, nil

	case msg.ToolExecStartMsg:
		if done {
			return s, nil
		}
		// tool_use_delta 可能已创建占位步骤：优先绑定并回填参数。
		i := s.last(func(it Item) bool {
			return it.Kind == itemAction && it.Action.Status == ActionStepExecuting &&
				it.Action.ToolCallID == "" &&
				(it.Action.ToolName == e.ToolName || it.Action.ToolName == "")
		})
		if i >= 0 {
			it := &s.Items[i]
			it.Action.ToolCallID = e.ToolCallID
			if len(e.Params) > 0 {
				it.Action.Params = e.Params
			}
			if it.Action.StartTime.IsZero() {
				it.Action.StartTime = time.Now()
			}
		} else {
			s.append(Item{Kind: itemAction, Action: ActionStep{
				ToolName: e.ToolName, ToolCallID: e.ToolCallID, Params: e.Params,
				Status: ActionStepExecuting, StartTime: time.Now(),
			}})
		}
		s.Status = StatusExecuting
		return s, nil

	case msg.ToolExecEndMsg:
		i := s.findStep(e.ToolCallID, e.ToolName)
		if i < 0 {
			return s, nil
		}
		step := &s.Items[i].Action
		step.Duration = e.Duration
		// 真实 token 消耗口径（服务器计算）：输入 + 输出 - 缓存。
		step.Tokens = e.PromptTokens + e.CompletionTokens - e.CachedTokens
		if step.Tokens < 0 {
			step.Tokens = 0
		}
		if e.Success {
			step.Status = ActionStepDone
			step.Result = e.Result
		} else {
			step.Status = ActionStepFailed
			step.Result = e.Error
		}
		if e.DiffText != "" {
			step.DiffText, step.DiffFile = e.DiffText, e.DiffFile
			step.DiffAdds, step.DiffDels = e.DiffAdds, e.DiffDels
		}
		return s, nil

	case msg.ContentDeltaMsg:
		if done {
			return s, nil
		}
		i := s.last(func(it Item) bool { return it.Kind == itemOutput })
		if i >= 0 && s.Items[i].Streaming {
			s.Items[i].Text += e.Content
		} else {
			s.append(Item{Kind: itemOutput, Text: e.Content, Streaming: true})
		}
		s.Status = StatusResponding
		return s, nil

	case msg.FinalAnswerMsg:
		i := s.last(func(it Item) bool { return it.Kind == itemOutput })
		if i >= 0 && s.Items[i].Streaming {
			s.Items[i].Text = e.Content
			s.Items[i].Streaming = false
		} else if i < 0 || s.Items[i].Text != e.Content {
			s.append(Item{Kind: itemOutput, Text: e.Content})
		}
		s.Status = StatusResponding
		// 结论已到达：折叠本轮工具步骤为单行摘要（显示门控见 foldActive，
		// 需等终态才生效，避免流式期间误藏后续工具）。
		s.ToolsFolded = true
		return s, nil

	case msg.TaskSummaryMsg:
		if e.Content != "" {
			s.append(Item{Kind: itemOutput, Text: e.Content})
		}
		return s, nil

	case msg.AgentErrorMsg:
		errMsg := e.Error.Error()
		if i := s.last(func(it Item) bool { return it.Kind == itemError }); i >= 0 && s.Items[i].ErrorData.Error == errMsg {
			return s, nil
		}
		s.append(Item{Kind: itemError, ErrorData: ErrorMsg{Error: errMsg, Phase: extractPhase(errMsg), Time: time.Now()}})
		s.Status = StatusError
		return s, nil

	case msg.MaxTurnsReachedMsg:
		text := e.Suggestion
		if text == "" {
			text = fmt.Sprintf("%d / %d", e.TurnsCompleted, e.MaxTurns)
		}
		s.append(Item{Kind: itemNotice, Text: "💡 " + text})
		s.Status = StatusDone
		return s, nil

	case msg.LLMCancelledMsg:
		s.append(Item{Kind: itemNotice, Text: fmt.Sprintf(i18n.T("output.cancelled"), e.Elapsed.Round(time.Second))})
		s.Status = StatusDone
		return s, nil

	case msg.LLMTimeoutMsg:
		text := fmt.Sprintf(i18n.T("output.timeout.llm"), e.Timeout.Round(time.Second), e.Elapsed.Round(time.Second))
		s.append(Item{Kind: itemNotice, Text: text})
		s.Status = StatusError
		return s, nil

	case msg.SubtaskSpawnedMsg:
		icon := "🚀"
		name := e.AgentName
		if name == "" {
			name = i18n.T("svc.md.subtask.spawned")
		} else {
			name = icon + " " + name
		}
		s.append(Item{Kind: itemNotice, Text: strings.TrimSpace(name + " · " + e.Description)})
		return s, nil

	case msg.SubtaskCompletedMsg:
		if e.Success && e.Answer != "" {
			s.append(Item{Kind: itemOutput, Text: e.Answer})
		} else if !e.Success {
			s.append(Item{Kind: itemNotice, Text: "❌ " + e.AgentName + " · " + e.Error})
		}
		return s, nil

	case msg.CollapseToggleMsg:
		toggleAt := func(idx int) {
			n := 0
			for i := range s.Items {
				if s.Items[i].Kind != itemAction {
					continue
				}
				if idx < 0 || n == idx {
					s.Items[i].Action.Collapsed = !s.Items[i].Action.Collapsed
				}
				n++
			}
		}
		toggleAt(e.ActionIndex)
		return s, nil

	case msg.ToggleToolsFoldMsg:
		s.ToolsFolded = !s.ToolsFolded
		return s, nil

	case msg.SessionDoneMsg:
		s.Status = StatusDone
		s.ToolsFolded = true
		for i := range s.Items {
			it := &s.Items[i]
			if it.Kind == itemThinking && it.ThinkingActive {
				it.ThinkingActive = false
				if !it.ThinkingStart.IsZero() {
					it.ThinkingDur = time.Since(it.ThinkingStart).Round(10 * time.Millisecond)
				}
			}
			if it.Kind == itemAction && it.Action.Status == ActionStepExecuting {
				it.Action.Status = ActionStepDone
			}
			it.Streaming = false
		}
		return s, nil

	case msg.TickMsg:
		s.BlinkOn = !s.BlinkOn
		return s, nil
	}

	return s, nil
}

// ── 渲染 ────────────────────────────────────────────────────

// ViewStream 渲染整条会话流；width 为可用列宽。
// 完成态输出块按 (item, width) 缓存渲染结果：Tick 每 250ms 触发全量重渲，
// glamour 解析随历史增长线性变贵，是长会话卡顿的主因，缓存后稳态帧零重算。
func ViewStream(s *Stream, width int) string {
	fold := s.foldActive()
	windowStart := s.lastQuestionIndex() + 1

	var b strings.Builder
	summaryDone := false
	for i := range s.Items {
		it := s.Items[i]
		var v string
		switch {
		case fold && it.Kind == itemAction && i >= windowStart:
			// 折叠窗口内的首个工具步骤处输出摘要行，其余跳过。
			if !summaryDone {
				v = viewFoldedTools(s, windowStart, width)
				summaryDone = true
			}
		case it.Kind == itemOutput:
			v = s.cachedOutputView(i, width)
		default:
			v = viewItem(*s, it, width)
		}
		if v == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(v)
	}
	return b.String()
}

// foldActive 折叠仅在终态生效：同流再次激活时状态离开 Done/Error，
// 工具步骤自动恢复完整显示，无需额外状态迁移。
func (s *Stream) foldActive() bool {
	return s.ToolsFolded && (s.Status == StatusDone || s.Status == StatusError)
}

func (s *Stream) lastQuestionIndex() int {
	return s.last(func(it Item) bool { return it.Kind == itemQuestion })
}

// viewFoldedTools 把折叠窗口内（最后一个用户提问之后）的工具步骤
// 压缩为单行摘要：数量 + token 总耗 + 累计时长。
func viewFoldedTools(s *Stream, windowStart, width int) string {
	count := 0
	var tokens int
	var elapsed time.Duration
	for i := windowStart; i < len(s.Items); i++ {
		it := s.Items[i]
		if it.Kind != itemAction {
			continue
		}
		count++
		tokens += it.Action.Tokens
		if it.Action.Duration > 0 {
			elapsed += it.Action.Duration
		}
	}
	if count == 0 {
		return ""
	}
	meta := fmt.Sprintf(i18n.T("conv.tools.folded"), count, formatNumber(tokens))
	if elapsed > 0 {
		meta += " · " + formatDuration(elapsed)
	}
	line := style.GrayStyle.Render("⏺ ") + style.DimStyle.Render(meta+" (ctrl+o)")
	return lipgloss.NewStyle().Width(width).Render(line)
}

// cachedOutputView 返回输出块的渲染结果；完成态命中缓存直接返回，
// 流式块与宽度变化时重新渲染。
func (s *Stream) cachedOutputView(idx, width int) string {
	it := &s.Items[idx]
	if strings.TrimSpace(it.Text) == "" {
		return ""
	}
	if !it.Streaming && it.cachedWidth == width && it.cachedView != "" {
		return it.cachedView
	}
	v := viewOutputBlock(*it, width)
	if !it.Streaming {
		it.cachedView, it.cachedWidth = v, width
	}
	return v
}

func viewItem(s Stream, it Item, width int) string {
	switch it.Kind {
	case itemQuestion:
		return ViewQuestion(Question{Text: it.Text}, width)
	case itemThinking:
		return viewThinkingItem(s, it)
	case itemAction:
		return ViewActionStep(s, it.Action, width)
	case itemOutput:
		return viewOutputBlock(it, width)
	case itemError:
		return ViewErrorMsg(it.ErrorData, width)
	case itemNotice:
		return viewNotice(it.Text, width)
	}
	return ""
}

// viewThinkingItem 完整显示整个思想流，小斜体呈现。
func viewThinkingItem(s Stream, it Item) string {
	if it.Text == "" && !it.ThinkingActive {
		return ""
	}

	thoughtStyle := lipgloss.NewStyle().Foreground(style.ThemeDim).Italic(true)

	var b strings.Builder
	if it.ThinkingActive {
		b.WriteString(ViewBlink(Blink{Symbol: "⏺ " + i18n.T("client.ui.thinking.active"), BlinkOn: s.BlinkOn}, thoughtStyle))
		b.WriteByte('\n')
	} else {
		d := it.ThinkingDur
		b.WriteString(style.GrayStyle.Render(fmt.Sprintf(i18n.T("client.ui.thinking.done"), d)))
		b.WriteByte('\n')
	}

	lines := strings.Split(strings.TrimRight(it.Text, "\n"), "\n")
	for _, line := range lines {
		b.WriteString(thoughtStyle.Render("│ " + line))
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// viewOutputBlock 渲染 Content 块（markdown），沿用分隔线视觉。
func viewOutputBlock(it Item, width int) string {
	if strings.TrimSpace(it.Text) == "" {
		return ""
	}
	sep := style.Divider(strings.Repeat("─", width))
	content := render.MarkdownWithWidth(it.Text, width-4)
	if it.Streaming {
		content += style.DimStyle.Render("▌")
	}
	return sep + "\n" + content
}

func viewNotice(text string, width int) string {
	yellow := lipgloss.NewStyle().Foreground(style.ThemeYellow)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemeYellow).
		Padding(0, 1).
		Width(width - 4)
	return border.Render(yellow.Render(text))
}

// ── 多会话列表 ──────────────────────────────────────────────

// StreamList 维护多条会话流并把事件路由到对应 SessionID 的最后一条。
type StreamList struct {
	Streams []Stream
	width   int
	timer   timer.Model
}

func NewStreamList() StreamList {
	return StreamList{
		timer: timer.New(100*365*24*time.Hour, timer.WithInterval(tickInterval)),
	}
}

func (l StreamList) Init() tea.Cmd {
	return l.timer.Init()
}

func (l StreamList) Update(e tea.Msg) (StreamList, tea.Cmd) {
	switch e := e.(type) {
	case msg.WindowResizeMsg:
		l.width = e.Width

	case msg.ClearScreenMsg:
		l.Streams = nil

	case timer.TickMsg:
		newTimer, timerCmd := l.timer.Update(e)
		l.timer = newTimer
		now := time.Now()
		for i := range l.Streams {
			l.Streams[i], _ = UpdateStream(l.Streams[i], msg.TickMsg{Time: now})
		}
		return l, timerCmd

	default:
		sessionID := getSessionID(e)
		if sessionID == "" {
			return l, nil
		}
		for i := len(l.Streams) - 1; i >= 0; i-- {
			if l.Streams[i].SessionID != sessionID {
				continue
			}
			newStream, cmd := UpdateStream(l.Streams[i], e)
			l.Streams[i] = newStream
			return l, cmd
		}
		// daemon 会广播所有会话的事件（定时任务、其他客户端的流量），
		// 本地未打开对应流属正常情况，静默丢弃。
		// 此处严禁回发 AgentErrorMsg：它同样携带 SessionID，会再次进入
		// 本路由且永远找不到流，形成无限错误消息循环（界面被日志冲乱的元凶）。
	}
	return l, nil
}

func (l StreamList) View() string {
	if len(l.Streams) == 0 {
		return ""
	}
	var parts []string
	for i := len(l.Streams) - 1; i >= 0; i-- {
		v := ViewStream(&l.Streams[i], l.width)
		if v == "" {
			continue
		}
		parts = append([]string{v}, parts...)
	}
	return strings.Join(parts, "\n\n")
}

func (l *StreamList) Clear() {
	l.Streams = nil
}

// AppendUserMessage 追加一条以用户提问开头的新会话流。
func (l *StreamList) AppendUserMessage(sessionID, agentName, question string) {
	l.Streams = append(l.Streams, NewStream(sessionID, agentName, question))
}

// AppendQuestion 向指定会话的最后一条流追加 AskUser 问题条目；
// 会话流不存在时创建。用于把 AskUser 事件内联渲染到对话中。
func (l *StreamList) AppendQuestion(sessionID, question string) {
	for i := len(l.Streams) - 1; i >= 0; i-- {
		if l.Streams[i].SessionID == sessionID {
			l.Streams[i].append(Item{Kind: itemQuestion, Text: question})
			return
		}
	}
	l.AppendUserMessage(sessionID, "", question)
}

// getSessionID 提取事件的会话归属。
func getSessionID(e tea.Msg) string {
	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		return e.SessionID
	case msg.ThinkingDoneMsg:
		return e.SessionID
	case msg.ToolUseDeltaMsg:
		return e.SessionID
	case msg.ToolExecStartMsg:
		return e.SessionID
	case msg.ToolExecEndMsg:
		return e.SessionID
	case msg.ContentDeltaMsg:
		return e.SessionID
	case msg.FinalAnswerMsg:
		return e.SessionID
	case msg.ExecutionSummaryMsg:
		return e.SessionID
	case msg.TaskSummaryMsg:
		return e.SessionID
	case msg.SubtaskSpawnedMsg:
		return e.SessionID
	case msg.SubtaskCompletedMsg:
		return e.SessionID
	case msg.AgentErrorMsg:
		return e.SessionID
	case msg.LLMTimeoutMsg:
		return e.SessionID
	case msg.LLMCancelledMsg:
		return e.SessionID
	case msg.MaxTurnsReachedMsg:
		return e.SessionID
	case msg.CompactionMsg:
		return e.SessionID
	case msg.IterationMsg:
		return e.SessionID
	case msg.UserMessageSavedMsg:
		return e.SessionID
	case msg.MessageQueuedMsg:
		return e.SessionID
	case msg.MessageProcessingMsg:
		return e.SessionID
	case msg.PermissionDeniedMsg:
		return e.SessionID
	case msg.CollapseToggleMsg:
		return e.SessionID
	case msg.ThinkCollapseMsg:
		return e.SessionID
	case msg.ToggleToolsFoldMsg:
		return e.SessionID
	case msg.SessionDoneMsg:
		return e.SessionID
	default:
		return ""
	}
}

// ── 会话历史还原 ────────────────────────────────────────────

// StreamsFromMessages 从持久化的会话消息还原事件流时间线
// （assistant 消息可能同时携带 reasoning_content、tool_calls 与正文）。
func StreamsFromMessages(sessionID, agentName string, msgs []session.Message) []Stream {
	var streams []Stream
	cur := -1

	current := func() *Stream {
		if cur < 0 {
			return nil
		}
		return &streams[cur]
	}

	// 按 tool_call_id 索引待匹配的工具调用（与 Web 前端逻辑对齐）。
	type pendingToolCall struct {
		name string
		args map[string]any
	}
	pending := map[string]pendingToolCall{}

	for _, m := range msgs {
		switch m.Role {
		case "user":
			streams = append(streams, NewStream(sessionID, agentName, m.Content))
			cur = len(streams) - 1
			streams[cur].Status = StatusDone

		case "assistant":
			s := current()
			if s == nil {
				continue
			}
			// 1) 思想流（reasoning_content 为 assistant 消息内嵌字段）
			if m.ReasoningContent != "" {
				s.append(Item{Kind: itemThinking, Text: m.ReasoningContent})
			}
			// 2) 收集 tool_calls（goharness 扁平格式 {id,name,arguments}）
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &args)
				}
				pending[tc.ID] = pendingToolCall{name: tc.Name, args: args}
			}
			// 3) 正文 → Output
			if m.Content != "" {
				s.append(Item{Kind: itemOutput, Text: m.Content})
			}

		case "tool":
			s := current()
			if s == nil {
				continue
			}
			match, ok := pending[m.ToolCallID]
			if !ok {
				match = pendingToolCall{name: "tool"}
			}
			success := true
			if strings.HasPrefix(m.Content, "[") && strings.Contains(m.Content, "] error:") {
				success = false
			}
			status := ActionStepDone
			if !success {
				status = ActionStepFailed
			}
			s.append(Item{Kind: itemAction, Action: ActionStep{
				ToolName: match.name, ToolCallID: m.ToolCallID, Params: match.args,
				Status: status, Result: m.Content,
			}})
			delete(pending, m.ToolCallID)
		}
	}

	for i := range streams {
		streams[i].Status = StatusDone
		// 历史还原默认折叠：与运行时终态一致，且避免长会话回放时的全量重渲开销。
		streams[i].ToolsFolded = true
	}
	return streams
}
