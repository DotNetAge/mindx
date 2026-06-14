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
		defer cl.Close()
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
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.SessionList(agent)
		if err != nil {
			return err
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
				truncateStr(s.SessionID, 12),
				s.AgentName,
				truncateStr(s.Title, 40),
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
		if id == "" {
			return fmt.Errorf("--session-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.SessionGet(id)
		if err != nil {
			return err
		}

		// Show session summary as table, then messages in raw JSON
		var resp sessionGetResponse
		if err := json.Unmarshal(result, &resp); err == nil {
			fmt.Printf("Session: %s\n", resp.SessionID)

			// Count messages
			var msgs []interface{}
			if resp.Messages != nil {
				json.Unmarshal(resp.Messages, &msgs)
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
						truncateStr(content, 60),
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
		defer cl.Close()
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
		defer cl.Close()
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
		defer cl.Close()
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
		defer cl.Close()
		result, err := cl.SessionRollbackFiles(id, files)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── helpers ───────────────────────────────────────────────────

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	sessionCreateCmd.Flags().String("agent", "", "Agent name (required)")
	sessionCreateCmd.Flags().String("project-dir", "", "Project directory for file indexing")
	sessionListCmd.Flags().String("agent", "", "Filter by agent name")
	sessionGetCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionDeleteCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionMetaCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionConfirmCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionConfirmCmd.Flags().String("files", "", "Comma-separated file paths to confirm")
	sessionRollbackCmd.Flags().String("session-id", "", "Session ID (required)")
	sessionRollbackCmd.Flags().String("files", "", "Comma-separated file paths to rollback")

	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionGetCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionMetaCmd)
	sessionCmd.AddCommand(sessionConfirmCmd)
	sessionCmd.AddCommand(sessionRollbackCmd)
}
