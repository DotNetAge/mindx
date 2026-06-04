package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/goreact/config"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/core"
)

// RunWizard runs the interactive setup wizard and applies the results.
// It handles provider selection, API key input, model selection, daemon setup,
// Python venv setup, and PATH configuration.
func RunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, cfg *core.MindxConfig) error {
	result := runFirstRunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg)
	if result.Err != nil {
		return result.Err
	}

	// Update provider's api_key to credential reference and persist.
	// Runs before storing the actual key so that if file update fails,
	// no key is left orphaned in the credential store.
	if result.SelectedProvider != "" {
		if err := updateProviderCredRef(modelsPath, result.SelectedProvider); err != nil {
			return fmt.Errorf("更新提供商配置失败: %w", err)
		}
	}

	// Store the actual API key in credential store (not in YAML).
	// Runs after models.yml is confirmed updated to avoid inconsistent state.
	credStore := core.NewCredentialStore(workspaceDir)
	if result.APIKey != "" {
		if err := credStore.Set(result.SelectedProvider, result.APIKey); err != nil {
			return fmt.Errorf("存储 API Key 失败: %w", err)
		}
	}

	if result.SelectedModel != "" {
		if err := updateAllAgentsModel(agentsDir, result.SelectedModel); err != nil {
			return fmt.Errorf("更新 Agent 模型配置失败: %w", err)
		}
		cfg.DefaultModel = result.SelectedModel
	}
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
			if already, err := AddToSystemPath(dir); err != nil {
				fmt.Printf("⚠️  PATH 设置失败 (可稍后手动配置): %v\n", err)
			} else if !already {
				fmt.Printf("✅ mindx 已添加到系统 PATH: %s\n", dir)
				fmt.Print("\033[31m⚠️  必须重启终端后自动生效\033[0m\n\n")
			} else {
				fmt.Println("✅ mindx 已存在于系统 PATH")
			}
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("保存 mindx.json 失败: %w", err)
	}

	if result.WebUIReady {
		fmt.Print("\n🌐 WebUI 已就绪\n\n")
		fmt.Printf("   启动 Daemon 后访问: http://localhost:1313\n")
		fmt.Printf("   或直接运行: mindx web\n\n")
	} else {
		fmt.Print("\n💡 提示: 运行 'mindx web' 可打开 WebUI 界面 (需 Daemon 运行中)\n\n")
	}

	return nil
}

func updateProviderCredRef(modelsPath, providerName string) error {
	registry, err := config.LoadModels(modelsPath)
	if err != nil {
		return err
	}

	provider := registry.GetProvider(providerName)
	if provider == nil {
		return fmt.Errorf("提供商 %q 未在配置中找到", providerName)
	}
	provider.APIKey = providerName
	registry.RegisterProvider(providerName, provider)

	type modelsWrapper struct {
		Providers []config.ProviderConfig `yaml:"providers"`
		Models    []config.ModelConfig    `yaml:"models"`
	}

	providers := registry.Providers()
	providerCfgs := make([]config.ProviderConfig, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			providerCfgs = append(providerCfgs, *p)
		}
	}

	rawModels := registry.ListRaw()
	modelCfgs := make([]config.ModelConfig, 0, len(rawModels))
	for _, m := range rawModels {
		if m != nil {
			modelCfgs = append(modelCfgs, *m)
		}
	}

	wrapper := modelsWrapper{
		Providers: providerCfgs,
		Models:    modelCfgs,
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(modelsPath, data, 0644)
}

func updateAllAgentsModel(agentsDir, modelName string) error {
	registry, err := config.LoadAgentsFrom(agentsDir)
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
