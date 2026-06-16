package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── fs parent ─────────────────────────────────────────────────

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Filesystem operations through the daemon (requires daemon)",
	Long: `Read, list, and manage files through the daemon process.

All operations require the daemon to be running (mindx start).

Examples:
  mindx fs list /path/to/dir
  mindx fs read /path/to/file
  mindx fs mkdir /path/to/new-dir
  mindx fs rm /path/to/file`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(fsCmd)
}

// ── response type (aligned with RPC) ──────────────────────────

type fsEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

// ── fs list ───────────────────────────────────────────────────

var fsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List directory contents",
	Args:    cobra.ExactArgs(1),
	Example: `  mindx fs list /path/to/dir
  mindx fs ls /path/to/dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSList(args[0])
		if err != nil {
			return err
		}

		var entries []fsEntry
		if err := json.Unmarshal(result, &entries); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(entries) == 0 {
			fmt.Println("Empty directory.")
			return nil
		}

		table := render.NewTable([]string{"Name", "Type", "Size", "Mode", "Modified"}, 100)
		for _, e := range entries {
			fileType := "file"
			if e.IsDir {
				fileType = "dir"
			}
			size := fmt.Sprintf("%d", e.Size)
			if e.IsDir {
				size = "-"
			}
			table.AddRow([]string{e.Name, fileType, size, e.Mode, e.ModTime})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d entr(ies)\n", len(entries))
		return nil
	},
}

// ── fs read ───────────────────────────────────────────────────

var fsReadCmd = &cobra.Command{
	Use:     "read",
	Short:   "Read file contents",
	Args:    cobra.ExactArgs(1),
	Example: `  mindx fs read /path/to/file.go`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSRead(args[0])
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fs write ──────────────────────────────────────────────────

var fsWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "Write content to a file",
	Long: `Write content to a file. Content is read from stdin by default.
Use --content to provide inline content, or pipe data via stdin.

Examples:
  mindx fs write /path/to/file --content "file content here"
  echo "file content" | mindx fs write /path/to/file`,
	Example: `  mindx fs write /path/to/file --content "Hello, World!"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			return fmt.Errorf("--content is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSWrite(args[0], content)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fs mkdir ──────────────────────────────────────────────────

var fsMkdirCmd = &cobra.Command{
	Use:     "mkdir",
	Short:   "Create a directory",
	Args:    cobra.ExactArgs(1),
	Example: `  mindx fs mkdir /path/to/new-dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("parents")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSMkdir(args[0], all)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fs rm ─────────────────────────────────────────────────────

var fsRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a file or directory",
	Args:  cobra.ExactArgs(1),
	Example: `  mindx fs rm /path/to/file
  mindx fs rm --recurse /path/to/dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		recurse, _ := cmd.Flags().GetBool("recurse")
		force, _ := cmd.Flags().GetBool("force")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSRm(args[0], recurse, force)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fs mv ─────────────────────────────────────────────────────

var fsMvCmd = &cobra.Command{
	Use:     "mv",
	Short:   "Move or rename a file or directory",
	Args:    cobra.ExactArgs(2),
	Example: `  mindx fs mv /path/to/src /path/to/dst`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSMv(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fs home ───────────────────────────────────────────────────

var fsHomeCmd = &cobra.Command{
	Use:   "home",
	Short: "Show the daemon's home directory path",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FSHome()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	fsWriteCmd.Flags().String("content", "", "File content (required)")
	fsMkdirCmd.Flags().BoolP("parents", "p", false, "Create parent directories as needed")
	fsRmCmd.Flags().BoolP("recurse", "r", false, "Recursively remove directories")
	fsRmCmd.Flags().BoolP("force", "f", false, "Force remove without confirmation")

	fsCmd.AddCommand(fsListCmd)
	fsCmd.AddCommand(fsReadCmd)
	fsCmd.AddCommand(fsWriteCmd)
	fsCmd.AddCommand(fsHomeCmd)
	fsCmd.AddCommand(fsMkdirCmd)
	fsCmd.AddCommand(fsRmCmd)
	fsCmd.AddCommand(fsMvCmd)
}
