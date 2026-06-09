package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DotNetAge/goreact/config"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// RunWizard runs the interactive setup wizard and applies the results.
// It handles provider selection, API key input, model selection, daemon setup,
// Python venv setup, and PATH configuration.
func RunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, cfg *core.MindxConfig, embeddedFS fs.FS) error {
	// 强制将内置的 providers.yml 同步到用户设置目录（覆盖旧版本）
	if err := core.SyncEmbeddedFile(embeddedFS, "runtime/settings/providers.yml", providersPath); err != nil {
		return fmt.Errorf(i18n.T("setup.sync.providers.failed"), err)
	}

	result := runFirstRunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg)
	if result.Err != nil {
		return result.Err
	}

	// Update provider's api_key to credential reference and persist.
	// Runs before storing the actual key so that if file update fails,
	// no key is left orphaned in the credential store.
	if result.SelectedProvider != "" {
		if err := updateProviderCredRef(providersPath, result.SelectedProvider); err != nil {
			return fmt.Errorf(i18n.T("setup.update.provider.config.failed"), err)
		}
	}

	// Store the actual API key in credential store (not in YAML).
	// Runs after models.yml is confirmed updated to avoid inconsistent state.
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
				// 单个写入失败不阻断流程，仅记录警告
				fmt.Printf("⚠️  "+i18n.T("setup.store.apikey.provider.failed")+"\n", providerName, err)
			}
		}
	}

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

	// Setup PATH: copy binary and configure shell RC (must run before daemon)
	if result.PathSetup {
		fmt.Print(i18n.T("setup.path.installing") + "\n")
		exe, err := os.Executable()
		if err != nil {
			fmt.Printf("⚠️  "+i18n.T("setup.exe.path.failed")+"\n", err)
		} else {
			installDir, err := resolveInstallDir("")
			if err != nil {
				fmt.Printf("⚠️  "+i18n.T("setup.install.dir.unknown")+"\n", err)
			} else {
				if err := os.MkdirAll(installDir, 0755); err != nil {
					fmt.Printf("⚠️  "+i18n.T("setup.mkdir.install.failed")+"\n", err)
				} else {
					destExe := filepath.Join(installDir, filepath.Base(exe))
					if destExe != exe {
						if err := copyFile(exe, destExe); err != nil {
							fmt.Printf("⚠️  "+i18n.T("setup.copy.binary.failed")+"\n", err)
						} else {
							fmt.Printf(i18n.T("setup.binary.installed")+"\n", destExe)
						}
					} else {
						fmt.Printf(i18n.T("setup.binary.exists")+"\n", destExe)
					}

					// add install directory to shell RC PATH
					if already, err := AddToSystemPath(installDir); err != nil {
						fmt.Printf("⚠️  "+i18n.T("setup.path.set.failed")+"\n", err)
					} else if !already {
						fmt.Printf(i18n.T("setup.path.added")+"\n", installDir)
						fmt.Print("\033[31m" + i18n.T("setup.path.restart.hint") + "\033[0m\n\n")
					} else {
						fmt.Println(i18n.T("setup.path.already.exists"))
					}
				}
			}
		}
	}

	// Setup daemon if user requested
	if result.DaemonSetup {
		fmt.Print(i18n.T("setup.daemon.registering") + "\n")
		if err := SetupDaemon(workspaceDir); err != nil {
			cfg.Daemon.Installed = false
			cfg.Daemon.AutoStart = false
			fmt.Printf("⚠️  "+i18n.T("setup.daemon.register.failed")+"\n", err)
		} else {
			cfg.Daemon.Installed = true
			cfg.Daemon.AutoStart = true
			fmt.Println(i18n.T("setup.daemon.registered"))
		}
	}

	// Setup Python virtual environment if user requested
	// SetupPython internally detects Python and attempts InstallPython if missing
	if result.PythonSetup {
		fmt.Print(i18n.T("setup.python.checking") + "\n")
		pyInfo, err := SetupPython(workspaceDir)
		if err != nil {
			fmt.Printf("⚠️  "+i18n.T("setup.python.config.failed")+"\n", err)
			cfg.Python = result.PythonInfo
		} else {
			cfg.Python = pyInfo
			fmt.Printf(i18n.T("setup.python.ready")+"\n", pyInfo.VenvPath)
		}
	} else {
		cfg.Python = result.PythonInfo
	}

	if result.EmbedderModel != "" {
		cfg.EmbedderModel = result.EmbedderModel
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf(i18n.T("config.error.serialize.failed"), err)
	}

	if result.WebUIReady {
		fmt.Print("\n" + i18n.T("setup.webui.ready") + "\n\n")
		fmt.Println("   " + i18n.T("setup.webui.access"))
		fmt.Println("   " + i18n.T("setup.webui.cmd.hint") + "\n")
	} else {
		fmt.Print("\n" + i18n.T("setup.webui.hint") + "\n\n")
	}

	return nil
}

func updateProviderCredRef(providersPath, providerName string) error {
	data, err := os.ReadFile(providersPath)
	if err != nil {
		return fmt.Errorf(i18n.T("setup.read.providers.failed"), err)
	}

	var provConfig struct {
		Providers []config.ProviderConfig `yaml:"providers"`
	}
	if err := yaml.Unmarshal(data, &provConfig); err != nil {
		return fmt.Errorf(i18n.T("setup.parse.providers.failed"), err)
	}

	var found *config.ProviderConfig
	for i := range provConfig.Providers {
		if provConfig.Providers[i].Name == providerName {
			found = &provConfig.Providers[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf(i18n.T("setup.provider.notfound"), providerName)
	}

	// 将 api_key 设为 provider name（作为 CredentialStore 的引用 key）
	found.APIKey = providerName

	out, err := yaml.Marshal(provConfig)
	if err != nil {
		return fmt.Errorf(i18n.T("setup.serialize.config.failed"), err)
	}

	return os.WriteFile(providersPath, out, 0644)
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
