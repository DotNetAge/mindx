package conv

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

// ═══════════════════════════════════════════════════════════
// 树状对话流渲染器（对齐 Desktop TreeView.vue + NodeCard.vue）
//
// 核心视觉元素：
//   1. 树缩进导轨 ─── 顶层节点无缩进；group 成员缩进 + 子树竖线
//   2. 统一名片 ──── icon + 状态动词 + 对象摘要 + 徽标 + 时长阈值（>2s 才显）
//   3. Gap 标记 ──── 相邻节点 startedAt 差值 > 30s 时显示 ⏸ 暂停
//   4. 流光文字 ──── executing 节点的状态动词/executing 文案走 Shimmer 渲染
//   5. 四态着色 ──── executing 青色流光 / success 中性灰 / attention 黄 / failed 红
//   6. 折叠策略 ──── FoldState 三态（默认折叠 + 执行中自动展开 + 手动覆盖）
//   7. 树尾 pending ─ 轮级 executing 但无 executing 节点 = LLM 建流空窗
//
// 与旧 ViewStream 的差异：
//   旧 ViewStream 是线性 Item 列表 + 终态整体折叠（ctrl+o）；
//   新 ViewTree 是节点级树状结构 + 分组聚合 + 节点级折叠 + 流光动画。
// ═══════════════════════════════════════════════════════════

// 树缩进字符（Unicode box-drawing）。
const (
	treePipe   = "│ "   // 竖线导轨（贯穿子树）
	treeBranch = "├── " // 中间节点分支
	treeLast   = "└── " // 最后节点分支
	treeIndent = "    " // 纯缩进（最后节点的下一层不再显示竖线）
)

// 时长徽标阈值（对齐 Desktop DURATION_BADGE_MS = 2000）。
const durationBadgeThreshold = 2 * time.Second

// Gap 标记阈值（对齐 Desktop GAP_THRESHOLD_MS = 30_000）。
const gapThreshold = 30 * time.Second

// ── 主入口 ──

// ViewTree 渲染整条会话流的树状结构。
// width 为可用列宽。先懒构建 Nodes。
func ViewTree(s *Stream, width int) string {
	s.EnsureNodes()
	if s.Nodes == nil || len(s.Nodes) == 0 {
		return ""
	}

	var b strings.Builder
	var prevEnd time.Duration

	for i, node := range s.Nodes {
		offset := node.Base().Offset
		duration := node.Base().Duration

		// 折叠跳过：顶层 GroupNode/FoldDefault 节点走 FoldState 判定
		// Standalone 节点（content/thinking/subagent）始终渲染
		pres := Presentations[node.Base().Kind]
		if !pres.Standalone && !isExpanded(s, node) {
			// 折叠态：顶层输出单行摘要（对齐 Desktop GroupNode 的折叠态展示）
			// GroupNode: 输出组头摘要行（▶ 已读取 2 个文件 ─ 5.1s）
			if IsGroupNode(node) {
				if summary := renderGroupSummary(s, node.(*GroupNode)); summary != "" {
					if i > 0 && offset > prevEnd+gapThreshold {
						b.WriteString(renderGap(offset - prevEnd))
						b.WriteByte('\n')
					}
					b.WriteString(summary)
					prevEnd = offset + duration
				}
			} else {
				// 独立 ToolNode：输出单行卡片（状态动词 + 对象 + 时长）
				if i > 0 && offset > prevEnd+gapThreshold {
					b.WriteString(renderGap(offset - prevEnd))
					b.WriteByte('\n')
				}
				b.WriteString(renderCard(s, node, width))
				prevEnd = offset + duration
			}
			continue
		}

		// gap 标记
		if i > 0 && offset > prevEnd+gapThreshold {
			b.WriteString(renderGap(offset - prevEnd))
			b.WriteByte('\n')
		}

		if pres.Standalone {
			b.WriteString(renderStandalone(s, node, width))
		} else if IsGroupNode(node) {
			b.WriteString(renderGroup(s, node.(*GroupNode), width))
		} else {
			b.WriteString(renderCard(s, node, width))
		}

		prevEnd = offset + duration
	}

	// 树尾 pending 行（对齐 Desktop §3.4）
	if isRoundExecuting(s) && !hasExecutingNode(s.Nodes) {
		b.WriteString(renderPendingRow(s))
	}

	return b.String()
}

// ── 统一名片 ──

// renderCard 渲染统一名片（非 standalone 节点）。
//
// 名片行格式：[icon] [状态动词流光/静态] [对象]  [徽标1] [徽标2]  [时长 >2s]
// 缩进由 renderGroup 传入 indent/isLast 处理（顶层节点无缩进）。
func renderCard(s *Stream, node TreeNode, width int, indentParts ...string) string {
	pres, ok := Presentations[node.Base().Kind]
	if !ok {
		return ""
	}

	status := classifyNodeStatus(node)
	b := node.Base()
	var sb strings.Builder

	// icon
	sb.WriteString(status.IconStyle.Render(pres.Icon))
	sb.WriteString(" ")

	// 状态文案：executing → 流光，否则静态
	if b.Status == NodeExecuting && pres.Executing != nil {
		// executing 流光文案（优先）
		sb.WriteString(renderShimmeredText(s, b.ID, pres.Executing(node), status.VerbStyle))
		sb.WriteString(" ")
	} else if pres.Verb != nil {
		verb := pres.Verb(node)
		if verb != "" {
			sb.WriteString(status.VerbStyle.Render(verb))
			sb.WriteString(" ")
		}
	}

	// 对象摘要
	if pres.Object != nil {
		obj := pres.Object(node)
		if obj != "" {
			sb.WriteString(status.ObjectStyle.Render(obj))
		}
	}

	// 徽标
	if pres.Badges != nil {
		for _, badge := range pres.Badges(node) {
			if badge == "" {
				continue
			}
			sb.WriteString("  ")
			sb.WriteString(style.DimStyle.Render(badge))
		}
	}

	// 时长徽标（阈值化）
	if b.Duration > durationBadgeThreshold {
		sb.WriteString(" ")
		sb.WriteString(style.GrayStyle.Render("| " + formatDuration(b.Duration)))
	}

	sb.WriteByte('\n')

	// 展开态视图
	if isExpanded(s, node) && pres.HasDetail {
		sb.WriteString(renderDetail(node, width))
	}

	return sb.String()
}

// ── GroupNode ──

// renderGroupSummary GroupNode 折叠态单行摘要（对齐 Desktop 折叠态组头展示）。
func renderGroupSummary(s *Stream, g *GroupNode) string {
	status := classifyNodeStatus(g)
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(style.DimStyle.Render("▶ "))
	headerText := groupHeaderOf(g)
	b.WriteString(status.VerbStyle.Render(headerText))
	if g.Base().Duration > durationBadgeThreshold {
		b.WriteString(" ")
		b.WriteString(style.GrayStyle.Render("─ " + formatDuration(g.Base().Duration)))
	}
	b.WriteByte('\n')
	return b.String()
}

func renderGroup(s *Stream, g *GroupNode, width int) string {
	status := classifyNodeStatus(g)
	var b strings.Builder

	// 组头行
	headerText := groupHeaderOf(g)
	if g.Base().Status == NodeExecuting {
		headerText = renderShimmeredText(s, g.Base().ID, headerText, status.VerbStyle)
	} else {
		headerText = status.VerbStyle.Render(headerText)
	}

	b.WriteString("  ")
	// 展开/折叠箭头
	if isExpanded(s, g) {
		b.WriteString(style.DimStyle.Render("▼ "))
	} else {
		b.WriteString(style.DimStyle.Render("▶ "))
	}
	b.WriteString(headerText)

	// 时长
	if g.Base().Duration > durationBadgeThreshold {
		b.WriteString(" ")
		b.WriteString(style.GrayStyle.Render("─ " + formatDuration(g.Base().Duration)))
	}
	b.WriteByte('\n')

	// 展开态成员
	if isExpanded(s, g) {
		for i, child := range g.Children {
			isLast := i == len(g.Children)-1
			b.WriteString(renderGroupMember(s, child, width, isLast))
		}
	}

	return b.String()
}

// renderGroupMember 渲染组内工具节点（带树缩进）。
func renderGroupMember(s *Stream, node TreeNode, width int, isLast bool) string {
	var prefix string
	if isLast {
		prefix = "  " + treeLast
	} else {
		prefix = "  " + treeBranch
	}

	// 渲染统一名片，缩进前缀先加
	card := renderCard(s, node, width-len(prefix))
	if card == "" {
		return ""
	}

	// 每行加前缀（名片一行 + 展开态可能多行）
	lines := strings.Split(strings.TrimRight(card, "\n"), "\n")
	var result strings.Builder
	for i, line := range lines {
		if i == 0 {
			result.WriteString(prefix)
		} else {
			// 展开态续行：保持与缩进对齐的垂直空间
			indent := "  " + treeIndent + strings.Repeat(" ", len(treeBranch))
			result.WriteString(indent)
		}
		result.WriteString(line)
		result.WriteByte('\n')
	}
	return result.String()
}

// ── Standalone 节点 ──

func renderStandalone(s *Stream, node TreeNode, width int) string {
	switch n := node.(type) {
	case *ContentNode:
		return renderContentNode(s, n, width)
	case *ThinkingNode:
		return renderThinkingNode(s, n, width)
	case *SubagentNode:
		return renderSubagentNode(s, n, width)
	default:
		return renderCard(s, node, width)
	}
}

func renderContentNode(s *Stream, n *ContentNode, width int) string {
	if n.Text == "" {
		return ""
	}
	var b strings.Builder

	// streaming 状态：流光前缀（对齐 Desktop content 行前的 LLM 状态提示）
	if n.Status == NodeExecuting && !n.IsFinal {
		prefix := renderShimmeredText(s, n.Base().ID, "⏺ ", lipgloss.NewStyle())
		b.WriteString(prefix)
	} else if n.IsFinal {
		b.WriteString(style.GreenStyle.Render("✓ "))
	} else {
		b.WriteString(style.GrayStyle.Render("  "))
	}

	b.WriteString(n.Text)
	b.WriteByte('\n')

	// 结束态信息
	if n.IsFinal && n.TurnUsage != nil {
		b.WriteString(style.DimStyle.Render(fmt.Sprintf("  cost=%s  tokens=%d",
			formatCompact(int(n.TurnUsage.ActualTokens)), n.TurnUsage.TotalTokens)))
		b.WriteByte('\n')
	}

	return b.String()
}

func renderThinkingNode(s *Stream, n *ThinkingNode, width int) string {
	if n.Text == "" && n.Status != NodeExecuting {
		return ""
	}
	var b strings.Builder

	// icon + 状态文案
	icon := style.PurpleStyle.Render("💭")
	b.WriteString("  ")
	b.WriteString(icon)
	b.WriteString(" ")

	if n.Status == NodeExecuting {
		b.WriteString(renderShimmeredText(s, n.Base().ID, "思考中", style.PurpleStyle))
	} else {
		b.WriteString(style.PurpleStyle.Render("思考"))
	}

	if n.Duration > 0 {
		b.WriteString(" ")
		b.WriteString(style.DimStyle.Render("(" + formatDuration(n.Duration) + ")"))
	}
	b.WriteByte('\n')

	// 思考内容（折叠默认开启，执行中展开）
	if isExpanded(s, n) || n.Status == NodeExecuting {
		text := n.Text
		if n.Status != NodeExecuting {
			// 完成态用斜体暗色
			b.WriteString("  ")
			b.WriteString(style.DarkStyle.Render(truncate(text, width-4)))
		} else {
			b.WriteString("  ")
			b.WriteString(style.DimStyle.Render(truncate(text, width-4)))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func renderSubagentNode(s *Stream, n *SubagentNode, width int) string {
	status := classifyNodeStatus(n)
	var b strings.Builder

	b.WriteString("  ")
	b.WriteString(status.IconStyle.Render("🚀"))
	b.WriteString(" ")

	if n.Status == NodeExecuting {
		b.WriteString(renderShimmeredText(s, n.Base().ID, "正在执行子任务", status.VerbStyle))
	} else if n.Status == NodeSuccess {
		b.WriteString(style.GrayStyle.Render("✓"))
		b.WriteString(" ")
	} else {
		b.WriteString(status.VerbStyle.Render("子任务"))
		b.WriteString(" ")
	}

	// agent name + task digest
	var parts []string
	if n.AgentName != "" {
		parts = append(parts, n.AgentName)
	}
	if n.TaskDigest != "" {
		parts = append(parts, truncate(n.TaskDigest, 40))
	}
	b.WriteString(status.ObjectStyle.Render(strings.Join(parts, " · ")))

	// restored 徽标
	if n.Restored {
		b.WriteString(" ")
		b.WriteString(style.YellowStyle.Render("【已恢复】"))
	}

	// 时长
	if n.Duration > durationBadgeThreshold {
		b.WriteString(" ")
		b.WriteString(style.GrayStyle.Render("| " + formatDuration(n.Duration)))
	}
	b.WriteByte('\n')

	return b.String()
}

// ── 系统/阻塞节点 ──

func renderGap(gap time.Duration) string {
	return style.DarkStyle.Render(fmt.Sprintf("  ⏸ 暂停 %s", formatDuration(gap)))
}

func renderPendingRow(s *Stream) string {
	text := "⏺ 正在规划下一步…"
	sh := s.Shimmers["__pending__"]
	if sh == nil {
		sh = NewShimmer(text)
		s.Shimmers["__pending__"] = sh
	}
	return "  " + sh.Render() + "\n"
}

// ── 展开态详情 ──

func renderDetail(node TreeNode, width int) string {
	// ToolNode 展开态：显示完整命令/结果
	if t, ok := node.(*ToolNode); ok {
		var resultLines []string
		if t.ResultTail != "" {
			resultLines = strings.Split(t.ResultTail, "\n")
		} else {
			resultLines = strings.Split(t.ResultText, "\n")
		}

		var b strings.Builder
		indent := "      "
		for i, line := range resultLines {
			if i >= 40 { // 最多显示 40 行展开态
				b.WriteString(style.DimStyle.Render(fmt.Sprintf("%s… (truncated)", indent)))
				b.WriteByte('\n')
				break
			}
			b.WriteString(style.DimStyle.Render(indent + truncate(line, width-len(indent))))
			b.WriteByte('\n')
		}
		return b.String()
	}

	// ErrorNode 展开态
	if e, ok := node.(*ErrorNode); ok {
		return style.RedStyle.Render(fmt.Sprintf("    %s\n", e.Message))
	}

	return ""
}

// ═══════════════════════════════════════════════════════════
// 状态着色（对齐 Desktop NodeCard.vue 四态逻辑）
// ═══════════════════════════════════════════════════════════

type NodeStatusClass struct {
	Name        string
	IconStyle   lipgloss.Style
	VerbStyle   lipgloss.Style
	ObjectStyle lipgloss.Style
}

func classifyNodeStatus(node TreeNode) NodeStatusClass {
	b := node.Base()
	pres := Presentations[b.Kind]

	switch b.Status {
	case NodeExecuting:
		return NodeStatusClass{
			Name:        "executing",
			IconStyle:   style.CyanStyle,
			VerbStyle:   style.BoldCyan,
			ObjectStyle: style.CyanStyle,
		}
	case NodeFailed:
		return NodeStatusClass{
			Name:        "failed",
			IconStyle:   style.RedStyle,
			VerbStyle:   style.RedStyle.Bold(true),
			ObjectStyle: style.WhiteStyle,
		}
	case NodeCancelled:
		return NodeStatusClass{
			Name:        "cancelled",
			IconStyle:   style.GrayStyle,
			VerbStyle:   style.GrayStyle,
			ObjectStyle: style.DimStyle,
		}
	}

	// success：静态 attention 类型 + 慢节点黄灯
	isAttention := false
	if pres.Attention {
		isAttention = true
	}
	if b.Duration > 30*time.Second {
		isAttention = true
	}

	if isAttention {
		return NodeStatusClass{
			Name:        "attention",
			IconStyle:   style.YellowStyle,
			VerbStyle:   style.YellowStyle,
			ObjectStyle: style.WhiteStyle,
		}
	}

	// 默认 success：中性灰
	return NodeStatusClass{
		Name:        "success",
		IconStyle:   style.GrayStyle,
		VerbStyle:   style.DimStyle,
		ObjectStyle: style.WhiteStyle,
	}
}

// ═══════════════════════════════════════════════════════════
// 辅助
// ═══════════════════════════════════════════════════════════

func isExpanded(s *Stream, node TreeNode) bool {
	if s.FoldState == nil {
		return !node.Base().FoldDefault
	}
	return s.FoldState.Expanded(node)
}

func isRoundExecuting(s *Stream) bool {
	return s.Status == StatusThinking || s.Status == StatusExecuting || s.Status == StatusResponding
}

func hasExecutingNode(nodes []TreeNode) bool {
	for _, n := range nodes {
		if n.Base().Status == NodeExecuting {
			return true
		}
	}
	return false
}

// renderShimmeredText 渲染流光文字。优先用 Stream.Shimmer 中已有的状态；
// 没有则新建 Shimmer 推进渲染一次。
func renderShimmeredText(s *Stream, nodeID, text string, baseStyle lipgloss.Style) string {
	if s.Shimmers == nil {
		return text
	}
	sh, ok := s.Shimmers[nodeID]
	if !ok {
		sh = NewShimmer(text)
		s.Shimmers[nodeID] = sh
	}
	return sh.Render()
}

// formatCompact 大数字压缩（tokens 显示用）。
func formatCompact(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
