package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/internal/utils"
	"time"
)

func Search(params map[string]any) (string, error) {
	terms, ok := params["terms"].(string)
	if !ok {
		return "", fmt.Errorf("terms is not a string")
	}

	br, err := utils.NewBrowser("")
	if err != nil {
		return "", fmt.Errorf("browser create failed: %w", err)
	}

	defer func() {
		br.Close()
		utils.StopChromeDriver()
	}()

	startTime := time.Now()
	results, err := br.Search(terms, 10)
	elapsed := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	return getJSONOutput(results, elapsed)
}

func getJSONOutput(results []utils.SearchResult, elapsed time.Duration) (string, error) {
	output := map[string]interface{}{
		"count":      len(results),
		"elapsed_ms": elapsed.Milliseconds(),
		"results":    results,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json serialize failed: %w", err)
	}
	return string(data), nil
}
