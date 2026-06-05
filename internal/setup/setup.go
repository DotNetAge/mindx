package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DotNetAge/goreact/config"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/core"
)

// RunWizard runs the interactive setup wizard and applies the results.
// It handles provider selection, API key input, model selection, daemon setup,
// Python venv setup, and PATH configuration.
func RunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, cfg *core.MindxConfig, embeddedFS fs.FS) error {
	// 强制将内置的 providers.yml 同步到用户设置目录（覆盖旧版本）
	if err := core.SyncEmbeddedFile(embeddedFS, "runtime/settings/providers.yml", providersPath); err != nil {
		return fmt.Errorf("同步 providers.yml 失败: %w", err)
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

	// 将所有从环境变量预解析的非空 Provider Key 写入 Credential Store
	for providerName, key := range result.ResolvedKeys {
		if key != "" && providerName != result.SelectedProvider {
			if err := credStore.Set(providerName, key); err != nil {
				// 单个写入失败不阻断流程，仅记录警告
				fmt.Printf("⚠️  存储 %s 的 API Key 失败: %v\n", providerName, err)
			}
		}
	}

	if result.SelectedModel != "" {
		if err := updateAllAgentsModel(agentsDir, result.SelectedModel); err != nil {
			return fmt.Errorf("更新 Agent 模型配置失败: %w", err)
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
		fmt.Print("📌 安装 mindx 到系统并配置 PATH...\n")
		exe, err := os.Executable()
		if err != nil {
			fmt.Printf("⚠️  无法获取可执行文件路径: %v\n", err)
		} else {
			installDir, err := resolveInstallDir("")
			if err != nil {
				fmt.Printf("⚠️  无法确定安装目录: %v\n", err)
			} else {
				if err := os.MkdirAll(installDir, 0755); err != nil {
					fmt.Printf("⚠️  创建安装目录失败: %v\n", err)
				} else {
					destExe := filepath.Join(installDir, filepath.Base(exe))
					if destExe != exe {
						if err := copyFile(exe, destExe); err != nil {
							fmt.Printf("⚠️  复制二进制失败: %v\n", err)
						} else {
							fmt.Printf("✅ 二进制已安装到: %s\n", destExe)
						}
					} else {
						fmt.Printf("✅ 二进制已存在于: %s\n", destExe)
					}

					// add install directory to shell RC PATH
					if already, err := AddToSystemPath(installDir); err != nil {
						fmt.Printf("⚠️  PATH 设置失败 (可稍后手动配置): %v\n", err)
					} else if !already {
						fmt.Printf("✅ mindx 已添加到系统 PATH: %s\n", installDir)
						fmt.Print("\033[31m⚠️  必须重启终端后生效\033[0m\n\n")
					} else {
						fmt.Println("✅ mindx 已存在于系统 PATH")
					}
				}
			}
		}
	}

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

func updateProviderCredRef(providersPath, providerName string) error {
	data, err := os.ReadFile(providersPath)
	if err != nil {
		return fmt.Errorf("读取 providers.yml 失败: %w", err)
	}

	var provConfig struct {
		Providers []config.ProviderConfig `yaml:"providers"`
	}
	if err := yaml.Unmarshal(data, &provConfig); err != nil {
		return fmt.Errorf("解析 providers.yml 失败: %w", err)
	}

	var found *config.ProviderConfig
	for i := range provConfig.Providers {
		if provConfig.Providers[i].Name == providerName {
			found = &provConfig.Providers[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("提供商 %q 未在配置中找到", providerName)
	}

	// 将 api_key 设为 provider name（作为 CredentialStore 的引用 key）
	found.APIKey = providerName

	out, err := yaml.Marshal(provConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
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
			return fmt.Errorf("保存 Agent %q 模型配置失败: %w", agent.Name, err)
		}
	}

	return nil
}
