package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Build-time injected via LDFLAGS (see Makefile).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	Dirty     = "clean"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print MindX version and build information",
	Long: `Display the current MindX binary version, git commit, build time,
and Go runtime information.

Examples:
  mindx version`,
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	fmt.Println("MindX")
	fmt.Println(strings.Repeat("─", 30))
	fmt.Printf("  Version:    %s\n", Version)
	fmt.Printf("  Commit:     %s\n", Commit)
	fmt.Printf("  Build Time: %s\n", BuildTime)
	fmt.Printf("  Dirty:      %s\n", Dirty)
	fmt.Printf("  Go:         %s\n", runtime.Version())
	fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}
