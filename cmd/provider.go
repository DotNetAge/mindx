package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"charm.land/bubbles/v2/table"
	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
)

// ── provider parent ────────────────────────────────────────────

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage LLM providers",
	Long: `List, add, or remove LLM API providers.

Providers define the API endpoints used to access language models.
See 'mindx provider list' to view configured providers.`,
}

// ── provider list ──────────────────────────────────────────────

var providerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := filepath.Join(core.DefaultUserPrefsDir(), "settings", "providers.yml")
		providers, err := core.LoadProvidersFile(path)
		if err != nil {
			return fmt.Errorf("cannot load providers: %w", err)
		}
		if len(providers) == 0 {
			fmt.Println("No providers configured.")
			return nil
		}

		cols := []table.Column{
			{Title: "Name", Width: 20},
			{Title: "Title", Width: 24},
			{Title: "Base URL", Width: 36},
			{Title: "API Key", Width: 10},
			{Title: "Local", Width: 8},
		}

		rows := make([]table.Row, 0, len(providers))
		for _, p := range providers {
			apiKey := "✓ set"
			if p.APIKey == "" {
				apiKey = "—"
			}
			local := ""
			if p.IsLocal {
				local = "✓"
			}
			title := p.Title
			if title == "" {
				title = "—"
			}
			rows = append(rows, table.Row{p.Name, title, p.BaseURL, apiKey, local})
		}

		tbl := table.New(
			table.WithColumns(cols),
			table.WithRows(rows),
			table.WithHeight(len(rows)+1),
			table.WithWidth(100),
		)
		fmt.Println(tbl.View())
		return nil
	},
}

// ── provider rm ────────────────────────────────────────────────

var providerRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		path := filepath.Join(core.DefaultUserPrefsDir(), "settings", "providers.yml")
		providers, err := core.LoadProvidersFile(path)
		if err != nil {
			return fmt.Errorf("cannot load providers: %w", err)
		}

		found := false
		filtered := providers[:0]
		for _, p := range providers {
			if p.Name == name {
				found = true
				continue
			}
			filtered = append(filtered, p)
		}
		if !found {
			return fmt.Errorf("provider %q not found", name)
		}

		if err := core.SaveProvidersFile(path, filtered); err != nil {
			return fmt.Errorf("cannot save providers: %w", err)
		}

		fmt.Printf("Provider %q removed.\n", name)
		return nil
	},
}

// ── provider add ───────────────────────────────────────────────

var providerAddFlags struct {
	name    string
	title   string
	baseURL string
	apiKey  string
	local   bool
}

var providerAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update a provider",
	Long: `Add a new provider or update an existing one.

The --api-key flag stores an environment variable name (e.g. "MY_API_KEY"),
not the actual secret. The value will be read from the environment at runtime.

Examples:
  mindx provider add --name my-provider --title "My Provider" --base-url https://api.example.com/v1 --api-key MY_API_KEY
  mindx provider add --name ollama --base-url http://localhost:11434 --local`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if providerAddFlags.name == "" {
			return fmt.Errorf("--name is required")
		}
		if providerAddFlags.baseURL == "" {
			return fmt.Errorf("--base-url is required")
		}

		path := filepath.Join(core.DefaultUserPrefsDir(), "settings", "providers.yml")
		providers, err := core.LoadProvidersFile(path)
		if err != nil {
			return fmt.Errorf("cannot load providers: %w", err)
		}

		// Update existing or append new
		found := false
		for _, p := range providers {
			if p.Name == providerAddFlags.name {
				if providerAddFlags.title != "" {
					p.Title = providerAddFlags.title
				}
				p.BaseURL = providerAddFlags.baseURL
				if providerAddFlags.apiKey != "" {
					p.APIKey = providerAddFlags.apiKey
				}
				p.IsLocal = providerAddFlags.local
				found = true
				break
			}
		}
		if !found {
			providers = append(providers, &goharnessconfig.ProviderConfig{
				Name:    providerAddFlags.name,
				Title:   providerAddFlags.title,
				BaseURL: providerAddFlags.baseURL,
				APIKey:  providerAddFlags.apiKey,
				IsLocal: providerAddFlags.local,
			})
		}

		if err := core.SaveProvidersFile(path, providers); err != nil {
			return fmt.Errorf("cannot save providers: %w", err)
		}

		if found {
			fmt.Printf("Provider %q updated.\n", providerAddFlags.name)
		} else {
			fmt.Printf("Provider %q added.\n", providerAddFlags.name)
		}
		return nil
	},
}

func init() {
	providerAddCmd.Flags().StringVar(&providerAddFlags.name, "name", "", "Provider name (required)")
	providerAddCmd.Flags().StringVar(&providerAddFlags.title, "title", "", "Display title")
	providerAddCmd.Flags().StringVar(&providerAddFlags.baseURL, "base-url", "", "API base URL (required)")
	providerAddCmd.Flags().StringVar(&providerAddFlags.apiKey, "api-key", "", "Environment variable name for the API key")
	providerAddCmd.Flags().BoolVar(&providerAddFlags.local, "local", false, "Mark as a local provider")

	providerCmd.AddCommand(providerListCmd)
	providerCmd.AddCommand(providerRmCmd)
	providerCmd.AddCommand(providerAddCmd)
	providerCmd.AddCommand(providerSetkeyCmd)
	rootCmd.AddCommand(providerCmd)
}

// ── provider setkey ────────────────────────────────────────────

var providerSetkeyCmd = &cobra.Command{
	Use:   "setkey <provider> <api-key>",
	Short: "Store an API key for a provider",
	Long: `Store the actual API key for a provider in the system credential store.

On macOS the key is stored in the system Keychain.
On Linux/Windows it is stored in an AES-GCM encrypted file.

Unlike "provider add --api-key" which stores an environment variable name,
this command stores the actual secret value that will be used at runtime.

Example:
  mindx provider setkey dashscope sk-xxxxxxxxxxxx
  mindx provider setkey deepseek sk-xxxxxxxxxxxx`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName := args[0]
		apiKey := args[1]

		workspaceDir := core.DefaultUserPrefsDir()
		providersPath := filepath.Join(workspaceDir, "settings", "providers.yml")

		// Verify the provider exists
		providers, err := core.LoadProvidersFile(providersPath)
		if err != nil {
			return fmt.Errorf("cannot load providers: %w", err)
		}
		found := false
		for _, p := range providers {
			if p.Name == providerName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("provider %q not found. Available: %v",
				providerName, providerNames(providers))
		}

		// Store the key in credential store
		credStore := core.NewCredentialStore(workspaceDir)
		if err := credStore.Set(providerName, apiKey); err != nil {
			return fmt.Errorf("cannot store API key: %w", err)
		}

		// If the provider's api_key is not already set (or is set to an env var),
		// update it to reference the credential store entry.
		// Use LoadProvidersFile/SaveProvidersFile to update the YAML.
		// reload to get fresh pointers
		providers, err = core.LoadProvidersFile(providersPath)
		if err != nil {
			return fmt.Errorf("cannot reload providers: %w", err)
		}
		for _, p := range providers {
			if p.Name == providerName {
				if p.APIKey != providerName {
					oldRef := p.APIKey
					p.APIKey = providerName
					if err := core.SaveProvidersFile(providersPath, providers); err != nil {
						return fmt.Errorf("cannot update provider config: %w", err)
					}
					if oldRef != "" && oldRef != providerName {
						fmt.Printf("  Updated API key reference from %q to %q\n", oldRef, providerName)
					}
				}
				break
			}
		}

		storeName := "system keychain"
		if runtime.GOOS != "darwin" {
			storeName = "encrypted file"
		}
		fmt.Printf("API key for provider %q stored in %s.\n", providerName, storeName)
		return nil
	},
}

func providerNames(providers []*goharnessconfig.ProviderConfig) []string {
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name)
	}
	return names
}
