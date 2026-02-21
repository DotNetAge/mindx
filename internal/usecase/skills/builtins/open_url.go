package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/internal/utils"
	"strings"
	"time"
)

func OpenURL(params map[string]any) (string, error) {
	url, ok := params["url"].(string)
	if !ok {
		return "", fmt.Errorf("invalid param: url")
	}

	br, err := utils.NewBrowser("")
	if err != nil {
		return "", fmt.Errorf("failed to create browser: %w", err)
	}

	defer func() {
		br.Close()
		utils.StopChromeDriver()
	}()

	startTime := time.Now()
	result, err := br.Open(url)
	elapsed := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("failed to open url %s: %w", url, err)
	}

	title := extractTitle(result.Content)

	return getJSONResult(url, title, result, elapsed)
}

func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "Skip to") && len(line) > 3 {
			return line
		}
	}
	return ""
}

func getJSONResult(url, title string, result *utils.OpenResult, elapsed time.Duration) (string, error) {
	output := map[string]interface{}{
		"title":      title,
		"url":        url,
		"content":    result.Content,
		"refs":       result.Refs,
		"elapsed_ms": elapsed.Milliseconds(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json serialize failed: %w", err)
	}
	return string(data), nil
}
