package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── daemon parent (server process) ────────────────────────────

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the MindX daemon process (server)",
	Long: `Run the MindX daemon process providing WebSocket gateway and HTTP services.

This is the actual server process, typically started by the system service
manager via 'mindx start'. Used by WebUI, MacUI, or other clients.

Subcommands (version, check-update, apply-update, restart, config) require
the daemon to be running.`,
	PersistentPreRunE: requireDaemon,
	RunE:              runDaemon,
}

var (
	daemonPort string
	daemonPath string
)

func init() {
	daemonCmd.Flags().StringVarP(&daemonPort, "port", "p", ":1314", i18n.T("cmd.daemon.flag.port.desc"))
	daemonCmd.Flags().StringVar(&daemonPath, "path", "", i18n.T("cmd.daemon.flag.path.desc"))
}

func runDaemon(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	if err := core.ExtractWorkspace(RuntimeFS, workspaceDir); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.daemon.error.workspace.init"), err)
	}

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.daemon.error.config.load"), err)
	}

	if _, err := core.SyncRuntimeAssets(RuntimeFS, workspaceDir, core.Version, cfg); err != nil {
		return fmt.Errorf("sync runtime assets: %w", err)
	}

	if !cfg.Initialized {
		return fmt.Errorf("%s", i18n.T("cmd.daemon.error.notconfigured"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wsPath := daemonPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	server, err := svc.NewServer(daemonPort, wsPath, AppIconFS, RuntimeFS)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server.RegisterBuiltinCommands()

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// ── daemon version ────────────────────────────────────────────

var daemonVersionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Show daemon version information",
	Example: `  mindx daemon version`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ServerVersion()
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var info map[string]string
		if err := json.Unmarshal(result, &info); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"Key", "Value"}, 60)
		for k, v := range info {
			table.AddRow([]string{k, v})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── daemon check-update ───────────────────────────────────────

var daemonCheckUpdateCmd = &cobra.Command{
	Use:     "check-update",
	Short:   "Check if a daemon update is available",
	Example: `  mindx daemon check-update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ServerCheckUpdate()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── daemon apply-update ───────────────────────────────────────

var daemonApplyUpdateCmd = &cobra.Command{
	Use:     "apply-update",
	Short:   "Download and apply a daemon update",
	Example: `  mindx daemon apply-update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ServerApplyUpdate()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── daemon restart ────────────────────────────────────────────

var daemonRestartCmd = &cobra.Command{
	Use:     "restart",
	Short:   "Restart the running daemon process",
	Example: `  mindx daemon restart`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ServerRestartDaemon()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── daemon config ─────────────────────────────────────────────

var daemonConfigCmd = &cobra.Command{
	Use:     "config",
	Short:   "Show daemon user configuration",
	Example: `  mindx daemon config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.UserConfig()
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(result, &cfg); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"Key", "Value"}, 60)
		for k, v := range cfg {
			valStr := fmt.Sprintf("%v", v)
			table.AddRow([]string{k, valStr})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	daemonVersionCmd.Flags().Bool("json", false, "Output raw JSON")
	daemonConfigCmd.Flags().Bool("json", false, "Output raw JSON")

	daemonCmd.AddCommand(daemonVersionCmd)
	daemonCmd.AddCommand(daemonCheckUpdateCmd)
	daemonCmd.AddCommand(daemonApplyUpdateCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonConfigCmd)
}
