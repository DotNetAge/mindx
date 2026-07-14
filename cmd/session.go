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

	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionGetCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionMetaCmd)
	sessionCmd.AddCommand(sessionContextCmd)
	sessionCmd.AddCommand(sessionConfirmCmd)
	sessionCmd.AddCommand(sessionRollbackCmd)
}
