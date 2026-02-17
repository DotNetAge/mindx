package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"os"
	"strings"
	"time"
)

func OpenURL(params map[string]any) (string, error) {
	url, ok := params["url"].(string)
	if !ok {
		return "", fmt.Errorf(i18n.TWithData("skill.params_invalid", map[string]interface{}{"Param": "url"}))
	}

	br, err := utils.NewBrowser("")
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.TWithData("browser.create_failed", map[string]interface{}{"Error": err.Error()})+"\n")
		os.Exit(1)
	}

	defer func() {
		br.Close()
		utils.StopChromeDriver()
	}()

	startTime := time.Now()
	result, err := br.Open(url)
	elapsed := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf(i18n.TWithData("browser.open_url_failed", map[string]interface{}{"URL": url, "Error": err.Error()}))
	}

	title := extractTitle(result.Content)

	return getJSONResult(url, title, result, elapsed), nil
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

func getJSONResult(url, title string, result *utils.OpenResult, elapsed time.Duration) string {

	output := map[string]interface{}{
		"title":      title,
		"url":        url,
		"content":    result.Content,
		"refs":       result.Refs,
		"elapsed_ms": elapsed.Milliseconds(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.TWithData("skill.json_serialize_failed", map[string]interface{}{"Error": err.Error()})+"\n")
		os.Exit(1)
	}
	return string(data)
}
