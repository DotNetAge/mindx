package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── session parent ────────────────────────────────────────────

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session management (requires daemon)",
	Long: `Create, list, get, and delete agent sessions.

All operations require the daemon to be running (mindx start).

Examples:
  mindx session create --agent "my-agent" --project-dir "/path/to/project"
  mindx session list
  mindx session get --session-id "01ABCDEF..."
  mindx session delete --session-id "01ABCDEF..."`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}

// ── response types (aligned with RPC) ─────────────────────────

type sessionInfo struct {
	SessionID      string    `json:"session_id"`
	AgentName      string    `json:"agent_name,omitempty"`
	Title          string    `json:"title,omitempty"`
	ProjectDir     string    `json:"project_dir,omitempty"`
	SessionDir     string    `json:"session_dir,omitempty"`
	LastActivityAt time.Time `json:"last_activity_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type sessionGetResponse struct {
	SessionID string          `json:"session_id"`
	Messages  json.RawMessage `json:"messages"`
	Meta      json.RawMessage `json:"meta,omitempty"`
}

// ── session create ────────────────────────────────────────────

var sessionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new session",
	Example: `  mindx session create --agent "notes"
  mindx session create --agent "writer" --project-dir "/workspace/docs"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		projectDir, _ := cmd.Flags().GetString("project-dir")
		if agent == "" {
			return fmt.Errorf("--agent is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionCreate(agent, projectDir)
		if err != nil {
			return err
		}

		// Parse response to show session_id prominently
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if sid, ok := resp["session_id"].(string); ok {
				fmt.Printf("Session created: %s\n", sid)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── session list ──────────────────────────────────────────────

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions (optionally filtered by agent)",
	Example: `  mindx session list
  mindx session list --agent "notes"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionList(agent)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var sessions []sessionInfo
		if err := json.Unmarshal(result, &sessions); err != nil {
			// Fallback: raw output
			fmt.Println(string(result))
			return nil
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		table := render.NewTable([]string{"Session ID", "Agent", "Title", "Created"}, 100)
		for _, s := range sessions {
			table.AddRow([]string{
				s.SessionID,
				s.AgentName,
				s.Title,
				s.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d session(s)\n", len(sessions))
		return nil
	},
}

// ── session get ───────────────────────────────────────────────

var sessionGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Get session details and messages by ID",
	Example: `  mindx session get --session-id "01ABCDEFGHJK..."`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionGet(id)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		// Show session summary as table, then messages in raw JSON
		var resp sessionGetResponse
		if err := json.Unmarshal(result, &resp); err == nil {
			fmt.Printf("Session: %s\n", resp.SessionID)

			// Count messages
			var msgs []interface{}
			if resp.Messages != nil {
				_ = json.Unmarshal(resp.Messages, &msgs)
			}
			fmt.Printf("Messages: %d\n", len(msgs))

			// Show meta if present
			if resp.Meta != nil {
				fmt.Printf("Meta: %s\n", string(resp.Meta))
			}

			// Show message table
			if len(msgs) > 0 {
				fmt.Println()
				msgTable := render.NewTable([]string{"#", "Role", "Content"}, 100)
				for i, m := range msgs {
					msg, ok := m.(map[string]interface{})
					if !ok {
						continue
					}
					role, _ := msg["role"].(string)
					content, _ := msg["content"].(string)
					msgTable.AddRow([]string{
						fmt.Sprintf("%d", i+1),
						role,
						content,
					})
				}
				fmt.Println(msgTable.Render())
			}
			return nil
		}

		fmt.Println(string(result))
		return nil
	},
}

// ── session delete ────────────────────────────────────────────

var sessionDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a session by ID",
	Example: `  mindx session delete --session-id "01ABCDEFGHJK..."`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionDelete(id)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── session meta ──────────────────────────────────────────────

var sessionMetaCmd = &cobra.Command{
	Use:     "meta",
	Short:   "Get session metadata by ID",
	Example: `  mindx session meta --session-id "01ABCDEFGHJK..."`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionMeta(id)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── session confirm ───────────────────────────────────────────

var sessionConfirmCmd = &cobra.Command{
	Use:   "confirm",
	Short: "Confirm files for a session",
	Example: `  mindx session confirm --session-id "01ABCDEFGHJK..."
  mindx session confirm --session-id "01ABCDEFGHJK..." --files "file1.go,file2.go"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		filesRaw, _ := cmd.Flags().GetString("files")
		var files []string
		if filesRaw != "" {
			files = splitComma(filesRaw)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionConfirmFiles(id, files)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── session rollback ──────────────────────────────────────────

var sessionRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback files for a session",
	Example: `  mindx session rollback --session-id "01ABCDEFGHJK..."
  mindx session rollback --session-id "01ABCDEFGHJK..." --files "file1.go,file2.go"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		filesRaw, _ := cmd.Flags().GetString("files")
		var files []string
		if filesRaw != "" {
			files = splitComma(filesRaw)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionRollbackFiles(id, files)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── session context ───────────────────────────────────────────

var sessionContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show context window usage for a session",
	Long: `Shows the current context window usage for a session, including
estimated token count, max window size, and usage ratio.

The calculation is consistent with GoHarness's MicroCompact method:
  - Uses DeepSeek token estimation formula (ASCII ≈ 0.3 tok/char, CJK ≈ 0.6 tok/char)
  - Compacted messages count as ~20 tokens (placeholder size)
  - Ratio = window_tokens / max_window_size

Examples:
  mindx session context --session-id "01ABCDEFGHJK..."`,
	Example: `  mindx session context --session-id "01ABCDEFGHJK..."`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.SessionContext(id)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var resp rpc.ContextWindowUsage
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		// Calculate percentage or show N/A
		var usagePct string
		if resp.MaxWindowSize > 0 {
			pct := resp.UsageRatio * 100
			usagePct = fmt.Sprintf("%.1f%%", pct)
		} else {
			usagePct = "N/A (no max window size configured)"
		}

		table := render.NewTable([]string{"Metric", "Value"}, 80)
		table.AddRow([]string{"Window Tokens", fmt.Sprintf("%d", resp.WindowTokens)})
		table.AddRow([]string{"Max Window Size", fmt.Sprintf("%d", resp.MaxWindowSize)})
		table.AddRow([]string{"Usage Ratio", usagePct})
		table.AddRow([]string{"Total Messages", fmt.Sprintf("%d", resp.MessageCount)})
		table.AddRow([]string{"Cursor", fmt.Sprintf("%d", resp.Cursor)})
		table.AddRow([]string{"Active Messages", fmt.Sprintf("%d", resp.ActiveMessageCount)})
		fmt.Println(table.Render())
		return nil
	},
}

// ── session compact ───────────────────────────────────────────

// forceCompactMinTokens 与 goharness session.ForceCompact 的内部下限保持一致
// （session/compaction.go: const forceCompactThreshold int64 = 100_000）。
// 低于该值时服务端直接跳过压缩，CLI 必须提前拦下，避免“看起来压了其实没压”。
const forceCompactMinTokens int64 = 100_000

var sessionCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact a session's context window (LLM summary + cursor slide)",
	Long: `Compact 手动触发一次上下文压缩。

压缩方式为 LLM 摘要式滑动窗口：
  - 旧消息不会被删除，session.yml 保留完整历史
  - 只把 cursor 前移，使活跃窗口从 messages[cursor:] 变为空
  - 摘要经 SummarySink 落盘到项目 .agents/memory/

服务端走 ForceCompact，它有两道跳过条件，CLI 会提前预检并拒绝：
  - 活跃窗口为空（消息已全部滑出）
  - 窗口 tokens 未超过 100,000

压缩会真实调用一次 LLM 生成摘要并消耗 token（约等于把整个窗口再读一遍），
cursor 前移不可逆，故必须显式加 --yes；建议先跑 --dry-run 看占用与预检结论。`,
	Example: `  mindx session compact --session-id "01ABCDEF..." --dry-run
  mindx session compact --session-id "01ABCDEF..." --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("session-id")
		jsonOut, _ := cmd.Flags().GetBool("json")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		assumeYes, _ := cmd.Flags().GetBool("yes")
		memoryOnly, _ := cmd.Flags().GetBool("memory-only")
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		printUsage := func(raw json.RawMessage) {
			var u rpc.ContextWindowUsage
			if json.Unmarshal(raw, &u) != nil {
				fmt.Println(string(raw))
				return
			}
			pct := "N/A"
			if u.MaxWindowSize > 0 {
				pct = fmt.Sprintf("%.1f%%", u.UsageRatio*100)
			}
			table := render.NewTable([]string{"Metric", "Value"}, 80)
			table.AddRow([]string{"Window Tokens", fmt.Sprintf("%d", u.WindowTokens)})
			table.AddRow([]string{"Max Window Size", fmt.Sprintf("%d", u.MaxWindowSize)})
			table.AddRow([]string{"Usage Ratio", pct})
			table.AddRow([]string{"Total Messages", fmt.Sprintf("%d", u.MessageCount)})
			table.AddRow([]string{"Cursor", fmt.Sprintf("%d", u.Cursor)})
			table.AddRow([]string{"Active Messages", fmt.Sprintf("%d", u.ActiveMessageCount)})
			fmt.Println(table.Render())
		}

		// 压缩前快照：占用很低时压缩纯属白烧 token，让用户先看清楚
		var before rpc.ContextWindowUsage
		haveBefore := false
		if raw, err := cl.SessionContext(id); err == nil {
			if json.Unmarshal(raw, &before) == nil {
				haveBefore = true
			}
			if !jsonOut {
				fmt.Println("压缩前：")
				printUsage(raw)
			}
		}

		// 预检：goharness 的 ForceCompact 内部有两道跳过条件——
		// activeMessages 为空、或 windowTokens <= 100K。GUI 端已据此控制按钮可用性，
		// CLI 必须先判掉，否则服务端静默跳过而命令看起来“成功了”。
		//
		// --memory-only 不受 100K 下限约束：它只沉淀记忆、不清窗口，
		// 低占用会话同样值得提取，因此只判空窗口。
		verdict := ""
		if haveBefore {
			switch {
			case before.ActiveMessageCount == 0:
				verdict = fmt.Sprintf("活跃窗口为空（cursor=%d），无可提取内容", before.Cursor)
			case !memoryOnly && before.WindowTokens <= forceCompactMinTokens:
				verdict = fmt.Sprintf("窗口 %d tokens 未超过 ForceCompact 下限 %d，服务端会跳过压缩（如需只提取记忆请加 --memory-only）",
					before.WindowTokens, forceCompactMinTokens)
			}
		}

		if dryRun {
			if verdict != "" {
				fmt.Printf("预检结论：%s\n", verdict)
			} else {
				fmt.Println("预检结论：满足压缩条件，可执行。")
			}
			fmt.Println("--dry-run：仅查看占用，未执行压缩。")
			return nil
		}
		if verdict != "" {
			return fmt.Errorf("未执行压缩：%s", verdict)
		}
		if !assumeYes {
			if memoryOnly {
				return fmt.Errorf("提取记忆会消耗一次 LLM 摘要调用；确认执行请加 --yes")
			}
			return fmt.Errorf("压缩会消耗一次 LLM 摘要调用且 cursor 前移不可逆；确认执行请加 --yes")
		}

		var callErr error
		if memoryOnly {
			_, callErr = cl.SessionExtractMemory(id)
		} else {
			_, callErr = cl.SessionCompact(id)
		}
		if callErr != nil {
			return callErr
		}

		if jsonOut {
			// 操作后重查一次，不依赖返回体的字段完整性
			if after, err := cl.SessionContext(id); err == nil {
				fmt.Println(string(after))
			}
			return nil
		}

		after, err := cl.SessionContext(id)
		if err != nil {
			fmt.Println("操作已完成，但读取后续占用失败：", err)
			return nil
		}
		if memoryOnly {
			fmt.Println("提取后（窗口应保持不变）：")
		} else {
			fmt.Println("压缩后：")
		}
		printUsage(after)

		// 事后校验：压缩时 cursor 未推进 = 服务端仍跳过了，不能报成功。
		// --memory-only 本就要求 cursor 不动，跳过该校验。
		var afterUsage rpc.ContextWindowUsage
		if !memoryOnly && json.Unmarshal(after, &afterUsage) == nil && haveBefore &&
			afterUsage.Cursor == before.Cursor {
			return fmt.Errorf("压缩未生效：cursor 仍为 %d，请检查 daemon 日志中的 ForceCompact 记录", before.Cursor)
		}
		// --memory-only 的等价校验：活跃窗口必须原封不动
		if memoryOnly && haveBefore &&
			json.Unmarshal(after, &afterUsage) == nil &&
			(afterUsage.Cursor != before.Cursor || afterUsage.ActiveMessageCount != before.ActiveMessageCount) {
			return fmt.Errorf("窗口被意外改动：cursor %d→%d，活跃消息 %d→%d",
				before.Cursor, afterUsage.Cursor,
				before.ActiveMessageCount, afterUsage.ActiveMessageCount)
		}
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	sessionCreateCmd.Flags().String("agent", "", "Agent name (required)")
	sessionCreateCmd.Flags().String("project-dir", "", "Project directory for file indexing")
	sessionListCmd.Flags().String("agent", "", "Filter by agent name")
	sessionListCmd.Flags().Bool("json", false, "Output raw JSON")
	sessionGetCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionGetCmd.Flags().Bool("json", false, "Output raw JSON")
	sessionDeleteCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionMetaCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionContextCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionContextCmd.Flags().Bool("json", false, "Output raw JSON")
	sessionConfirmCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionConfirmCmd.Flags().String("files", "", "Comma-separated file paths to confirm")
	sessionRollbackCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionRollbackCmd.Flags().String("files", "", "Comma-separated file paths to rollback")
	sessionCompactCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionCompactCmd.Flags().Bool("json", false, "Output raw JSON")
	sessionCompactCmd.Flags().Bool("dry-run", false, "Only show context usage, do not compact")
	sessionCompactCmd.Flags().Bool("memory-only", false, "Only extract memory, keep context window untouched")
	sessionCompactCmd.Flags().BoolP("yes", "y", false, "Confirm compaction (required; consumes one LLM summary call)")

	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionGetCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionMetaCmd)
	sessionCmd.AddCommand(sessionContextCmd)
	sessionCmd.AddCommand(sessionConfirmCmd)
	sessionCmd.AddCommand(sessionRollbackCmd)
	sessionCmd.AddCommand(sessionCompactCmd)
}
