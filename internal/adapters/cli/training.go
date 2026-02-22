package cli

import (
	"fmt"
	"log"
	"mindx/internal/config"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/embedding"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/training"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

var trainCmd = &cobra.Command{
	Use:   "train",
	Short: i18n.T("cli.train.short"),
	Long:  i18n.T("cli.train.long"),
	Example: fmt.Sprintf(`  # %s
  mindx train --run-once

  # %s
  mindx train --run-once --mode lora

  # %s
  mindx train --run-once --model qwen3:0.6b

  # %s
  mindx train --run-once --min-corpus 50`,
		i18n.T("cli.train.example1"),
		i18n.T("cli.train.example2"),
		i18n.T("cli.train.example3"),
		i18n.T("cli.train.example4")),
	Run: func(cmd *cobra.Command, args []string) {
		baseModel, _ := cmd.Flags().GetString("model")
		ollamaURL, _ := cmd.Flags().GetString("ollama")
		dataDir, _ := cmd.Flags().GetString("data-dir")
		minCorpus, _ := cmd.Flags().GetInt("min-corpus")
		runOnce, _ := cmd.Flags().GetBool("run-once")
		configPath, _ := cmd.Flags().GetString("config")
		mode, _ := cmd.Flags().GetString("mode")
		trainingDir, _ := cmd.Flags().GetString("training-dir")
		workspace, _ := cmd.Flags().GetString("workspace")

		logger := logging.GetSystemLogger()

		var trainMode training.TrainingMode
		switch mode {
		case "lora":
			trainMode = training.ModeLora
			if _, err := os.Stat(filepath.Join(trainingDir, ".venv")); os.IsNotExist(err) {
				log.Fatal(i18n.TWithData("cli.train.lora.env_required", map[string]interface{}{"Dir": trainingDir}))
			}
		case "message":
			trainMode = training.ModeMessage
		default:
			log.Fatal(i18n.TWithData("cli.train.mode.unknown", map[string]interface{}{"Mode": mode}))
		}

		if workspace == "" {
			workspace, _ = os.Getwd()
		}

		if err := config.EnsureWorkspace(); err != nil {
			log.Fatal(err)
		}

		srvCfg, _, _, _, err := config.InitVippers()
		if err != nil {
			log.Fatal(err)
		}

		dataPath, err := config.GetWorkspaceDataPath()
		if err != nil {
			log.Fatal(err)
		}

		store, err := persistence.NewStore("badger", filepath.Join(dataPath, "memory"), nil)
		if err != nil {
			log.Fatal(i18n.TWithData("cli.store.create_failed", map[string]interface{}{"Error": err.Error()}))
		}
		defer store.Close()

		ollamaEmbeddingURL := ollamaURL
		if ollamaEmbeddingURL == "" {
			ollamaEmbeddingURL = "http://localhost:11434"
		}
		embeddingProvider, err := infraEmbedding.NewOllamaEmbedding(ollamaEmbeddingURL, "qllama/bge-small-zh-v1.5:latest")
		if err != nil {
			log.Printf("%s: %v", i18n.T("cli.train.embedding_warning"), err)
		}

		var embeddingSvc *embedding.EmbeddingService
		if embeddingProvider != nil {
			embeddingSvc = embedding.NewEmbeddingService(embeddingProvider)
		}

		modelsMgr := config.GetModelsManager()
		brainModels := modelsMgr.GetBrainModels()
		defaultModelName := modelsMgr.GetDefaultModel()
		if defaultModelName == "" {
			defaultModelName = brainModels.SubconsciousModel
		}
		defaultModel, err := modelsMgr.GetModel(defaultModelName)
		if err != nil {
			defaultModel = &config.ModelConfig{
				Name:    "qwen3:0.6b",
				BaseURL: "http://localhost:11434/v1",
			}
		}

		openaiCfg := openai.DefaultConfig(defaultModel.APIKey)
		openaiCfg.BaseURL = defaultModel.BaseURL
		memLLMClient := openai.NewClientWithConfig(openaiCfg)

		mem, err := memory.NewMemory(srvCfg, memLLMClient, logger, store, embeddingSvc)
		if err != nil {
			log.Fatal(i18n.TWithData("cli.memory.init_failed", map[string]interface{}{"Error": err.Error()}))
		}

		memAdapter, err := training.NewMemoryAdapter(mem)
		if err != nil {
			log.Fatal(i18n.TWithData("cli.memory.adapter_failed", map[string]interface{}{"Error": err.Error()}))
		}

		collector, err := training.NewCollector(memAdapter, logger)
		if err != nil {
			log.Fatal(i18n.TWithData("cli.collector.create_failed", map[string]interface{}{"Error": err.Error()}))
		}

		filter := training.NewFilter(logger)

		outputDir := filepath.Join(dataDir, "training")
		generator, err := training.NewGenerator(outputDir, logger)
		if err != nil {
			log.Fatal(i18n.TWithData("cli.generator.create_failed", map[string]interface{}{"Error": err.Error()}))
		}

		validator := training.NewValidator(ollamaURL, embeddingSvc, logger)

		configUpdater := training.NewConfigUpdater(configPath, logger)

		trainer := training.NewTrainer(
			collector,
			filter,
			generator,
			validator,
			configUpdater,
			baseModel,
			dataDir,
			ollamaURL,
			minCorpus,
			trainMode,
			trainingDir,
			logger,
		)

		if runOnce {
			log.Println(i18n.TWithData("cli.train.starting", map[string]interface{}{"Mode": mode}))
			report, err := trainer.RunTraining()
			if err != nil {
				log.Fatal(i18n.TWithData("cli.train.failed", map[string]interface{}{"Error": err.Error()}))
			}

			fmt.Printf("\n========== %s ==========\n", i18n.T("cli.train.report.title"))
			fmt.Printf("%s: %s\n", i18n.T("cli.train.report.id"), report.TrainingID)
			fmt.Printf("%s: %s\n", i18n.T("cli.train.report.status"), report.Status)
			fmt.Printf("%s: %s\n", i18n.T("cli.train.report.mode"), mode)
			fmt.Printf("%s: %s\n", i18n.T("cli.train.report.base_model"), report.BaseModel)
			if report.NewModel != "" {
				fmt.Printf("%s: %s\n", i18n.T("cli.train.report.new_model"), report.NewModel)
			}
			fmt.Printf("%s: %d %s\n", i18n.T("cli.train.report.total_pairs"), report.TotalPairs, i18n.T("cli.train.report.pairs_unit"))
			fmt.Printf("%s: %d %s\n", i18n.T("cli.train.report.filtered_pairs"), report.FilteredPairs, i18n.T("cli.train.report.pairs_unit"))
			fmt.Printf("%s: %s\n", i18n.T("cli.train.report.time"), report.TrainingTime)
			if report.BaseScore > 0 {
				fmt.Printf("%s: %.2f\n", i18n.T("cli.train.report.base_score"), report.BaseScore)
			}
			if report.ValidationScore > 0 {
				fmt.Printf("%s: %.2f\n", i18n.T("cli.train.report.new_score"), report.ValidationScore)
			}
			fmt.Printf("%s: %v\n", i18n.T("cli.train.report.improved"), report.Improved)
			if report.Error != "" {
				fmt.Printf("%s: %s\n", i18n.T("cli.train.report.error"), report.Error)
			}
			fmt.Println("==============================")

			os.Exit(0)
		}

		log.Println(i18n.T("cli.train.run_once_hint"))
	},
}

func init() {
	trainCmd.Flags().String("model", "qwen3:0.6b", i18n.T("cli.train.flag.model"))
	trainCmd.Flags().String("ollama", "http://localhost:11434", i18n.T("cli.train.flag.ollama"))
	trainCmd.Flags().String("data-dir", "data", i18n.T("cli.train.flag.data_dir"))
	trainCmd.Flags().Int("min-corpus", 50, i18n.T("cli.train.flag.min_corpus"))
	trainCmd.Flags().Bool("run-once", false, i18n.T("cli.train.flag.run_once"))
	trainCmd.Flags().String("config", "config/models.json", i18n.T("cli.train.flag.config"))
	trainCmd.Flags().String("mode", "message", i18n.T("cli.train.flag.mode"))
	trainCmd.Flags().String("training-dir", "training", i18n.T("cli.train.flag.training_dir"))
	trainCmd.Flags().String("workspace", "", i18n.T("cli.train.flag.workspace"))
}
