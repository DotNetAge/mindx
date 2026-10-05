package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── rag parent ────────────────────────────────────────────────

var ragCmd = &cobra.Command{
	Use:   "rag",
	Short: "RAG operations (requires daemon)",
	Long: `Retrieval-augmented generation operations against the daemon.

All operations require the daemon to be running (mindx start).`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(ragCmd)
	ragCmd.AddCommand(ragSegCmd)
}

// ── rag seg ───────────────────────────────────────────────────

var ragSegCmd = &cobra.Command{
	Use:   "seg",
	Short: "Entry semantic index (NativeRAG): add / search / list",
	Long: `Entry semantic index for structured facts — skill registries, tool
catalogs, config entries. Entries are (family, value, meta) triples;
meta key-value pairs become sub-key vectors so a hit on any dimension
recalls the entry. Re-adding the same value overwrites (update).

Data lives in ~/.mindx/data/native.db, owned by the daemon.

Examples:
  mindx rag seg add skill "websearch" --meta '{"name":"websearch","title":"网页搜索"}'
  mindx rag seg search skill "搜索网页"
  mindx rag seg search skill:title "网页"
  mindx rag seg list skill`,
}

// ── rag seg add ───────────────────────────────────────────────

var ragSegAddCmd = &cobra.Command{
	Use:   "add <family> <value>",
	Short: "Add (or overwrite) an entry",
	Example: `  mindx rag seg add skill "websearch" --meta '{"name":"websearch","title":"网页搜索","description":"搜索网络上内容"}'
  mindx rag seg add tool "lint" --meta '{"title":"代码检查"}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		family, value := args[0], args[1]
		metaRaw, _ := cmd.Flags().GetString("meta")

		var meta map[string]string
		if metaRaw != "" {
			if err := json.Unmarshal([]byte(metaRaw), &meta); err != nil {
				return fmt.Errorf("--meta must be a JSON object of string values: %w", err)
			}
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.SegAdd(family, value, meta)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── rag seg search ────────────────────────────────────────────

var ragSegSearchCmd = &cobra.Command{
	Use:   "search <key> <query>",
	Short: "Semantic search over entries (key = family or family:field)",
	Example: `  mindx rag seg search skill "搜索网页"
  mindx rag seg search skill:title "网页" --top-k 3
  mindx rag seg search skill "搜索" --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, query := args[0], args[1]
		topK, _ := cmd.Flags().GetInt("top-k")
		jsonOut, _ := cmd.Flags().GetBool("json")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.SegSearch(key, query, topK)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var wrapped struct {
			Count int `json:"count"`
			Hits  []struct {
				Key   string            `json:"key"`
				Field string            `json:"field"`
				Value string            `json:"value"`
				Meta  map[string]string `json:"meta"`
				Score float32           `json:"score"`
			} `json:"hits"`
		}
		if err := json.Unmarshal(result, &wrapped); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if wrapped.Count == 0 {
			fmt.Printf("No hits for %q.\n", query)
			return nil
		}

		table := render.NewTable([]string{"Value", "Field", "Score", "Key"}, 100)
		for _, h := range wrapped.Hits {
			table.AddRow([]string{h.Value, h.Field, fmt.Sprintf("%.4f", h.Score), h.Key})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d hit(s)\n", wrapped.Count)
		return nil
	},
}

// ── rag seg list ──────────────────────────────────────────────

var ragSegListCmd = &cobra.Command{
	Use:   "list <family>",
	Short: "List entries in a family",
	Example: `  mindx rag seg list skill
  mindx rag seg list tool --limit 20 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		family := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.SegList(family, limit)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var wrapped struct {
			Count int `json:"count"`
			Hits  []struct {
				Key   string            `json:"key"`
				Value string            `json:"value"`
				Meta  map[string]string `json:"meta"`
			} `json:"hits"`
		}
		if err := json.Unmarshal(result, &wrapped); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if wrapped.Count == 0 {
			fmt.Printf("No entries in family %q.\n", family)
			return nil
		}

		table := render.NewTable([]string{"Value", "Meta", "Key"}, 100)
		for _, h := range wrapped.Hits {
			metaJSON, _ := json.Marshal(h.Meta)
			table.AddRow([]string{h.Value, string(metaJSON), h.Key})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d entry(ies)\n", wrapped.Count)
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	ragSegAddCmd.Flags().String("meta", "", "JSON object of string values, each pair becomes a sub-key vector")
	ragSegSearchCmd.Flags().Int("top-k", 1, "Maximum number of hits to return")
	ragSegSearchCmd.Flags().Bool("json", false, "Output raw JSON")
	ragSegListCmd.Flags().Int("limit", 100, "Maximum number of entries to return")
	ragSegListCmd.Flags().Bool("json", false, "Output raw JSON")

	ragSegCmd.AddCommand(ragSegAddCmd)
	ragSegCmd.AddCommand(ragSegSearchCmd)
	ragSegCmd.AddCommand(ragSegListCmd)
}
