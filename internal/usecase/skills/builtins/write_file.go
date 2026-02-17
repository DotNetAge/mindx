package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/pkg/i18n"
	"os"
	"path/filepath"
	"time"
)

func WriteFile(params map[string]any) (string, error) {
	filename, ok := params["filename"].(string)
	if !ok {
		return "", fmt.Errorf(i18n.TWithData("skill.params_invalid", map[string]interface{}{"Param": "filename"}))
	}

	content, ok := params["content"].(string)
	if !ok {
		return "", fmt.Errorf(i18n.TWithData("skill.params_invalid", map[string]interface{}{"Param": "content"}))
	}

	startTime := time.Now()

	workDir := os.Getenv("MINDX_WORKSPACE")
	if workDir == "" {
		return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
	}

	var filePath string
	if path, ok := params["path"].(string); ok && path != "" {
		filePath = filepath.Join(workDir, "documents", path, filename)
	} else {
		filePath = filepath.Join(workDir, "documents", filename)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf(i18n.TWithData("file.create_dir_failed", map[string]interface{}{"Dir": dir, "Error": err.Error()}))
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf(i18n.TWithData("file.write_failed", map[string]interface{}{"Path": filePath, "Error": err.Error()}))
	}

	elapsed := time.Since(startTime)

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	return getJSONWriteResult(absPath, len(content), elapsed), nil
}

func getJSONWriteResult(filePath string, contentLength int, elapsed time.Duration) string {
	output := map[string]interface{}{
		"file_path":      filePath,
		"content_length": contentLength,
		"elapsed_ms":     elapsed.Milliseconds(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.TWithData("skill.json_serialize_failed", map[string]interface{}{"Error": err.Error()})+"\n")
		os.Exit(1)
	}
	return string(data)
}
