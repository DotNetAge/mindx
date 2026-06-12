package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"charm.land/bubbles/v2/table"
	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── model parent ───────────────────────────────────────────────

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage LLM models",
	Long: `List, add, or remove LLM models.

Models define the specific language models available for use.
See 'mindx model list' to view configured models.`,
}

// ── model list ─────────────────────────────────────────────────

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured models",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := modelsFilePath()
		registry, err := goharnessconfig.LoadModels(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No models configured yet.")
				return nil
			}
			return fmt.Errorf("cannot load models: %w", err)
		}

		models := registry.List()
		if len(models) == 0 {
			fmt.Println("No models configured.")
			return nil
		}

		cols := []table.Column{
			{Title: "Name", Width: 28},
			{Title: "Provider", Width: 16},
			{Title: "Context", Width: 10},
			{Title: "Max Tokens", Width: 12},
			{Title: "Func", Width: 6},
			{Title: "Enabled", Width: 8},
		}

		rows := make([]table.Row, 0, len(models))
		for _, m := range models {
			fc := ""
			if m.FuncCalling {
				fc = "✓"
			}
			en := ""
			if m.Enabled {
				en = "✓"
			}
			name := m.Title
			if name == "" {
				name = m.Name
			}
			provTitle := m.Provider
			if prov := registry.GetProvider(m.Provider); prov != nil && prov.Title != "" {
				provTitle = prov.Title
			}
			ctx := formatInt(m.ContextLength)
			maxTok := formatInt(m.MaxTokens)
			rows = append(rows, table.Row{name, provTitle, ctx, maxTok, fc, en})
		}

		tbl := table.New(
			table.WithColumns(cols),
			table.WithRows(rows),
			table.WithHeight(len(rows)+1),
			table.WithWidth(80),
		)
		fmt.Println(tbl.View())
		fmt.Printf("%d model(s) configured.\n", len(models))
		return nil
	},
}

// ── model rm ───────────────────────────────────────────────────

var modelRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		path := modelsFilePath()

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("model %q not found", name)
			}
			return fmt.Errorf("cannot read models file: %w", err)
		}

		var cfg goharnessconfig.ModelsConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("cannot parse models file: %w", err)
		}

		found := false
		filtered := cfg.Models[:0]
		for _, m := range cfg.Models {
			if m.Name == name {
				found = true
				continue
			}
			filtered = append(filtered, m)
		}
		if !found {
			return fmt.Errorf("model %q not found", name)
		}
		cfg.Models = filtered

		out, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("cannot marshal models: %w", err)
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			return fmt.Errorf("cannot write models file: %w", err)
		}

		fmt.Printf("Model %q removed.\n", name)
		return nil
	},
}

// ── model add ──────────────────────────────────────────────────

var modelAddFlags struct {
	name              string
	title             string
	provider          string
	contextLength     int64
	maxTokens         int64
	enabled           bool
	funcCalling       bool
	webSearching      bool
	temperature       float64
	topP              float64
	repetitionPenalty float64
}

var modelAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update a model",
	Long: `Add a new model or update an existing one.

The model must reference an existing provider (see 'mindx provider list').

Examples:
  mindx model add --name gpt-4 --title "GPT-4" --provider openai --context-length 8192 --max-tokens 4096
  mindx model add --name my-model --provider ollama --context-length 4096 --func-calling`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if modelAddFlags.name == "" {
			return fmt.Errorf("--name is required")
		}
		if modelAddFlags.provider == "" {
			return fmt.Errorf("--provider is required")
		}

		path := modelsFilePath()
		registry, err := goharnessconfig.LoadModels(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot load models: %w", err)
		}

		// Check the provider exists
		existing := registry.GetRaw(modelAddFlags.name)
		model := goharnessconfig.ModelConfig{
			Name:              modelAddFlags.name,
			Title:             modelAddFlags.title,
			Provider:          modelAddFlags.provider,
			ContextLength:     modelAddFlags.contextLength,
			MaxTokens:         modelAddFlags.maxTokens,
			Enabled:           modelAddFlags.enabled,
			FuncCalling:       modelAddFlags.funcCalling,
			WebSearching:      modelAddFlags.webSearching,
			Temperature:       modelAddFlags.temperature,
			TopP:              modelAddFlags.topP,
			RepetitionPenalty: modelAddFlags.repetitionPenalty,
		}
		if existing != nil {
			// Preserve fields not covered by flags
			if model.Title == "" {
				model.Title = existing.Title
			}
			if model.ContextLength == 0 {
				model.ContextLength = existing.ContextLength
			}
			if model.MaxTokens == 0 {
				model.MaxTokens = existing.MaxTokens
			}
		}

		if err := registry.Save(&model); err != nil {
			return fmt.Errorf("cannot save model: %w", err)
		}

		if existing != nil {
			fmt.Printf("Model %q updated.\n", modelAddFlags.name)
		} else {
			fmt.Printf("Model %q added.\n", modelAddFlags.name)
		}
		return nil
	},
}

func init() {
	modelAddCmd.Flags().StringVar(&modelAddFlags.name, "name", "", "Model name (required)")
	modelAddCmd.Flags().StringVar(&modelAddFlags.title, "title", "", "Display title")
	modelAddCmd.Flags().StringVar(&modelAddFlags.provider, "provider", "", "Provider name (required)")
	modelAddCmd.Flags().Int64Var(&modelAddFlags.contextLength, "context-length", 0, "Maximum context length")
	modelAddCmd.Flags().Int64Var(&modelAddFlags.maxTokens, "max-tokens", 0, "Maximum output tokens")
	modelAddCmd.Flags().BoolVar(&modelAddFlags.enabled, "enabled", true, "Enable this model")
	modelAddCmd.Flags().BoolVar(&modelAddFlags.funcCalling, "func-calling", false, "Supports function calling")
	modelAddCmd.Flags().BoolVar(&modelAddFlags.webSearching, "web-searching", false, "Supports web search")
	modelAddCmd.Flags().Float64Var(&modelAddFlags.temperature, "temperature", 0.7, "Temperature (0.0–2.0)")
	modelAddCmd.Flags().Float64Var(&modelAddFlags.topP, "top-p", 0, "Top-p sampling")
	modelAddCmd.Flags().Float64Var(&modelAddFlags.repetitionPenalty, "repetition-penalty", 0, "Repetition penalty")

	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelRmCmd)
	modelCmd.AddCommand(modelAddCmd)
	modelCmd.AddCommand(modelSetCmd)
	rootCmd.AddCommand(modelCmd)
}

// ── model set ──────────────────────────────────────────────────

var modelSetCmd = &cobra.Command{
	Use:   "set <model-name>",
	Short: "Set the default model",
	Long: `Set the specified model as the default model for new conversations.

This updates the mindx.json configuration so that new sessions
will use this model by default.

Example:
  mindx model set gpt-4
  mindx model set deepseek-v4-flash`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		workspaceDir := core.DefaultUserPrefsDir()

		// Verify the model exists
		registry, err := goharnessconfig.LoadModels(modelsFilePath())
		if err != nil {
			return fmt.Errorf("cannot load models: %w", err)
		}
		if registry.GetRaw(modelName) == nil {
			return fmt.Errorf("model %q not found. Run 'mindx model list' to see available models", modelName)
		}

		cfg, err := core.LoadMindxConfig(workspaceDir)
		if err != nil {
			return fmt.Errorf("cannot load config: %w", err)
		}

		cfg.DefaultModel = modelName
		cfg.LastModel = modelName

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("cannot save config: %w", err)
		}

		fmt.Printf("Default model set to %q.\n", modelName)
		return nil
	},
}

func modelsFilePath() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "settings", "models.yml")
}

func formatInt(n int64) string {
	if n == 0 {
		return "—"
	}
	return strconv.FormatInt(n, 10)
}
