package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// treeNode is used by "kb chunks tree" to build a directory tree.
type treeNode struct {
	name     string
	id       string
	summary  string
	children map[string]*treeNode
}

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

// ── kb chunks ──────────────────────────────────────────────────

var kbChunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "List or inspect knowledge base chunks (requires daemon)",
	Long: `List, inspect or view the knowledge base chunk directory tree.

Examples:
  mindx kb chunks                      # paginated list (default page 1, size 50)
  mindx kb chunks -p 2 -s 10          # page 2, 10 per page
  mindx kb chunks -r "my-region"      # filter by region
  mindx kb chunks -g                   # global knowledge base only
  mindx kb chunks -i <chunk_id>        # show single chunk as JSON
  mindx kb chunks tree                 # directory tree view
  mindx kb chunks tree -r "my-region"  # tree filtered by region`,
	RunE: func(cmd *cobra.Command, args []string) error {
		chunkID, _ := cmd.Flags().GetString("id")
		page, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("size")
		region, _ := cmd.Flags().GetString("region")
		global, _ := cmd.Flags().GetBool("global")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		// ── Single chunk by ID ──
		if chunkID != "" {
			result, err := cl.KBChunksGet(chunkID)
			if err != nil {
				return err
			}
			var pretty json.RawMessage
			if err := json.Unmarshal(result, &pretty); err != nil {
				fmt.Println(string(result))
				return nil
			}
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}

		// ── Paginated list ──
		var filters []rpc.FilterCondition
		if region != "" {
			filters = append(filters, rpc.FilterCondition{Key: "region", Type: "eq", Value: region})
		}
		if global {
			filters = append(filters, rpc.FilterCondition{Key: "global", Type: "eq", Value: true})
		}

		result, err := cl.KBChunks(page, pageSize, filters...)
		if err != nil {
			return err
		}

		var listResult rpc.MemoryChunksResult
		if err := json.Unmarshal(result, &listResult); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(listResult.Chunks) == 0 {
			fmt.Println("No chunks found.")
			return nil
		}

		table := render.NewTable([]string{"ID", "DocID", "Content Preview"}, 120)
		for _, ch := range listResult.Chunks {
			preview := ch.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			table.AddRow([]string{ch.ID, ch.DocID, preview})
		}
		fmt.Println(table.Render())
		fmt.Printf("\nPage %d / %d per page  |  Total: %d\n", listResult.Page, listResult.PageSize, listResult.Total)
		return nil
	},
}

// ── kb chunks tree ─────────────────────────────────────────────

var kbChunksTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display knowledge base chunks as a directory tree",
	Long: `Display the knowledge base as a directory tree.

Each node shows: filename | chunk-id | summary

Examples:
  mindx kb chunks tree
  mindx kb chunks tree -r "my-region"
  mindx kb chunks tree -g`,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		global, _ := cmd.Flags().GetBool("global")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		var filters []rpc.FilterCondition
		if region != "" {
			filters = append(filters, rpc.FilterCondition{Key: "region", Type: "eq", Value: region})
		}
		if global {
			filters = append(filters, rpc.FilterCondition{Key: "global", Type: "eq", Value: true})
		}

		// Fetch all chunks (use large page size)
		var allChunks []rpc.ChunkItem
		page := 1
		pageSize := 200
		for {
			result, err := cl.KBChunks(page, pageSize, filters...)
			if err != nil {
				return err
			}
			var listResult rpc.MemoryChunksResult
			if err := json.Unmarshal(result, &listResult); err != nil {
				return err
			}
			allChunks = append(allChunks, listResult.Chunks...)
			if !listResult.HasMore || len(listResult.Chunks) == 0 {
				break
			}
			page++
		}

		if len(allChunks) == 0 {
			fmt.Println("No chunks found.")
			return nil
		}

		// Build directory tree from source_file metadata
		root := &treeNode{children: make(map[string]*treeNode)}

		for _, ch := range allChunks {
			srcFile, _ := ch.Metadata["source_file"].(string)
			if srcFile == "" {
				srcFile = ch.DocID
			}
			summary, _ := ch.Metadata["summary"].(string)

			parts := splitPath(srcFile)
			node := root
			for _, part := range parts {
				if node.children[part] == nil {
					node.children[part] = &treeNode{name: part, children: make(map[string]*treeNode)}
				}
				node = node.children[part]
			}
			// Leaf: store chunk info
			node.id = ch.ID
			node.summary = summary
		}

		// Render tree
		printTree(root, "", true)
		fmt.Printf("\n%d chunk(s)\n", len(allChunks))
		return nil
	},
}

// splitPath splits a file path into directory components + filename.
func splitPath(p string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' || p[i] == '\\' {
			if i > start {
				parts = append(parts, p[start:i])
			}
			start = i + 1
		}
	}
	if start < len(p) {
		parts = append(parts, p[start:])
	}
	return parts
}

// printTree recursively prints a directory tree.
func printTree(n *treeNode, prefix string, isRoot bool) {
	if isRoot {
		// Print children of root directly
		keys := sortedKeys(n.children)
		for i, k := range keys {
			last := i == len(keys)-1
			child := n.children[k]
			printNode(child, prefix, last)
		}
		return
	}

	// Print this node's children
	keys := sortedKeys(n.children)
	for i, k := range keys {
		last := i == len(keys)-1
		child := n.children[k]
		printNode(child, prefix, last)
	}
}

func printNode(n *treeNode, prefix string, last bool) {
	var connector, childPrefix string
	if last {
		connector = "└── "
		childPrefix = prefix + "    "
	} else {
		connector = "├── "
		childPrefix = prefix + "│   "
	}

	if len(n.children) > 0 || n.id == "" {
		// Directory node
		fmt.Println(prefix + connector + n.name + "/")
	} else {
		// File node with chunk info
		info := n.name
		if n.id != "" {
			info += " | " + n.id
		}
		if n.summary != "" {
			summary := n.summary
			if len(summary) > 60 {
				summary = summary[:60] + "..."
			}
			info += " | " + summary
		}
		fmt.Println(prefix + connector + info)
	}

	if len(n.children) > 0 {
		keys := sortedKeys(n.children)
		for i, k := range keys {
			lastChild := i == len(keys)-1
			child := n.children[k]
			printNode(child, childPrefix, lastChild)
		}
	}
}

// sortedKeys returns sorted map keys for deterministic tree output.
func sortedKeys(m map[string]*treeNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

	kbChunksCmd.Flags().StringP("id", "i", "", "Chunk ID to show as JSON")
	kbChunksCmd.Flags().IntP("page", "p", 1, "Page number")
	kbChunksCmd.Flags().IntP("size", "s", 50, "Page size")
	kbChunksCmd.Flags().StringP("region", "r", "", "Filter by region")
	kbChunksCmd.Flags().BoolP("global", "g", false, "Global knowledge base only")

	kbChunksTreeCmd.Flags().StringP("region", "r", "", "Filter by region")
	kbChunksTreeCmd.Flags().BoolP("global", "g", false, "Global knowledge base only")

	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbStatsCmd)
	kbCmd.AddCommand(kbSyncCmd)
	kbCmd.AddCommand(kbFileStatesCmd)
	kbCmd.AddCommand(kbIndexCmd)
	kbCmd.AddCommand(kbChunksCmd)
	kbChunksCmd.AddCommand(kbChunksTreeCmd)
}
