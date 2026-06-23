package setup

import (
	"fmt"
	"io/fs"

	"github.com/DotNetAge/goharness/config"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// RunWizard runs the interactive setup wizard and applies the results.
// The wizard collects Provider/Model/APIKey from the user (steps 0-2),
// then automatically executes all remaining installation steps (daemon,
// python venv, embedder model, path) with a progress display (step 3),
// and finally shows the completion summary (step 4).
func RunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, cfg *core.MindxConfig, embeddedFS fs.FS) error {
	// 强制将内置的 providers.yml 同步到用户设置目录（覆盖旧版本）
	if err := core.SyncEmbeddedFile(embeddedFS, "runtime/settings/providers.yml", providersPath); err != nil {
		return fmt.Errorf(i18n.T("setup.sync.providers.failed"), err)
	}

	result := runFirstRunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg)
	if result.Err != nil {
		return result.Err
	}

	// Store the actual API key in credential store (not in YAML).
	credStore := core.NewCredentialStore(workspaceDir)
	if result.APIKey != "" {
		if err := credStore.Set(result.SelectedProvider, result.APIKey); err != nil {
			return fmt.Errorf(i18n.T("setup.store.apikey.failed"), err)
		}
	}

	// 将所有从环境变量预解析的非空 Provider Key 写入 Credential Store
	for providerName, key := range result.ResolvedKeys {
		if key != "" && providerName != result.SelectedProvider {
			if err := credStore.Set(providerName, key); err != nil {
				fmt.Printf("⚠️  "+i18n.T("setup.store.apikey.provider.failed")+"\n", providerName, err)
			}
		}
	}

	// Update agent models and config
	if result.SelectedModel != "" {
		if err := updateAllAgentsModel(agentsDir, result.SelectedModel); err != nil {
			return fmt.Errorf(i18n.T("setup.update.agent.model.failed"), err)
		}
		cfg.DefaultModel = result.SelectedModel
		cfg.LastModel = result.SelectedModel
	}

	if result.SelectedProvider != "" {
		cfg.DefaultProvider = result.SelectedProvider
	}
	cfg.Initialized = true

	// Daemon 状态（已由 wizard 内部执行，这里只记录到配置）
	if result.DaemonOK {
		cfg.Daemon.Installed = true
		cfg.Daemon.AutoStart = true
	} else if result.DaemonErr != nil {
		cfg.Daemon.Installed = false
		cfg.Daemon.AutoStart = false
		fmt.Printf("⚠️  "+i18n.T("setup.daemon.register.failed")+"\n", result.DaemonErr)
	}

	// Python 环境（已由 wizard 内部执行）
	if result.PythonOK {
		cfg.Python = result.PythonInfo
	} else if result.PythonErr != nil {
		cfg.Python = result.PythonInfo
		fmt.Printf("⚠️  "+i18n.T("setup.python.config.failed")+"\n", result.PythonErr)
	} else {
		cfg.Python = result.PythonInfo
	}

	// Embedder 模型（已由 wizard 内部执行）
	if result.EmbedderModel != "" {
		cfg.EmbedderModel = result.EmbedderModel
	}

	// PATH 配置（仅 Windows，已由 wizard 内部执行）
	if !result.PathOK && result.PathErr != nil {
		fmt.Printf("⚠️  PATH configuration failed: %v\n", result.PathErr)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf(i18n.T("config.error.serialize.failed"), err)
	}

	// WebUI 提示
	if result.WebUIReady {
		fmt.Print("\n" + i18n.T("setup.webui.ready") + "\n\n")
		fmt.Println("   " + i18n.T("setup.webui.access"))
		fmt.Println("   " + i18n.T("setup.webui.cmd.hint") + "\n")
	} else {
		fmt.Print("\n" + i18n.T("setup.webui.hint") + "\n\n")
	}

	return nil
}

func updateAllAgentsModel(agentsDir, modelName string) error {
	registry, err := config.LoadAgentsFrom(agentsDir)
	if err != nil {
		return err
	}

	for _, agent := range registry.List() {
		agent.Model = modelName
		if err := registry.SaveTo(agent); err != nil {
			return fmt.Errorf(i18n.T("setup.save.agent.model.failed"), agent.Name, err)
		}
	}

	return nil
}
