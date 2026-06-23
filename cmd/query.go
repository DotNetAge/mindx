package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/spf13/cobra"
)

var queryFlags struct {
	limit    int
	minScore float64
}

var queryCmd = &cobra.Command{
	Use:   "query <search terms>",
	Short: "Search long-term memory",
	Long: `Search the MindX long-term memory store and return matching records.

Requires an embedder model to be configured (see 'mindx doctor').
Searches by semantic similarity to the provided search terms.

Examples:
  mindx query "project architecture"
  mindx query -n 20 "API design decisions"
  mindx query --min-score 0.5 "database schema"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().IntVarP(&queryFlags.limit, "limit", "n", 10, "Maximum number of results to return")
	queryCmd.Flags().Float64Var(&queryFlags.minScore, "min-score", 0, "Minimum similarity score (0.0 to 1.0)")
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	if !core.WorkspaceExists(workspaceDir) {
		return fmt.Errorf("MindX workspace not found at %s\nRun 'mindx' or 'mindx doctor' to initialize", workspaceDir)
	}

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	if !cfg.HasEmbedder() {
		return fmt.Errorf("no embedder model configured — run 'mindx doctor' to set one up")
	}

	modelPath := cfg.EmbedderModelPath(workspaceDir)
	if _, statErr := os.Stat(modelPath); statErr != nil {
		return fmt.Errorf("embedder model file not found at %s\nRun 'mindx doctor' to download it", modelPath)
	}

	emb, err := memory.NewEmbedderFromConfig(modelPath)
	if err != nil {
		return fmt.Errorf("cannot create embedder: %w", err)
	}
	if emb == nil {
		return fmt.Errorf("embedder model could not be loaded from %s", modelPath)
	}

	memDir := filepath.Join(workspaceDir, "memory")
	if _, statErr := os.Stat(memDir); statErr != nil {
		return fmt.Errorf("memory store not found at %s\nNo long-term memory has been stored yet", memDir)
	}

	logDir := filepath.Join(workspaceDir, "logs")
	if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
		return fmt.Errorf("cannot create log directory: %w", mkErr)
	}
	queryLogger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(logDir, "mindx.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    false,
	})

	mem, err := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
		AgentName: "_shared",
		MemoryDir: memDir,
		Embedder:  emb,
		Logger:    queryLogger,
	})
	if err != nil {
		return fmt.Errorf("cannot open memory store: %w", err)
	}
	defer func() {
		if cerr := mem.Close(context.Background()); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: memory close error: %v\n", cerr)
		}
	}()

	query := strings.Join(args, " ")
	opts := []goharnessmemory.RetrieveOption{
		goharnessmemory.WithMemoryLimit(queryFlags.limit),
		goharnessmemory.WithMemoryTypes(goharnessmemory.MemoryTypeLongTerm),
	}
	if queryFlags.minScore > 0 {
		opts = append(opts, goharnessmemory.WithMinScore(queryFlags.minScore))
	}

	start := time.Now()
	records, err := mem.Retrieve(context.Background(), query, opts...)
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("memory query failed: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No matching memory records found.")
		return nil
	}

	// Render table using lipgloss-styled table (bubble ecosystem)
	table := render.NewTable([]string{"Summary", "Content", "Tags"}, 120)
	for _, r := range records {
		summary := r.Summary
		if summary == "" {
			summary = "(untitled)"
		}
		content := truncateText(r.Content, 80)
		tags := strings.Join(r.Tags, ", ")
		table.AddRow([]string{summary, content, tags})
	}
	fmt.Println(table.Render())
	fmt.Printf("\n%d record(s) found in %s.\n", len(records), elapsed.Round(time.Millisecond))

	return nil
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
