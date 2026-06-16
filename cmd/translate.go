package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── translate command ─────────────────────────────────────────

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate text via the daemon",
	Long: `Translate text to any supported language via the daemon.

Examples:
  mindx translate --text "Hello" --lang "中文"
  mindx translate --text "Bonjour" --lang "English"`,
	PersistentPreRunE: requireDaemon,
	RunE: func(cmd *cobra.Command, args []string) error {
		text, _ := cmd.Flags().GetString("text")
		lang, _ := cmd.Flags().GetString("lang")
		if text == "" {
			return fmt.Errorf("--text is required")
		}
		if lang == "" {
			return fmt.Errorf("--lang is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.Translate(text, lang)
		if err != nil {
			return err
		}

		// Try to extract translated text from response
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if translated, ok := resp["translated"].(string); ok {
				fmt.Println(translated)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(translateCmd)
	translateCmd.Flags().String("text", "", "Text to translate (required)")
	translateCmd.Flags().String("lang", "", "Target language (e.g. 中文, 日本語) (required)")
}
