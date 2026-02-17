package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"os"
	"time"
)

func Search(params map[string]any) (string, error) {
	terms, ok := params["terms"].(string)
	if !ok {
		return "", fmt.Errorf("terms is not a string")
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
	results, err := br.Search(terms, 10)
	elapsed := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	return getJSONOutput(results, elapsed), nil
}

func getJSONOutput(results []utils.SearchResult, elapsed time.Duration) string {

	output := map[string]interface{}{
		"count":      len(results),
		"elapsed_ms": elapsed.Milliseconds(),
		"results":    results,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.TWithData("json_serialize_failed", map[string]interface{}{"Error": err.Error()})+"\n", err)
		os.Exit(1)
	}
	return string(data)
}
