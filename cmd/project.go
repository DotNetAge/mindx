package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── project parent ────────────────────────────────────────────

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Project inspection (requires daemon)",
	Long: `Inspect projects discovered from all sessions.

Projects are aggregated by deduplicating session project_dir values across
all working directories (per-project .sessions shards).

  - leader: the team-leader agent appearing as session sponsor in the
    project; empty means the user handles the project personally.
  - last agent / last activity: sponsor and time of the most recent
    session activity in the project.

All operations require the daemon to be running (mindx start).

Examples:
  mindx project list`,
	PersistentPreRunE: requireDaemon,
}

// ── response types (aligned with RPC) ─────────────────────────

type projectInfo struct {
	Name           string    `json:"name"`
	ProjectDir     string    `json:"project_dir"`
	Leader         string    `json:"leader,omitempty"`
	LastAgent      string    `json:"last_agent,omitempty"`
	LastActivityAt time.Time `json:"last_activity_at"`
	SessionCount   int       `json:"session_count"`
}

// displayAgentOrUser 将 Sponsor 投影为显示值：空 = 用户亲自发起。
func displayAgentOrUser(agent string) string {
	if strings.TrimSpace(agent) == "" {
		return "(user)"
	}
	return agent
}

// ── project list ──────────────────────────────────────────────

var projectListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all projects (deduplicated from sessions)",
	Example: `  mindx project list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.ProjectList()
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var projects []projectInfo
		if err := json.Unmarshal(result, &projects); err != nil {
			// Fallback: raw output
			fmt.Println(string(result))
			return nil
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		table := render.NewTable([]string{"Project", "Leader", "Last Agent", "Last Activity", "Sessions"}, 100)
		for _, p := range projects {
			table.AddRow([]string{
				p.Name,
				displayAgentOrUser(p.Leader),
				displayAgentOrUser(p.LastAgent),
				p.LastActivityAt.Format("2006-01-02 15:04"),
				fmt.Sprintf("%d", p.SessionCount),
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d project(s)\n", len(projects))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	projectListCmd.Flags().Bool("json", false, "Output raw JSON")
	projectCmd.AddCommand(projectListCmd)
	rootCmd.AddCommand(projectCmd)
}
