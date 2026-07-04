package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── kb parent ──────────────────────────────────────────────────

var kbCmd = &cobra.Command{
	Use:   "kb",
	Short: "Knowledge base operations (requires daemon)",
	Long: `Query and manage the knowledge base (GraphIndexer).

All operations require the daemon to be running (mindx start).

Examples:
  mindx kb search "project architecture"
  mindx kb stats --project-dir "/path/to/project"
  mindx kb sync --project-dir "/path/to/project"
  mindx kb file-states --project-dir "/path/to/project"`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(kbCmd)
}

// ── kb search ──────────────────────────────────────────────────

var kbSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Semantic search of the knowledge base",
	Args:  cobra.MinimumNArgs(1),
	Example: `  mindx kb search "project architecture"
  mindx kb search "API design" --limit 20 --min-score 0.5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		minScore, _ := cmd.Flags().GetFloat64("min-score")
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.KBSearch(args[0], limit, minScore)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		type kbHit struct {
			ID      string  `json:"id"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
			DocID   string  `json:"doc_id,omitempty"`
		}
		var hits []kbHit
		if err := json.Unmarshal(result, &hits); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(hits) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		table := render.NewTable([]string{"ID", "DocID", "Score", "Content"}, 120)
		for _, h := range hits {
			table.AddRow([]string{
				h.ID,
				h.DocID,
				fmt.Sprintf("%.3f", h.Score),
				h.Content,
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d result(s)\n", len(hits))
		return nil
	},
}

// ── kb stats ───────────────────────────────────────────────────

var kbStatsCmd = &cobra.Command{
	Use:     "stats",
	Short:   "Show knowledge base indexing statistics",
	Example: `  mindx kb stats --project-dir "/path/to/project"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if projectDir == "" {
			return fmt.Errorf("--project-dir is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.KBStats(projectDir)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var stats map[string]interface{}
		if err := json.Unmarshal(result, &stats); err == nil {
			table := render.NewTable([]string{"Metric", "Value"}, 60)
			for k, v := range stats {
				table.AddRow([]string{k, fmt.Sprintf("%v", v)})
			}
			fmt.Println(table.Render())
			return nil
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── kb sync ────────────────────────────────────────────────────

var kbSyncCmd = &cobra.Command{
	Use:     "sync",
	Short:   "Sync project files into the knowledge base",
	Example: `  mindx kb sync --project-dir "/path/to/project"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		if projectDir == "" {
			return fmt.Errorf("--project-dir is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.KBSyncProject(projectDir)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── kb file-states ─────────────────────────────────────────────

var kbFileStatesCmd = &cobra.Command{
	Use:     "file-states",
	Short:   "Show file sync states for a project directory",
	Example: `  mindx kb file-states --project-dir "/path/to/project"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if projectDir == "" {
			return fmt.Errorf("--project-dir is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.KBFileStates(projectDir)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		// Response: { states: [...], counts: {...} }
		var resp struct {
			States []struct {
				File  string `json:"path"`
				State string `json:"state"`
			} `json:"states"`
			Counts map[string]int `json:"counts"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		// Show counts summary
		if resp.Counts != nil {
			fmt.Println("── File State Summary ──")
			ct := render.NewTable([]string{"State", "Count"}, 30)
			for _, s := range []string{"indexed", "changed", "new", "removed", "skipped", "total"} {
				if c, ok := resp.Counts[s]; ok {
					ct.AddRow([]string{s, fmt.Sprintf("%d", c)})
				}
			}
			fmt.Println(ct.Render())
		}

		// Show individual file states
		if len(resp.States) == 0 {
			fmt.Println("No file states found.")
			return nil
		}
		fmt.Println("\n── File States ──")
		table := render.NewTable([]string{"File", "State"}, 100)
		for _, s := range resp.States {
			table.AddRow([]string{s.File, s.State})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d file(s)\n", len(resp.States))
		return nil
	},
}

// ── kb index ──────────────────────────────────────────────────

var kbIndexCmd = &cobra.Command{
	Use:   "index <path>",
	Short: "Index a single file or directory into the knowledge base",
	Long: `Index a file or directory without re-indexing the entire project.

Without --force, skips files that are already indexed (cache hit).
With --force, clears the cache entry and re-indexes from scratch.

Examples:
  mindx kb index path/to/file.md
  mindx kb index path/to/dir
  mindx kb index --force path/to/file.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.KBIndex(args[0], force)
		if err != nil {
			return err
		}

		var resp struct {
			Status string `json:"status"`
			Path   string `json:"path"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		fmt.Printf("[%s] %s (%s)\n", resp.Status, resp.Path, resp.Type)
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	kbSearchCmd.Flags().Int("limit", 10, "Maximum number of results")
	kbSearchCmd.Flags().Float64("min-score", 0, "Minimum similarity score (0.0 to 1.0)")
	kbSearchCmd.Flags().Bool("json", false, "Output raw JSON")
	kbStatsCmd.Flags().String("project-dir", "", "Project directory path (required)")
	kbStatsCmd.Flags().Bool("json", false, "Output raw JSON")
	kbSyncCmd.Flags().String("project-dir", "", "Project directory path (required)")
	kbFileStatesCmd.Flags().String("project-dir", "", "Project directory path (required)")
	kbFileStatesCmd.Flags().Bool("json", false, "Output raw JSON")
	kbIndexCmd.Flags().Bool("force", false, "Force re-index even if already cached")

	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbStatsCmd)
	kbCmd.AddCommand(kbSyncCmd)
	kbCmd.AddCommand(kbFileStatesCmd)
	kbCmd.AddCommand(kbIndexCmd)
}
