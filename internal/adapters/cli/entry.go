package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"mindx/internal/config"
	"mindx/pkg/i18n"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mindx",
	Short: i18n.T("cli.root.short"),
	Long:  i18n.T("cli.root.long"),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show MindX version",
	Long:  "Display the current version of MindX",
	Run: func(cmd *cobra.Command, args []string) {
		version, buildTime, gitCommit := config.GetBuildInfo()
		
		fmt.Printf("MindX version: %s\n", version)
		if buildTime != "" {
			fmt.Printf("Build time:   %s\n", buildTime)
		}
		if gitCommit != "" {
			fmt.Printf("Git commit:   %s\n", gitCommit)
		}
	},
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message to MindX",
	Long:  "Send a message to MindX and wait for response",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		message, _ := cmd.Flags().GetString("message")
		
		if message == "" {
			if len(args) > 0 {
				message = args[0]
			} else {
				fmt.Println("Error: message is required")
				os.Exit(1)
			}
		}
		
		err := sendMessageToMindX(port, message)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func sendMessageToMindX(port int, message string) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	
	reqBody := map[string]any{
		"type":    "message",
		"content": message,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := client.Post(fmt.Sprintf("http://localhost:%d/api/conversations/current/message", port), "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(kernelCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(trainCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(sendCmd)

	modelCmd.AddCommand(testCmd)
	kernelCmd.AddCommand(kernelMainCmd)
	kernelCmd.AddCommand(kernelCtrlStartCmd)
	kernelCmd.AddCommand(kernelCtrlStopCmd)
	kernelCmd.AddCommand(kernelCtrlRestartCmd)
	kernelCmd.AddCommand(kernelCtrlStatusCmd)
	
	sendCmd.Flags().IntP("port", "p", 1314, "HTTP server port")
	sendCmd.Flags().StringP("message", "m", "", "Message to send")
}

func Execute() {
	if err := i18n.Init(); err != nil {
		fmt.Printf("i18n init failed: %v\n", err)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
