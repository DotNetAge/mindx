package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show or tail MindX daemon logs",
	Long: `Display the MindX daemon and application logs.

Log files are stored in the user's MindX data directory:
  macOS/Linux: ~/.mindx/logs/
  Windows:     %APPDATA%\mindx\logs\

Available log files:
  mindx.log    — Main application log (TUI + daemon)
  daemon.log   — Daemon stdout (launchd/systemd)
  daemon.err   — Daemon stderr (launchd/systemd)

Examples:
  mindx logs              # Show last 50 lines of all log files
  mindx logs -n 100       # Show last 100 lines
  mindx logs --follow     # Tail -f all log files`,
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Tail log files (like tail -f)")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "Number of lines to show")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	logDir := resolveLogDir()

	logFiles := []string{
		"daemon.log",
		"daemon.err",
		"mindx.log",
	}

	// Filter to only existing files
	var existing []string
	for _, name := range logFiles {
		path := filepath.Join(logDir, name)
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}

	if len(existing) == 0 {
		return fmt.Errorf("no log files found in %s", logDir)
	}

	if logsFollow {
		// Tail -f all existing log files using tail
		args := append([]string{"-f", "-n", fmt.Sprintf("%d", logsLines)}, existing...)
		cmdTail := exec.Command("tail", args...)
		cmdTail.Stdout = os.Stdout
		cmdTail.Stderr = os.Stderr
		return cmdTail.Run()
	}

	// Print each log file with a header
	for i, path := range existing {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("📄 %s\n", path)
		fmt.Println(strings.Repeat("─", 50))
		data, err := readTail(path, logsLines)
		if err != nil {
			fmt.Printf("  (error reading: %v)\n", err)
			continue
		}
		fmt.Print(data)
	}

	return nil
}

func resolveLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mindx", "logs")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "mindx", "logs")
		}
	}
	return filepath.Join(home, ".mindx", "logs")
}

// readTail reads the last n lines from a file.
func readTail(path string, n int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}
