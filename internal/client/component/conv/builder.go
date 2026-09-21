package conv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════
// 对话树构建器（对齐 Desktop builder/index.ts）
//
// 输入：Item[]（累计事件序列）
// 输出：[]TreeNode（类型化节点树）
//
// 归一规则（与 Desktop 完全对齐）：
//   3. TaskCreate/Update 按 taskId 实体幂等 upsert，不落工具节点
//   3. TeamCreate 落 TeamNode（登记型一次性动作）
//   4. SubAgent 落 SubagentNode（subtask 双事件 upsert 单卡片）
//   4. CollectResults 落 CollectNode + 回填 subagent 完成态
//   11. 相邻同 GroupKey 工具段聚合为 GroupNode（tree depth ≤ 2）
//   11. 任何非工具节点都是 GroupBuffer 的边界
//
// 单轮 Items 数量通常 < 100，BuildNodes 是 O(n) 纯函数
// （无 I/O、无分配热点），每事件到达重建延迟 < 1ms。
// ═══════════════════════════════════════════════════════════

// BuildState 构建器会话级状态（对齐 Desktop SessionBuildState）。
// 负责实体注册表（taskNodes / subagentNodes），
// 跨轮 upsert 时新轮可以更新旧轮创建的实体卡。
type BuildState struct {
	roundStart    time.Time
	taskNodes     map[string]*TaskNode
	subagentNodes map[string]*SubagentNode
}

// NewBuildState 创建会话级构建状态。
func NewBuildState(roundStart time.Time) *BuildState {
	return &BuildState{
		roundStart:    roundStart,
		taskNodes:     map[string]*TaskNode{},
		subagentNodes: map[string]*SubagentNode{},
	}
}

// BuildNodes 主入口：从累计 Items 重建本轮节点树。
// 对齐 Desktop buildRound()。
func BuildNodes(items []Item, state *BuildState, isFinal bool) []TreeNode {
	if state == nil {
		state = NewBuildState(time.Now())
	}

	// 计算最后一个 itemQuestion 的下标 — 折叠窗口边界（对齐 Desktop 按轮分组）
	// 旧 ToolsFolded 语义：最后一轮（itemQuestion 之后）的工具才折叠，之前轮次保持展开
	lastQuestionIdx := -1
	for i, it := range items {
		if it.Kind == itemQuestion {
			lastQuestionIdx = i
		}
	}

	var nodes []TreeNode
	var buffer *GroupBuffer

	// flushBefore：任何非工具节点冲刷 GroupBuffer（对齐 Desktop flushBefore）
	flushBefore := func(inLastRound bool) {
		if buffer != nil {
			if flushed := buffer.flush(); flushed != nil {
				if g, ok := flushed.(*GroupNode); ok && !inLastRound {
					g.NodeBase.FoldDefault = false
				}
				nodes = append(nodes, flushed)
			}
			buffer = nil
		}
	}

	for i, it := range items {
		offset := state.offsetOf(it.At)
		inLastRound := lastQuestionIdx < 0 || i > lastQuestionIdx

		switch it.Kind {
		case itemThinking:
			flushBefore(inLastRound)
			if n := buildThinkingNode(it, offset); n != nil {
				nodes = append(nodes, n)
			}

		case itemOutput:
			flushBefore(inLastRound)
			if n := buildContentNode(it, offset, isFinal); n != nil {
				nodes = append(nodes, n)
			}

		case itemAction:
			// ── 实体归一路径 ──
			if TaskUpsertTools[it.Action.ToolName] {
				flushBefore(inLastRound)
				upsertTask(state, it, offset)
				continue
			}
			if it.Action.ToolName == TeamCreateTool {
				flushBefore(inLastRound)
				if n := buildTeamNode(state, it, offset); n != nil {
					nodes = append(nodes, n)
				}
				continue
			}
			if it.Action.ToolName == SubagentTool {
				flushBefore(inLastRound)
				upsertSubagent(state, it, offset)
				continue
			}
			if it.Action.ToolName == CollectTool {
				flushBefore(inLastRound)
				if n := buildCollectNode(state, it, offset); n != nil {
					nodes = append(nodes, n)
				}
				continue
			}
			// ── 工具节点路径 ──
			desc, ok := ToolDescriptors[it.Action.ToolName]
			if !ok {
				continue // 未登记工具 → 跳过（残余兜底位）
			}
			tool := buildToolNode(desc, it, offset)
			if tool == nil {
				continue
			}
			if !inLastRound {
				// 旧轮次：工具默认展开（对齐旧 ToolsFolded 窗口边界语义）
				tool.NodeBase.FoldDefault = false
			}
			if desc.GroupKey == nil {
				flushBefore(inLastRound)
				nodes = append(nodes, tool)
			} else {
				var flushed TreeNode
				flushed, buffer = buffer.push(tool, *desc.GroupKey)
				if flushed != nil {
					if g, ok := flushed.(*GroupNode); ok && !inLastRound {
						g.NodeBase.FoldDefault = false
					}
					nodes = append(nodes, flushed)
				}
			}

		case itemQuestion:
			flushBefore(inLastRound)
			nodes = append(nodes, buildAskUserNode(it, offset))

		case itemNotice:
			flushBefore(inLastRound)
			if n := buildSystemNode(state, it, offset); n != nil {
				nodes = append(nodes, n)
			}

		case itemError:
			flushBefore(inLastRound)
			nodes = append(nodes, buildErrorNode(it, offset))
		}
	}

	// 轮尾冲刷 GroupBuffer（默认最后一轮语义）
	flushBefore(true)
	return nodes
}

// offsetOf 计算 Item 到达时间相对于轮起点的偏移。
func (s *BuildState) offsetOf(at time.Time) time.Duration {
	if at.IsZero() {
		return 0
	}
	d := at.Sub(s.roundStart)
	if d < 0 {
		return 0
	}
	return d
}

// ═══════════════════════════════════════════════════════════
// GroupBuffer — 工具聚合缓冲（对齐 Desktop groupBuffer.ts）
//
// 相邻同 GroupKey 工具段聚合为 GroupNode，单成员去壳。
// 任何非工具节点/异类别工具/轮尾都会触发 flush。
// ═══════════════════════════════════════════════════════════

type GroupBuffer struct {
	groupKey GroupKey
	members  []*ToolNode
}

// push 入缓冲（对齐 Desktop pushToGroupBuffer）。
// 同 GroupKey → 追加；异类别 → flush 旧缓冲后开新缓冲；groupKey=nil → flush 后独立。
// 返回：flush 产生的节点（可能为 nil），更新后的 buffer。
func (g *GroupBuffer) push(tool *ToolNode, key GroupKey) (TreeNode, *GroupBuffer) {
	if g == nil {
		return nil, &GroupBuffer{groupKey: key, members: []*ToolNode{tool}}
	}
	if g.groupKey == key {
		g.members = append(g.members, tool)
		return nil, g
	}
	// 异类别：先 flush 再开新
	flushed := g.flush()
	return flushed, &GroupBuffer{groupKey: key, members: []*ToolNode{tool}}
}

// flush 冲刷缓冲为节点序列（对齐 Desktop flushGroupBuffer）。
// 单成员去壳返回 ToolNode；多成员聚合为 GroupNode。
func (g *GroupBuffer) flush() TreeNode {
	if g == nil || len(g.members) == 0 {
		return nil
	}
	if len(g.members) == 1 {
		return g.members[0] // 单成员不包组壳
	}

	// 组状态由成员推导（对齐 Desktop）
	status := NodeSuccess
	for _, m := range g.members {
		switch m.Base().Status {
		case NodeExecuting:
			status = NodeExecuting
			goto doneStatus
		case NodeFailed:
			status = NodeFailed
		}
	}
doneStatus:

	// 组内时序跨度：末成员 end − 首成员 start
	first := g.members[0]
	last := g.members[len(g.members)-1]
	duration := (last.Base().Offset + last.Base().Duration) - first.Base().Offset

	return &GroupNode{
		NodeBase: NodeBase{
			ID:          "group_" + first.Base().ID,
			Kind:        "group",
			Status:      status,
			Offset:      first.Base().Offset,
			Duration:    duration,
			FoldDefault: true,
		},
		GroupKey: string(g.groupKey),
		Children: g.members,
	}
}

// ═══════════════════════════════════════════════════════════
// 节点构造函数族
// ═══════════════════════════════════════════════════════════

// buildThinkingNode 思想流（对齐 Desktop thinking_delta + thinking_done）。
func buildThinkingNode(it Item, offset time.Duration) *ThinkingNode {
	if it.Text == "" && !it.ThinkingActive {
		return nil
	}
	status := NodeSuccess
	if it.ThinkingActive {
		status = NodeExecuting
	}
	return &ThinkingNode{
		NodeBase: NodeBase{
			ID:          fmt.Sprintf("thinking_%d", offset.Milliseconds()),
			Kind:        "thinking",
			Status:      status,
			Offset:      offset,
			Duration:    it.ThinkingDur,
			FoldDefault: true,
		},
		Text: it.Text,
	}
}

// buildContentNode 正文段（对齐 Desktop content/markdown/final_answer/task_summary）。
func buildContentNode(it Item, offset time.Duration, isFinal bool) *ContentNode {
	if it.Text == "" {
		return nil
	}
	finishReason := "streaming"
	if isFinal || it.IsFinal || !it.Streaming {
		finishReason = "stop"
	}
	status := NodeExecuting
	if !it.Streaming {
		status = NodeSuccess
	}
	return &ContentNode{
		NodeBase: NodeBase{
			ID:          fmt.Sprintf("content_%d", offset.Milliseconds()),
			Kind:        "content",
			Status:      status,
			Offset:      offset,
			FoldDefault: false, // content 默认展开
		},
		Text:         it.Text,
		Streaming:    it.Streaming,
		IsFinal:      it.IsFinal,
		FinishReason: finishReason,
	}
}

// buildToolNode 工具节点构造（对齐 Desktop buildToolNode）。
// 解析 ToolExecEndMsg 的 Result / Params / ToolCallID 填充 ToolNode payload。
func buildToolNode(desc ToolDescriptor, it Item, offset time.Duration) *ToolNode {
	step := it.Action
	if step.ToolName == "" {
		return nil
	}

	status := NodeExecuting
	switch step.Status {
	case ActionStepDone:
		status = NodeSuccess
	case ActionStepFailed:
		status = NodeFailed
	}

	node := &ToolNode{
		NodeBase: NodeBase{
			ID:          step.ToolCallID,
			Kind:        desc.NodeType,
			Status:      status,
			Offset:      offset,
			Duration:    step.Duration,
			Tokens:      step.Tokens,
			FoldDefault: true,
		},
		ToolName: step.ToolName,
	}

	// 从 Params 填充各工具专属字段（对齐 Desktop buildToolNode 的 switch）
	params := step.Params
	switch desc.NodeType {
	case "tool.bash":
		node.BashCmd = strParam(params, "command")
		node.BashExitCode = jsonResultInt(step.Result, "exit_code")
	case "tool.run_script":
		node.RunSkillName = strParam(params, "skill", "name")
	case "tool.read":
		node.FilePath = strParam(params, "filePath", "file_path", "path")
		node.ReadLines = step.DiffAdds + step.DiffDels // 或 Result 中的 lines_read
		if node.ReadLines == 0 {
			node.ReadLines = jsonResultInt(step.Result, "lines_read")
		}
	case "tool.write":
		node.FilePath = strParam(params, "filePath", "file_path")
		node.WriteBytes = jsonResultInt(step.Result, "bytes_written")
		node.EditAdds = step.DiffAdds
		node.EditDels = step.DiffDels
	case "tool.edit":
		node.FilePath = strParam(params, "file_path", "filePath")
		node.EditAdds = step.DiffAdds
		node.EditDels = step.DiffDels
	case "tool.ls":
		node.LsPath = strParam(params, "path")
		node.LsRecursive = boolParam(params, "recursive")
		node.LsEntries = jsonResultInt(step.Result, "entry_count")
	case "tool.glob":
		node.GlobPattern = strParam(params, "pattern")
		node.GlobMatches = jsonResultInt(step.Result, "match_count")
	case "tool.grep":
		node.GrepPattern = strParam(params, "pattern")
		node.GrepInclude = strParam(params, "include")
		node.GrepHits = jsonResultInt(step.Result, "hit_count")
	case "tool.web_fetch":
		node.WebFetchURL = strParam(params, "url")
		node.WebFetchTitle = builderJsonResultStr(step.Result, "title")
		node.WebFetchBytes = jsonResultInt(step.Result, "bytes")
	case "tool.web_search":
		node.WebSearchQuery = strParam(params, "query")
		node.WebSearchCount = jsonResultInt(step.Result, "result_count")
		node.WebSearchCached = boolParam(params, "cached")
	case "tool.kb_search":
		node.KBSearchQuery = strParam(params, "query")
		node.KBSearchHits = jsonResultInt(step.Result, "hit_count")
	case "tool.memory_search":
		node.MemorySearchQuery = strParam(params, "query")
		node.MemorySearchHits = jsonResultInt(step.Result, "hit_count")
	case "tool.skill":
		node.SkillName = strParam(params, "name")
	case "tool.sleep":
		node.SleepDuration = time.Duration(jsonResultInt(step.Result, "duration_ms")) * time.Millisecond
	case "tool.task_query":
		node.TaskQueryResult = truncate(step.Result, 60)
	case "tool.team_ops":
		node.TeamOpsAction = step.ToolName // TeamDelete/TeamList/TeamGetTasks → 归一后处理
		switch step.ToolName {
		case "TeamDelete":
			node.TeamOpsAction = "delete"
		case "TeamList":
			node.TeamOpsAction = "list"
		case "TeamGetTasks":
			node.TeamOpsAction = "get_tasks"
		}
		node.TeamOpsTeam = strParam(params, "team_name")
	case "tool.cron":
		node.CronAction = strParam(params, "action")
		node.CronID = strParam(params, "id")
		node.CronAgent = strParam(params, "agent")
		node.CronExpr = strParam(params, "cron_expr")
	case "tool.notify":
		node.NotifyTitle = strParam(params, "title")
		node.NotifyMessage = strParam(params, "message")
	}

	// 展开态原始输出（对齐 Desktop outputTail 尾段截断）
	node.ResultText = step.Result
	if len(step.Result) > 20000 {
		node.ResultTail = step.Result[len(step.Result)-20000:]
	} else {
		node.ResultTail = step.Result
	}

	return node
}

// ── 实体归一 ──

// upsertTask 任务实体幂等 upsert（对齐 Desktop upsertTaskNode）。
func upsertTask(state *BuildState, it Item, offset time.Duration) {
	result := parseResultJSON(it.Action.Result)
	taskID := builderJsonStr(result, "task_id")
	if taskID == "" {
		return
	}
	_, ok := state.taskNodes[taskID]
	if !ok {
		// 首次创建（位置固定）
		state.taskNodes[taskID] = &TaskNode{
			NodeBase: NodeBase{
				ID:          "task_" + taskID,
				Kind:        "task",
				Status:      NodeSuccess,
				Offset:      offset,
				FoldDefault: true,
			},
			TaskID:     taskID,
			Subject:    builderJsonStr(result, "subject"),
			TaskStatus: builderJsonStr(result, "status"),
		}
	} else {
		// 原位更新（后续迭代的 subject / status 回填）
		node := state.taskNodes[taskID]
		if s := builderJsonStr(result, "subject"); s != "" {
			node.Subject = s
		}
		if it.Action.ToolName == "TaskUpdate" {
			if st := builderJsonStr(result, "status"); st != "" && st != node.TaskStatus {
				node.TaskStatus = st
				node.Transitions = append(node.Transitions, TaskTransition{
					Status: st, At: offset,
				})
			}
		}
	}
}

// upsertSubagent 子任务卡 upsert（对齐 Desktop upsertSubagentFromTool）。
func upsertSubagent(state *BuildState, it Item, offset time.Duration) {
	result := parseResultJSON(it.Action.Result)
	sessionID := builderJsonStr(result, "session_id")
	agentName := builderJsonStr(result, "agent_name")
	taskDigest := strParam(it.Action.Params, "task")

	key := subagentKey(sessionID, agentName)
	if key == "" {
		return
	}
	existing, ok := state.subagentNodes[key]
	if ok {
		// 原位补全
		if sessionID != "" && existing.SessionID == "" {
			existing.SessionID = sessionID
		}
		if agentName != "" && existing.AgentName == "" {
			existing.AgentName = agentName
		}
		if taskDigest != "" && existing.TaskDigest == "" {
			existing.TaskDigest = taskDigest
		}
	} else {
		state.subagentNodes[key] = &SubagentNode{
			NodeBase: NodeBase{
				ID:          "subagent_" + key,
				Kind:        "subagent",
				Status:      NodeExecuting,
				Offset:      offset,
				FoldDefault: true,
			},
			AgentName:  agentName,
			TaskDigest: taskDigest,
			SessionID:  sessionID,
		}
	}
}

// buildTeamNode 团队创建卡（对齐 Desktop buildTeamNode）。
func buildTeamNode(state *BuildState, it Item, offset time.Duration) *TeamNode {
	params := it.Action.Params
	teamName := strParam(params, "team_name")
	if teamName == "" {
		return nil
	}
	status := NodeSuccess
	if it.Action.Status == ActionStepFailed {
		status = NodeFailed
	}
	if it.Action.Status == ActionStepExecuting {
		status = NodeExecuting
	}

	members := []TeamMember{}
	if ms, ok := params["members"].([]any); ok {
		for _, v := range ms {
			name := fmt.Sprintf("%v", v)
			if name == "" {
				continue
			}
			members = append(members, TeamMember{Name: name})
		}
	}
	leader := strParam(params, "leader")
	if leader != "" {
		exists := false
		for i, m := range members {
			if m.Name == leader {
				members[i].Role = "leader"
				exists = true
				break
			}
		}
		if !exists {
			members = append([]TeamMember{{Name: leader, Role: "leader"}}, members...)
		}
	}

	return &TeamNode{
		NodeBase: NodeBase{
			ID:          "team_" + it.Action.ToolCallID,
			Kind:        "team",
			Status:      status,
			Offset:      offset,
			FoldDefault: true,
		},
		TeamName:    teamName,
		Description: strParam(params, "description"),
		Members:     members,
	}
}

// buildCollectNode 结果收集卡（对齐 Desktop buildCollectNode）。
func buildCollectNode(state *BuildState, it Item, offset time.Duration) *CollectNode {
	result := parseResultJSON(it.Action.Result)
	if result == nil {
		return nil
	}
	entries, ok := result["results"].([]any)
	if !ok {
		return nil
	}

	sessionIDs := []string{}
	digests := []string{}
	for _, e := range entries {
		entry := e.(map[string]any)
		sid := builderJsonStr(entry, "session_id")
		if sid == "" {
			continue
		}
		sessionIDs = append(sessionIDs, sid)
		success, _ := entry["success"].(bool)
		var digest string
		if success {
			digest = builderJsonStr(entry, "result")
		} else {
			digest = builderJsonStr(entry, "error")
		}
		digests = append(digests, truncate(digest, 200))

		// 关联回填 subagent 卡片完成态（对齐 Desktop）
		if sn, ok := state.subagentNodes[sid]; ok {
			sn.NodeBase.Status = ternary(success, NodeSuccess, NodeFailed)
			sn.ResultDigest = truncate(digest, 200)
		}
	}
	if len(sessionIDs) == 0 {
		return nil
	}

	status := NodeSuccess
	if it.Action.Status == ActionStepFailed {
		status = NodeFailed
	}

	return &CollectNode{
		NodeBase: NodeBase{
			ID:          "collect_" + it.Action.ToolCallID,
			Kind:        "collect",
			Status:      status,
			Offset:      offset,
			Duration:    it.Action.Duration,
			FoldDefault: true,
		},
		SessionIDs:    sessionIDs,
		ResultDigests: digests,
	}
}

// ── 阻塞/系统节点 ──

func buildAskUserNode(it Item, offset time.Duration) *AskUserNode {
	return &AskUserNode{
		NodeBase: NodeBase{
			ID:          fmt.Sprintf("ask_user_%d", offset.Milliseconds()),
			Kind:        "ask_user",
			Status:      NodeExecuting,
			Offset:      offset,
			FoldDefault: true,
		},
	}
}

func buildErrorNode(it Item, offset time.Duration) *ErrorNode {
	source := "runtime"
	switch {
	case contains(it.ErrorData.Error, "timeout") || contains(it.ErrorData.Error, "llm_timeout"):
		source = "llm_timeout"
	case contains(it.ErrorData.Error, "402"):
		source = "provider_402"
	}
	return &ErrorNode{
		NodeBase: NodeBase{
			ID:          fmt.Sprintf("error_%d", offset.Milliseconds()),
			Kind:        "error",
			Status:      NodeFailed,
			Offset:      offset,
			FoldDefault: false,
		},
		Message: truncate(firstLine(it.ErrorData.Error), 200),
		Source:  source,
	}
}

// buildSystemNotice 从 Notice Item 分派到各系统节点类型。
func buildSystemNode(state *BuildState, it Item, offset time.Duration) TreeNode {
	if it.IsRetry {
		return &LlmRetryNode{
			NodeBase: NodeBase{
				ID:          fmt.Sprintf("llm_retry_%d", offset.Milliseconds()),
				Kind:        "llm_retry",
				Status:      NodeSuccess,
				Offset:      offset,
				FoldDefault: true,
			},
			RetryAfter: 0, // 从 Text 里解析
		}
	}
	if it.Text != "" && contains(it.Text, "max_turns") {
		return &MaxTurnsNode{
			NodeBase: NodeBase{
				ID:          fmt.Sprintf("max_turns_%d", offset.Milliseconds()),
				Kind:        "max_turns",
				Status:      NodeSuccess,
				Offset:      offset,
				FoldDefault: true,
			},
		}
	}
	if it.Text != "" && contains(it.Text, "已中断") {
		return &CancelledNode{
			NodeBase: NodeBase{
				ID:          fmt.Sprintf("cancelled_%d", offset.Milliseconds()),
				Kind:        "cancelled",
				Status:      NodeCancelled,
				Offset:      offset,
				FoldDefault: false,
			},
		}
	}
	// 默认 fallback：compaction / permission 等系统事件
	return &MaxTurnsNode{
		NodeBase: NodeBase{
			ID:          fmt.Sprintf("system_%d", offset.Milliseconds()),
			Kind:        "max_turns", // 临时占位，后续细化
			Status:      NodeSuccess,
			Offset:      offset,
			FoldDefault: true,
		},
	}
}

// ═══════════════════════════════════════════════════════════
// 辅助函数
// ═══════════════════════════════════════════════════════════

func subagentKey(sessionID, agentName string) string {
	if sessionID != "" {
		return sessionID
	}
	if agentName != "" {
		return "name:" + agentName
	}
	return ""
}

func parseResultJSON(s string) map[string]any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

func builderJsonStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func jsonResultInt(s, key string) int {
	m := parseResultJSON(s)
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

func builderJsonResultStr(s, key string) string {
	m := parseResultJSON(s)
	return builderJsonStr(m, key)
}

func strParam(params map[string]any, keys ...string) string {
	if params == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := params[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func boolParam(params map[string]any, key string) bool {
	if params == nil {
		return false
	}
	v, _ := params[key].(bool)
	return v
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}

// formatDuration 时长格式化（公共辅助）。
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}
