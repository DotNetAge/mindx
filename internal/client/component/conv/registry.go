package conv

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/style"
)

// NodePresentation 节点呈现契约（对齐 Desktop NodePresentation）。
//
// TUI 与 Desktop 的唯一差异：
// - Desktop 的 Icon 是 Vue Component（Element Plus 图标），TUI 用 emoji/ASCII 符号；
// - Desktop 的 object/badges 直接内联文本，TUI 走同样的提取逻辑（一一对应）。
//
// 其他字段（Verb / Executing / Attention / Object / Badges / Summarize / Standalone / HasDetail）
// 全部与 Desktop registry/summary.ts 语义等价。
type NodePresentation struct {
	Icon       string                               // 图标符号（Terminal 无 Vue Component → emoji/ASCII）
	Verb       func(TreeNode) string                // 完成态状态动词
	Executing  func(TreeNode) string                // executing 流光文案
	Attention  bool                                 // 静态 attention 类型（成功也着黄色）
	Object     func(TreeNode) string                // 名片对象（精选主体，单行截断）
	Badges     func(TreeNode) []string              // 元信息徽标（空串项过滤）
	Summarize  func(int) string                     // 组头单类型模板
	Standalone bool                                 // 全卡直渲（树壳不套统一名片）
	HasDetail  bool                                 // 有展开态视图
}

// Presentations 32 种类型全量注册表（对齐 Desktop PRESENTATIONS 表，漏类型编译报错由 TreeNodeTypes 运行时校验）。
var Presentations = map[string]NodePresentation{

	// ═══════════════════════════════════════════════════════════
	// 族 1：执行内容
	// ═══════════════════════════════════════════════════════════

	// content 行内 markdown 直渲，不走统一名片。
	// 但 Executing 文案是树尾 pending 行的措辞来源（§3.4）。
	"content": {
		Executing: func(TreeNode) string { return "正在规划下一步" },
		Standalone: true,
	},

	"thinking": {
		Icon:       "💭",
		Verb:       func(TreeNode) string { return "思考" },
		Executing:  func(TreeNode) string { return "思考中" },
		Standalone: true,
	},

	// group 树壳内建（组头 + children 递归），表内占位。
	"group": {
		Icon: "📂",
	},

	// ═══════════════════════════════════════════════════════════
	// 族 2：实体与协作
	// ═══════════════════════════════════════════════════════════

	"task": {
		Icon:      "📋",
		Verb:      func(TreeNode) string { return "" },
		Executing: func(n TreeNode) string {
			if t, ok := n.(*TaskNode); ok && t.ActiveForm != "" {
				return t.ActiveForm
			}
			return "正在更新任务"
		},
		Object: func(n TreeNode) string {
			if t, ok := n.(*TaskNode); ok {
				return truncate(t.Subject, 80)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*TaskNode); ok {
				return []string{taskStatusLabel(t.TaskStatus)}
			}
			return nil
		},
		HasDetail: true,
	},

	"team": {
		Icon:      "👥",
		Verb:      func(TreeNode) string { return "已创建团队" },
		Executing: func(TreeNode) string { return "正在创建团队" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*TeamNode); ok {
				return t.TeamName
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*TeamNode); ok && len(t.Members) > 0 {
				return []string{fmt.Sprintf("%d 名成员", len(t.Members))}
			}
			return nil
		},
		HasDetail: true,
	},

	"subagent": {
		Icon:       "🚀",
		Verb:       func(TreeNode) string { return "执行子任务" },
		Executing:  func(TreeNode) string { return "正在执行子任务" },
		Object: func(n TreeNode) string {
			if s, ok := n.(*SubagentNode); ok {
				return joinNonEmpty(s.AgentName, truncate(s.TaskDigest, 60))
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if s, ok := n.(*SubagentNode); ok && s.Restored {
				return []string{"已恢复"}
			}
			return nil
		},
		Standalone: true,
	},

	"collect": {
		Icon:      "📦",
		Verb:      func(TreeNode) string { return "已收集" },
		Executing: func(TreeNode) string { return "正在收集结果" },
		Object: func(n TreeNode) string {
			if c, ok := n.(*CollectNode); ok {
				return fmt.Sprintf("%d 个子任务结果", len(c.SessionIDs))
			}
			return ""
		},
		HasDetail: true,
	},

	// ═══════════════════════════════════════════════════════════
	// 族 3：阻塞与系统
	// ═══════════════════════════════════════════════════════════

	"permission": {
		Icon:      "🔒",
		Verb: func(n TreeNode) string {
			if p, ok := n.(*PermissionNode); ok {
				switch p.Decision {
				case "denied":
					return "已拒绝"
				case "granted":
					return "已授权"
				default:
					return "等待授权"
				}
			}
			return ""
		},
		Executing: func(TreeNode) string { return "等待授权" },
		Object: func(n TreeNode) string {
			if p, ok := n.(*PermissionNode); ok {
				return joinNonEmpty(p.ToolName, truncate(p.Reason, 60))
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if p, ok := n.(*PermissionNode); ok {
				return []string{p.SecurityLevel}
			}
			return nil
		},
		HasDetail: true,
	},

	"ask_user": {
		Icon:      "💬",
		Verb: func(n TreeNode) string {
			if a, ok := n.(*AskUserNode); ok && len(a.Answers) > 0 {
				return "已回答"
			}
			return "向你提问"
		},
		Executing: func(TreeNode) string { return "等待你的回答" },
		Object: func(n TreeNode) string {
			if a, ok := n.(*AskUserNode); ok {
				if len(a.Answers) > 0 {
					parts := make([]string, 0, len(a.Answers))
					for _, ans := range a.Answers {
						parts = append(parts, ans.Answer)
					}
					return truncate(strings.Join(parts, "、"), 60)
				}
				if len(a.Questions) > 0 {
					return truncate(a.Questions[0].Question, 60)
				}
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if a, ok := n.(*AskUserNode); ok && len(a.Questions) > 1 {
				return []string{fmt.Sprintf("%d 个问题", len(a.Questions))}
			}
			return nil
		},
		HasDetail: true,
	},

	"error": {
		Icon:      "❌",
		Verb:      func(TreeNode) string { return "执行出错" },
		Object: func(n TreeNode) string {
			if e, ok := n.(*ErrorNode); ok {
				return truncate(e.Message, 120)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if e, ok := n.(*ErrorNode); ok {
				switch e.Source {
				case "llm_timeout":
					return []string{"请求超时"}
				case "provider_402":
					return []string{"服务商欠费"}
				}
			}
			return nil
		},
		HasDetail: true,
	},

	"compaction": {
		Icon:       "🔄",
		Verb:       func(TreeNode) string { return "上下文已压缩" },
		Attention:  true,
		Object: func(n TreeNode) string {
			if c, ok := n.(*CompactionNode); ok {
				return fmt.Sprintf("滑动 %d 条消息", c.MessagesSlid)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if c, ok := n.(*CompactionNode); ok && c.RemainingAfter > 0 {
				return []string{formatCompactNumber(c.RemainingAfter) + " tokens"}
			}
			return nil
		},
	},

	"max_turns": {
		Icon:       "⚠️",
		Verb:       func(TreeNode) string { return "已达最大轮数" },
		Attention:  true,
		Object: func(n TreeNode) string {
			if m, ok := n.(*MaxTurnsNode); ok {
				return fmt.Sprintf("%d/%d 轮", m.TurnsCompleted, m.MaxTurns)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if m, ok := n.(*MaxTurnsNode); ok && m.Suggestion != "" {
				return []string{truncate(m.Suggestion, 40)}
			}
			return nil
		},
	},

	"llm_retry": {
		Icon:       "🔁",
		Verb:       func(TreeNode) string { return "请求重试" },
		Executing:  func(TreeNode) string { return "正在重试" },
		Attention:  true,
		Object: func(n TreeNode) string {
			if r, ok := n.(*LlmRetryNode); ok {
				return joinNonEmpty(r.Provider, r.Model)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if r, ok := n.(*LlmRetryNode); ok {
				parts := []string{
					fmt.Sprintf("%d/%d", r.Attempt, r.MaxAttempts),
					ternary(r.StatusCode > 0, fmt.Sprintf("HTTP %d", r.StatusCode), ""),
					ternary(r.RetryAfter > 0, fmt.Sprintf("%s 后重试", formatDuration(r.RetryAfter)), ""),
				}
				return filterEmpty(parts...)
			}
			return nil
		},
	},

	"cancelled": {
		Icon:      "⏹",
		Verb:      func(TreeNode) string { return "已中断" },
		Object:    func(TreeNode) string { return "" },
		Badges: func(n TreeNode) []string {
			if c, ok := n.(*CancelledNode); ok && c.Elapsed > 0 {
				return []string{formatDuration(c.Elapsed)}
			}
			return nil
		},
	},

	// ═══════════════════════════════════════════════════════════
	// 族 4：工具（18 种，逐条对齐 Desktop summary.ts）
	// ═══════════════════════════════════════════════════════════

	"tool.read": {
		Icon:      "📄",
		Verb:      func(TreeNode) string { return "已读取" },
		Executing: func(TreeNode) string { return "正在读取" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return joinNonEmpty(baseName(t.FilePath), ternary(t.ReadLines > 0, fmt.Sprintf("%d 行", t.ReadLines), ""))
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已读取 %d 个文件", c) },
	},

	"tool.write": {
		Icon:      "📝",
		Verb:      func(TreeNode) string { return "已创建" },
		Executing: func(TreeNode) string { return "正在写入" },
		Object:    func(TreeNode) string { return "1 个文件" },
		Summarize: func(c int) string { return fmt.Sprintf("已创建 %d 个文件", c) },
		HasDetail: true,
	},

	"tool.edit": {
		Icon:      "✏️",
		Verb:      func(TreeNode) string { return "已编辑" },
		Executing: func(TreeNode) string { return "正在编辑" },
		Object:    func(TreeNode) string { return "1 个文件" },
		Summarize: func(c int) string { return fmt.Sprintf("已编辑 %d 个文件", c) },
		HasDetail: true,
	},

	"tool.ls": {
		Icon:      "📁",
		Verb:      func(TreeNode) string { return "已列出" },
		Executing: func(TreeNode) string { return "正在浏览目录" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return ternary(t.LsPath != "", baseName(t.LsPath), "")
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(
					ternary(t.LsEntries > 0, fmt.Sprintf("%d 项", t.LsEntries), ""),
					ternary(t.LsRecursive, "递归", ""),
				)
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已列出 %d 个目录", c) },
		HasDetail: true,
	},

	"tool.glob": {
		Icon:      "📂",
		Verb:      func(TreeNode) string { return "已匹配" },
		Executing: func(TreeNode) string { return "正在匹配" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.GlobPattern, 60)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(ternary(t.GlobMatches > 0, fmt.Sprintf("%d 个", t.GlobMatches), ""))
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已匹配 %d 个模式", c) },
		HasDetail: true,
	},

	"tool.grep": {
		Icon:      "🔍",
		Verb:      func(TreeNode) string { return "已搜索" },
		Executing: func(TreeNode) string { return "正在搜索" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return joinNonEmpty(truncate(t.GrepPattern, 60), t.GrepInclude)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(ternary(t.GrepHits > 0, fmt.Sprintf("%d 处", t.GrepHits), ""))
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已搜索 %d 次", c) },
		HasDetail: true,
	},

	"tool.bash": {
		Icon:      "⌘",
		Verb:      func(TreeNode) string { return "命令已执行" },
		Executing: func(TreeNode) string { return "正在执行命令" },
		Object: func(n TreeNode) string {
			if b, ok := n.(*ToolNode); ok {
				return truncate(b.BashCmd, 48)
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已执行 %d 条命令", c) },
		HasDetail: true,
	},

	"tool.run_script": {
		Icon:      "▶️",
		Verb:      func(TreeNode) string { return "已运行脚本" },
		Executing: func(TreeNode) string { return "正在运行脚本" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return t.RunSkillName
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok && t.BashExitCode != 0 {
				return []string{fmt.Sprintf("exit %d", t.BashExitCode)}
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已运行 %d 个脚本", c) },
		HasDetail: true,
	},

	"tool.web_fetch": {
		Icon:      "🔗",
		Verb:      func(TreeNode) string { return "已抓取" },
		Executing: func(TreeNode) string { return "正在阅读网页" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				if t.WebFetchTitle != "" {
					return t.WebFetchTitle
				}
				return hostOf(t.WebFetchURL)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok && t.WebFetchBytes > 0 {
				return []string{formatCompactNumber(t.WebFetchBytes)}
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已抓取 %d 个页面", c) },
		HasDetail: true,
	},

	"tool.web_search": {
		Icon:      "🌐",
		Verb:      func(TreeNode) string { return "已联网搜索" },
		Executing: func(TreeNode) string { return "正在联网搜索" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.WebSearchQuery, 60)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(
					ternary(t.WebSearchCount > 0, fmt.Sprintf("%d 条", t.WebSearchCount), ""),
					ternary(t.WebSearchCached, "缓存", ""),
				)
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已联网搜索 %d 次", c) },
		HasDetail: true,
	},

	"tool.kb_search": {
		Icon:      "📚",
		Verb:      func(TreeNode) string { return "知识库检索" },
		Executing: func(TreeNode) string { return "正在检索知识库" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.KBSearchQuery, 60)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(ternary(t.KBSearchHits > 0, fmt.Sprintf("%d 块", t.KBSearchHits), ""))
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("知识库检索 %d 次", c) },
		HasDetail: true,
	},

	"tool.memory_search": {
		Icon:      "🧠",
		Verb:      func(TreeNode) string { return "记忆检索" },
		Executing: func(TreeNode) string { return "正在检索记忆" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.MemorySearchQuery, 60)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok {
				return filterEmpty(ternary(t.MemorySearchHits > 0, fmt.Sprintf("%d 条", t.MemorySearchHits), ""))
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("记忆检索 %d 次", c) },
		HasDetail: true,
	},

	"tool.skill": {
		Icon:      "✨",
		Verb:      func(TreeNode) string { return "已加载技能" },
		Executing: func(TreeNode) string { return "正在加载技能" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return t.SkillName
			}
			return ""
		},
		// 关键动作不聚合（GroupKey=nil），无 Summarize
		HasDetail: true,
	},

	"tool.sleep": {
		Icon:      "⏱",
		Verb:      func(TreeNode) string { return "等待" },
		Executing: func(TreeNode) string { return "正在等待" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return ternary(t.SleepDuration > 0, formatDuration(t.SleepDuration), "")
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已等待 %d 次", c) },
		// sleep 无展开态视图（对齐 Desktop "—"），HasDetail=false
	},

	"tool.task_query": {
		Icon:      "📋",
		Verb:      func(TreeNode) string { return "已查询任务" },
		Executing: func(TreeNode) string { return "正在查询任务" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.TaskQueryResult, 60)
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已查询任务 %d 次", c) },
		HasDetail: true,
	},

	"tool.team_ops": {
		Icon:      "👥",
		Verb:      func(TreeNode) string { return "团队操作" },
		Executing: func(TreeNode) string { return "正在查询团队" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return joinNonEmpty(teamActionLabel(t.TeamOpsAction), t.TeamOpsTeam, truncate(t.TaskQueryResult, 40))
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已查询团队 %d 次", c) },
		HasDetail: true,
	},

	"tool.cron": {
		Icon:      "⏰",
		Verb:      func(TreeNode) string { return "定时任务" },
		Executing: func(TreeNode) string { return "正在编排定时任务" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return joinNonEmpty(t.CronAction, t.CronID, t.CronAgent, t.CronExpr)
			}
			return ""
		},
		Badges: func(n TreeNode) []string {
			if t, ok := n.(*ToolNode); ok && t.CronEnabled != nil && !*t.CronEnabled {
				return []string{"已停用"}
			}
			return nil
		},
		Summarize: func(c int) string { return fmt.Sprintf("已操作定时任务 %d 次", c) },
		HasDetail: true,
	},

	"tool.notify": {
		Icon:      "🔔",
		Verb:      func(TreeNode) string { return "已发送通知" },
		Executing: func(TreeNode) string { return "正在发送通知" },
		Object: func(n TreeNode) string {
			if t, ok := n.(*ToolNode); ok {
				return truncate(t.NotifyTitle, 60)
			}
			return ""
		},
		Summarize: func(c int) string { return fmt.Sprintf("已发送 %d 条通知", c) },
	},
}

// ═══════════════════════════════════════════════════════════
// 组头兜底模板（混合成员类型时按 GroupKey 措辞，对齐 Desktop GROUP_FALLBACK）
// ═══════════════════════════════════════════════════════════

var GroupFallback = map[GroupKey]func(int) string{
	GroupFSRead:    func(c int) string { return fmt.Sprintf("已读取 %d 个文件", c) },
	GroupFSWrite:   func(c int) string { return fmt.Sprintf("已变更 %d 个文件", c) },
	GroupFSBrowse:  func(c int) string { return fmt.Sprintf("已浏览 %d 项", c) },
	GroupFSSearch:  func(c int) string { return fmt.Sprintf("已搜索 %d 次", c) },
	GroupCmd:       func(c int) string { return fmt.Sprintf("已执行 %d 条命令", c) },
	GroupWebFetch:  func(c int) string { return fmt.Sprintf("已抓取 %d 个页面", c) },
	GroupWebSearch: func(c int) string { return fmt.Sprintf("已联网搜索 %d 次", c) },
	GroupKBSearch:  func(c int) string { return fmt.Sprintf("已检索 %d 次", c) },
	GroupTaskQuery: func(c int) string { return fmt.Sprintf("已查询任务 %d 次", c) },
	GroupTeamQuery: func(c int) string { return fmt.Sprintf("已查询团队 %d 次", c) },
	GroupSys:       func(c int) string { return fmt.Sprintf("已执行 %d 项系统操作", c) },
}

// groupHeaderOf 组头文案生成（对齐 Desktop groupHeaderOf）。
// 单成员类型用该类型 Summarize；混合类型按 GroupKey Fallback。
func groupHeaderOf(g *GroupNode) string {
	if len(g.Children) == 0 {
		return ""
	}
	// 收集成员类型集合
	types := make(map[string]bool)
	var firstKind string
	for i, c := range g.Children {
		k := c.Base().Kind
		types[k] = true
		if i == 0 {
			firstKind = k
		}
	}
	// 单一类型 → 该类型 Summarize
	if len(types) == 1 {
		if pres, ok := Presentations[firstKind]; ok && pres.Summarize != nil {
			return pres.Summarize(len(g.Children))
		}
	}
	// 混合类型 → GroupKey Fallback
	if fallback, ok := GroupFallback[GroupKey(g.GroupKey)]; ok {
		return fallback(len(g.Children))
	}
	return fmt.Sprintf("已执行 %d 项", len(g.Children))
}

// ═══════════════════════════════════════════════════════════
// 辅助函数（对齐 Desktop registry 的工具函数）
// ═══════════════════════════════════════════════════════════

// truncate 单行截断（对齐 Desktop singleLine）。
func truncate(s string, max int) string {
	if s == "" {
		return ""
	}
	if len(s) <= max {
		return s
	}
	// 中文字符按 rune 截断
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// joinNonEmpty 以 " · " 连接非空片段（对齐 Desktop joinObj）。
func joinNonEmpty(parts ...string) string {
	var filtered []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, " · ")
}

// filterEmpty 过滤空串（对齐 Desktop badges 过滤）。
func filterEmpty(parts ...string) []string {
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// ternary Go 版三元表达式。
func ternary[T any](cond bool, t T, f T) T {
	if cond {
		return t
	}
	return f
}

// baseName 取路径最后一段（对齐 Desktop basename）。
func baseName(path string) string {
	return filepath.Base(path)
}

// hostOf URL 取主机名（对齐 Desktop hostOf）。
func hostOf(url string) string {
	// 简单实现：截取 :// 后到 / 前的部分
	if i := strings.Index(url, "://"); i >= 0 {
		rest := url[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return url
}

// taskStatusLabel 任务状态中文标签（对齐 Desktop TASK_STATUS_LABELS）。
var taskStatusLabels = map[string]string{
	"pending":    "待处理",
	"in_progress": "进行中",
	"completed":  "已完成",
	"cancelled":  "已取消",
}

func taskStatusLabel(s string) string {
	if label, ok := taskStatusLabels[s]; ok {
		return label
	}
	return s
}

// teamActionLabel TeamOps 动作中文标签（对齐 Desktop TEAM_ACTION_LABELS）。
func teamActionLabel(action string) string {
	switch action {
	case "delete":
		return "已解散团队"
	case "get_tasks":
		return "已读取团队任务"
	case "list":
		return "已查询团队"
	default:
		return action
	}
}

// formatCompactNumber 大数字压缩显示（对齐 Desktop formatCompactNumber）。
func formatCompactNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}


// ensureStyleImport 确保 style 包被引用（registry 不直接用 style 但 Go 编译器不允许空 import，此处占位）。
var _ = style.ThemeCyan
