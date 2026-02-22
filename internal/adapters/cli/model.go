package cli

import (
	"context"
	"fmt"
	"mindx/internal/config"
	"mindx/pkg/i18n"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: i18n.T("cli.model.short"),
}

var testCmd = &cobra.Command{
	Use:   "test [model-name]",
	Short: i18n.T("cli.model.test.short"),
	Long:  i18n.T("cli.model.test.long"),
	Example: fmt.Sprintf(`  mindx model test              # %s
  mindx model test qwen3:1.7b  # %s`,
		i18n.T("cli.model.test.example1"),
		i18n.T("cli.model.test.example2")),
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := ""
		if len(args) > 0 {
			modelName = args[0]
		}
		if err := testModelTools(modelName); err != nil {
			fmt.Println(i18n.TWithData("cli.model.test.error", map[string]interface{}{"Error": err.Error()}))
			os.Exit(1)
		}
	},
}

func testModelTools(modelName string) error {
	if err := config.EnsureWorkspace(); err != nil {
		return err
	}

	_, _, _, _, err := config.InitVippers()
	if err != nil {
		return fmt.Errorf("初始化配置失败: %w", err)
	}
	modelsMgr := config.GetModelsManager()
	allModels := modelsMgr.ListModels()

	if len(allModels) == 0 {
		return fmt.Errorf("no models configured")
	}

	var baseURL string
	for _, m := range allModels {
		if m.BaseURL != "" {
			baseURL = m.BaseURL
			break
		}
	}

	if baseURL == "" {
		return fmt.Errorf("no ollama config found")
	}

	if modelName == "" {
		fmt.Println(i18n.T("cli.model.test.header"))
		fmt.Println(i18n.T("cli.model.test.description"))
		fmt.Println("")

		successCount := 0
		for _, model := range allModels {
			if err := testSingleModel(model, baseURL); err == nil {
				successCount++
			}
			fmt.Println("")
		}

		fmt.Println(i18n.T("cli.model.test.result_header"))
		fmt.Println(i18n.TWithData("cli.model.test.total", map[string]interface{}{"Count": len(allModels)}))
		fmt.Println(i18n.TWithData("cli.model.test.supported", map[string]interface{}{"Count": successCount}))
		fmt.Println(i18n.TWithData("cli.model.test.unsupported", map[string]interface{}{"Count": len(allModels) - successCount}))
		return nil
	}

	for _, model := range allModels {
		if model.Name == modelName {
			return testSingleModel(model, baseURL)
		}
	}

	return fmt.Errorf("model not found: %s", modelName)
}

func testSingleModel(model config.ModelConfig, baseURL string) error {
	fmt.Println(i18n.TWithData("cli.model.test.testing", map[string]interface{}{"Name": model.Name}))

	clientConfig := openai.DefaultConfig("")
	clientConfig.BaseURL = baseURL
	client := openai.NewClientWithConfig(clientConfig)

	testTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_weather",
			Description: i18n.T("cli.model.test.tool_desc"),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": i18n.T("cli.model.test.location_desc"),
					},
				},
				"required": []string{"location"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: model.Name,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: i18n.T("cli.model.test.system_prompt"),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: i18n.T("cli.model.test.user_prompt"),
			},
		},
		Tools:       []openai.Tool{testTool},
		Temperature: float32(model.Temperature),
		MaxTokens:   model.MaxTokens,
	}

	startTime := time.Now()
	resp, err := client.CreateChatCompletion(ctx, req)
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Println(i18n.TWithData("cli.model.test.test_error", map[string]interface{}{"Error": err.Error()}))
		return err
	}

	if len(resp.Choices) == 0 {
		fmt.Println(i18n.T("cli.model.test.no_response"))
		return fmt.Errorf("no response")
	}

	message := resp.Choices[0].Message

	if len(message.ToolCalls) > 0 {
		fmt.Println(i18n.T("cli.model.test.support_tools"))
		fmt.Println(i18n.TWithData("cli.model.test.response_time", map[string]interface{}{"Time": elapsed}))
		fmt.Println(i18n.TWithData("cli.model.test.tool_count", map[string]interface{}{"Count": len(message.ToolCalls)}))
		for i, toolCall := range message.ToolCalls {
			fmt.Printf("     [%d] %s: %s\n", i+1, i18n.T("cli.model.test.function"), toolCall.Function.Name)
			fmt.Printf("        %s: %s\n", i18n.T("cli.model.test.arguments"), toolCall.Function.Arguments)
		}
		if message.Content != "" {
			fmt.Println(i18n.TWithData("cli.model.test.extra_text", map[string]interface{}{"Text": truncateString(message.Content, 100)}))
		}
		return nil
	}

	if message.Content != "" {
		fmt.Println(i18n.T("cli.model.test.no_tools"))
		fmt.Println(i18n.TWithData("cli.model.test.response_time", map[string]interface{}{"Time": elapsed}))
		fmt.Println(i18n.T("cli.model.test.text_response"))
		fmt.Printf("     %s: %s\n", i18n.T("cli.model.test.content"), truncateString(message.Content, 100))
		return fmt.Errorf("model does not support tools")
	}

	fmt.Println(i18n.T("cli.model.test.empty_response"))
	return fmt.Errorf("empty response")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
