package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

type ServiceHandler struct {
	httpPort int
}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{
		httpPort: 911,
	}
}

func (h *ServiceHandler) Start(c *gin.Context) {
	if h.isHTTPServiceRunning() {
		c.JSON(200, gin.H{
			"message": "服务已在运行中",
			"running": true,
		})
		return
	}

	err := startService()
	if err != nil {
		c.JSON(500, gin.H{
			"message": fmt.Sprintf("启动服务失败: %v", err),
			"running": false,
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "服务启动成功",
		"running": true,
	})
}

func (h *ServiceHandler) Stop(c *gin.Context) {
	if !h.isHTTPServiceRunning() {
		c.JSON(200, gin.H{
			"message": "服务未运行",
			"running": false,
		})
		return
	}

	err := stopService()
	if err != nil {
		c.JSON(500, gin.H{
			"message": fmt.Sprintf("停止服务失败: %v", err),
			"running": true,
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "服务停止成功",
		"running": false,
	})
}

func (h *ServiceHandler) OllamaCheck(c *gin.Context) {
	result := gin.H{
		"installed": false,
		"running":   false,
		"models":    "",
	}

	_, err := exec.LookPath("ollama")
	if err != nil {
		c.JSON(http.StatusOK, result)
		return
	}
	result["installed"] = true

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		c.JSON(http.StatusOK, result)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		result["running"] = true

		var tagsResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err == nil {
			modelNames := ""
			for _, m := range tagsResp.Models {
				if modelNames != "" {
					modelNames += "\n"
				}
				modelNames += m.Name
			}
			result["models"] = modelNames
		}
	}

	c.JSON(http.StatusOK, result)
}

func (h *ServiceHandler) OllamaInstall(c *gin.Context) {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Env = append(os.Environ(), "PATH=/usr/local/bin:/usr/bin:/bin")
		go cmd.Run()
		c.JSON(http.StatusOK, gin.H{"message": "Ollama 安装已启动"})
	case "linux":
		cmd := exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		go cmd.Run()
		c.JSON(http.StatusOK, gin.H{"message": "Ollama 安装已启动"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "请手动安装 Ollama: https://ollama.com/download"})
	}
}

func (h *ServiceHandler) ModelTest(c *gin.Context) {
	var req struct {
		ModelName string `json:"model_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	client := http.Client{Timeout: 10 * time.Second}
	testReq := map[string]interface{}{
		"model": req.ModelName,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "test_function",
					"description": "A test function",
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(testReq)
	resp, err := client.Post("http://localhost:11434/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"supports_fc": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				ToolCalls []interface{} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, gin.H{"supports_fc": false, "error": err.Error()})
		return
	}

	supportsFC := len(result.Choices) > 0 && len(result.Choices[0].Message.ToolCalls) > 0
	c.JSON(http.StatusOK, gin.H{"supports_fc": supportsFC})
}

func (h *ServiceHandler) isHTTPServiceRunning() bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/api/health", h.httpPort))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func startService() error {
	switch runtime.GOOS {
	case "darwin":
		return startServiceMacOS()
	case "linux":
		return startServiceLinux()
	case "windows":
		return startServiceWindows()
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

func startServiceMacOS() error {
	cmd := exec.Command("launchctl", "start", "com.mindx.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}

func startServiceLinux() error {
	cmd := exec.Command("systemctl", "start", "mindx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}

func startServiceWindows() error {
	cmd := exec.Command("sc", "start", "MindX")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}

func stopService() error {
	switch runtime.GOOS {
	case "darwin":
		return stopServiceMacOS()
	case "linux":
		return stopServiceLinux()
	case "windows":
		return stopServiceWindows()
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

func stopServiceMacOS() error {
	cmd := exec.Command("launchctl", "stop", "com.mindx.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}

func stopServiceLinux() error {
	cmd := exec.Command("systemctl", "stop", "mindx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}

func stopServiceWindows() error {
	cmd := exec.Command("sc", "stop", "MindX")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}
	return nil
}
