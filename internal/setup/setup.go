package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/goreact"
	goreactcore "github.com/DotNetAge/goreact/core"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/core"
)

// RunWizard runs the interactive setup wizard and applies the results.
// It handles model selection, API key input, daemon setup, Python venv setup,
// and memory embedder model download.
func RunWizard(modelsPath, agentsDir, workspaceDir string, cfg *core.MindxConfig) error {
	result := runFirstRunWizard(modelsPath, agentsDir, workspaceDir, cfg)
	if result.Err != nil {
		return result.Err
	}

	// Update models.yml to use the credential reference name first.
	// This runs before storing the API key so that if models.yml update
	// fails, no key is left orphaned in the credential store.
	if err := updateModelCredRef(modelsPath, result.SelectedModel, result.CredRef); err != nil {
		return fmt.Errorf("更新模型配置失败: %w", err)
	}

	// Store the actual API key in credential store (not in YAML).
	// Runs after models.yml is confirmed updated to avoid inconsistent state.
	// If user skipped API key input, the existing key is preserved.
	credStore := core.NewCredentialStore(workspaceDir)
	if result.APIKey != "" {
		if err := credStore.Set(result.CredRef, result.APIKey); err != nil {
			return fmt.Errorf("存储 API Key 失败: %w", err)
		}
	}

	if err := updateAllAgentsModel(agentsDir, result.SelectedModel); err != nil {
		return fmt.Errorf("更新 Agent 模型配置失败: %w", err)
	}

	cfg.DefaultModel = result.SelectedModel
	cfg.Initialized = true

	// Setup daemon if user requested
	if result.DaemonSetup {
		fmt.Print("⚙️  注册 Daemon 自启动服务...\n")
		if err := SetupDaemon(workspaceDir); err != nil {
			cfg.Daemon.Installed = false
			cfg.Daemon.AutoStart = false
			fmt.Printf("⚠️  Daemon 注册失败 (可稍后手动配置): %v\n", err)
		} else {
			cfg.Daemon.Installed = true
			cfg.Daemon.AutoStart = true
			fmt.Println("✅ Daemon 自启动服务已注册")
		}
	}

	// Setup Python virtual environment if user requested
	// SetupPython internally detects Python and attempts InstallPython if missing
	if result.PythonSetup {
		fmt.Print("🐍 检查 Python 环境...\n")
		pyInfo, err := SetupPython(workspaceDir)
		if err != nil {
			fmt.Printf("⚠️  Python 配置失败 (可稍后手动配置): %v\n", err)
			cfg.Python = result.PythonInfo
		} else {
			cfg.Python = pyInfo
			fmt.Printf("✅ Python 环境已就绪: %s\n", pyInfo.VenvPath)
		}
	} else {
		cfg.Python = result.PythonInfo
	}

	if result.EmbedderModel != "" {
		cfg.EmbedderModel = result.EmbedderModel
	}

	if result.PathSetup {
		fmt.Print("📌 配置系统 PATH...\n")
		exe, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(exe)
			if !CheckInPath(dir) {
				if err := AddToPath(dir); err != nil {
					fmt.Printf("⚠️  PATH 设置失败 (可稍后手动配置): %v\n", err)
				} else {
					fmt.Printf("✅ mindx 已添加到系统 PATH: %s\n", dir)
					fmt.Print("\033[31m⚠️  必须重启终端后自动生效\033[0m\n\n")
				}
			} else {
				fmt.Println("✅ mindx 已存在于系统 PATH")
			}
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("保存 mindx.json 失败: %w", err)
	}

	return nil
}

func updateModelCredRef(modelsPath, modelName, credRef string) error {
	registry, err := goreact.LoadModels(modelsPath)
	if err != nil {
		return err
	}

	cfg := registry.Get(modelName)
	if cfg == nil {
		return fmt.Errorf("模型 %q 未在配置中找到", modelName)
	}

	cfg.APIKey = credRef

	type modelsWrapper struct {
		Models []goreactcore.ModelConfig `yaml:"models"`
	}

	wrapper := modelsWrapper{}
	for _, m := range registry.List() {
		wrapper.Models = append(wrapper.Models, *m)
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("序列化模型配置失败: %w", err)
	}

	return os.WriteFile(modelsPath, data, 0644)
}

func updateAllAgentsModel(agentsDir, modelName string) error {
	registry, err := goreact.LoadAgentsFrom(agentsDir)
	if err != nil {
		return err
	}

	for _, agent := range registry.List() {
		agent.Model = modelName
		if err := registry.SaveTo(agent); err != nil {
			return fmt.Errorf("保存 Agent %q 模型配置失败: %w", agent.Name, err)
		}
	}

	return nil
}
