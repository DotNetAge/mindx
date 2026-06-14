package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── memory parent ─────────────────────────────────────────────

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Long-term memory operations (requires daemon)",
	Long: `Query, store, and manage long-term memory (RAG).

All operations require the daemon to be running (mindx start).

Examples:
  mindx memory query "project architecture"
  mindx memory store --content "Important note" --title "Note" --source "chat"
  mindx memory stats`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(memoryCmd)
}

// ── response type (aligned with RPC) ──────────────────────────

type memoryRecord struct {
	ID        string  `json:"id"`
	SessionID string  `json:"session_id,omitempty"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Score     float64 `json:"score,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// ── memory query ──────────────────────────────────────────────

var memoryQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Semantic search of long-term memory",
	Args:  cobra.MinimumNArgs(1),
	Example: `  mindx memory query "project architecture"
  mindx memory query "API design" --limit 20 --min-score 0.5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		minScore, _ := cmd.Flags().GetFloat64("min-score")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryQuery(args[0], limit, minScore)
		if err != nil {
			return err
		}

		var records []memoryRecord
		if err := json.Unmarshal(result, &records); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(records) == 0 {
			fmt.Println("No matching memory records found.")
			return nil
		}

		table := render.NewTable([]string{"Score", "ID", "Title", "Content"}, 120)
		for _, r := range records {
			score := fmt.Sprintf("%.2f", r.Score)
			table.AddRow([]string{score, truncateStr(r.ID, 12), truncateStr(r.Title, 30), truncateStr(r.Content, 60)})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d record(s)\n", len(records))
		return nil
	},
}

// ── memory store ──────────────────────────────────────────────

var memoryStoreCmd = &cobra.Command{
	Use:     "store",
	Short:   "Store content in long-term memory",
	Example: `  mindx memory store --content "The API uses REST over HTTP" --title "API Design" --source "chat"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		content, _ := cmd.Flags().GetString("content")
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		source, _ := cmd.Flags().GetString("source")
		if content == "" {
			return fmt.Errorf("--content is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryStore(content, title, description, source)
		if err != nil {
			return err
		}

		// Parse response to show ID
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if id, ok := resp["id"].(string); ok {
				fmt.Printf("Memory stored: %s\n", id)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── memory delete ─────────────────────────────────────────────

var memoryDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete memory records by ID",
	Example: `  mindx memory delete --ids '["rec-abc123","rec-def456"]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		idsRaw, _ := cmd.Flags().GetString("ids")
		if idsRaw == "" {
			return fmt.Errorf("--ids is required (JSON array of strings)")
		}
		var ids []string
		if err := json.Unmarshal([]byte(idsRaw), &ids); err != nil {
			return fmt.Errorf("--ids must be valid JSON array: %w", err)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryDelete(ids)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── memory stats ──────────────────────────────────────────────

var memoryStatsCmd = &cobra.Command{
	Use:     "stats",
	Short:   "Show memory store statistics",
	Example: `  mindx memory stats`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryStats()
		if err != nil {
			return err
		}

		// Parse and display as key-value summary
		var stats map[string]interface{}
		if err := json.Unmarshal(result, &stats); err == nil {
			table := render.NewTable([]string{"Metric", "Value"}, 60)
			for k, v := range stats {
				valStr := fmt.Sprintf("%v", v)
				table.AddRow([]string{k, valStr})
			}
			fmt.Println(table.Render())
			return nil
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── memory chunks ─────────────────────────────────────────────

var memoryChunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "List memory chunks with pagination",
	Example: `  mindx memory chunks
  mindx memory chunks --page 2 --page-size 20
  mindx memory chunks --doc-id "doc_abc123"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		page, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		docID, _ := cmd.Flags().GetString("doc-id")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryChunks(page, pageSize, docID)
		if err != nil {
			return err
		}

		var records []memoryRecord
		if err := json.Unmarshal(result, &records); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(records) == 0 {
			fmt.Println("No memory chunks found.")
			return nil
		}

		table := render.NewTable([]string{"ID", "SessionID", "Type", "Title", "Content", "Score", "Created"}, 140)
		for _, r := range records {
			score := fmt.Sprintf("%.2f", r.Score)
			table.AddRow([]string{
				truncateStr(r.ID, 12),
				truncateStr(r.SessionID, 12),
				truncateStr(r.Type, 10),
				truncateStr(r.Title, 24),
				truncateStr(r.Content, 40),
				score,
				r.CreatedAt,
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d chunk(s)\n", len(records))
		return nil
	},
}

// ── memory get-chunks ─────────────────────────────────────────

var memoryGetChunksCmd = &cobra.Command{
	Use:     "get-chunks",
	Short:   "Get chunks by document ID",
	Example: `  mindx memory get-chunks --doc-id "doc_abc123"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if docID == "" {
			return fmt.Errorf("--doc-id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryGetChunks(docID)
		if err != nil {
			return err
		}

		var records []memoryRecord
		if err := json.Unmarshal(result, &records); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(records) == 0 {
			fmt.Println("No chunks found for this document.")
			return nil
		}

		table := render.NewTable([]string{"ID", "SessionID", "Type", "Title", "Content", "Score", "Created"}, 140)
		for _, r := range records {
			score := fmt.Sprintf("%.2f", r.Score)
			table.AddRow([]string{
				truncateStr(r.ID, 12),
				truncateStr(r.SessionID, 12),
				truncateStr(r.Type, 10),
				truncateStr(r.Title, 24),
				truncateStr(r.Content, 40),
				score,
				r.CreatedAt,
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d chunk(s)\n", len(records))
		return nil
	},
}

// ── memory count ──────────────────────────────────────────────

var memoryCountCmd = &cobra.Command{
	Use:     "count",
	Short:   "Count total memory records",
	Example: `  mindx memory count`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryCount()
		if err != nil {
			return err
		}

		// Parse count from response
		var count map[string]interface{}
		if json.Unmarshal(result, &count) == nil {
			if n, ok := count["count"].(float64); ok {
				fmt.Printf("%d\n", int64(n))
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── memory sync ───────────────────────────────────────────────

var memorySyncCmd = &cobra.Command{
	Use:     "sync",
	Short:   "Sync project files into memory",
	Example: `  mindx memory sync --project-dir "/path/to/project"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		if projectDir == "" {
			return fmt.Errorf("--project-dir is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemorySyncProject(projectDir)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── memory file-states ────────────────────────────────────────

var memoryFileStatesCmd = &cobra.Command{
	Use:     "file-states",
	Short:   "Show file sync states for a project",
	Example: `  mindx memory file-states --project-dir "/path/to/project"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		if projectDir == "" {
			return fmt.Errorf("--project-dir is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.MemoryFileStates(projectDir)
		if err != nil {
			return err
		}

		type fileStateRecord struct {
			File  string `json:"file"`
			State string `json:"state"`
		}
		var records []fileStateRecord
		if err := json.Unmarshal(result, &records); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(records) == 0 {
			fmt.Println("No file states found.")
			return nil
		}

		table := render.NewTable([]string{"File", "State"}, 100)
		for _, r := range records {
			table.AddRow([]string{r.File, r.State})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d file(s)\n", len(records))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	memoryQueryCmd.Flags().Int("limit", 10, "Maximum number of results")
	memoryQueryCmd.Flags().Float64("min-score", 0, "Minimum similarity score (0.0 to 1.0)")
	memoryStoreCmd.Flags().String("content", "", "Content to store (required)")
	memoryStoreCmd.Flags().String("title", "", "Title/summary")
	memoryStoreCmd.Flags().String("description", "", "Description")
	memoryStoreCmd.Flags().String("source", "", "Source identifier")
	memoryDeleteCmd.Flags().String("ids", "", "JSON array of record IDs to delete")
	memoryChunksCmd.Flags().Int("page", 1, "Page number")
	memoryChunksCmd.Flags().Int("page-size", 20, "Page size")
	memoryChunksCmd.Flags().String("doc-id", "", "Filter by document ID")
	memoryGetChunksCmd.Flags().String("doc-id", "", "Document ID (required)")
	memorySyncCmd.Flags().String("project-dir", "", "Project directory path (required)")
	memoryFileStatesCmd.Flags().String("project-dir", "", "Project directory path (required)")

	memoryCmd.AddCommand(memoryQueryCmd)
	memoryCmd.AddCommand(memoryStoreCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
	memoryCmd.AddCommand(memoryStatsCmd)
	memoryCmd.AddCommand(memoryChunksCmd)
	memoryCmd.AddCommand(memoryGetChunksCmd)
	memoryCmd.AddCommand(memoryCountCmd)
	memoryCmd.AddCommand(memorySyncCmd)
	memoryCmd.AddCommand(memoryFileStatesCmd)
}
