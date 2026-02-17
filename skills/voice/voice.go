package voice

import (
	"fmt"
	"os/exec"
)

func Execute(params map[string]interface{}) (string, error) {
	// 获取要播报的文本
	text, ok := params["text"].(string)
	if !ok {
		return "", fmt.Errorf("missing required parameter: text")
	}

	// 获取可选参数：语音类型
	voice := ""
	if v, ok := params["voice"].(string); ok {
		voice = v
	}

	// 构建say命令
	cmd := exec.Command("say")
	
	// 添加语音参数（如果指定）
	if voice != "" {
		cmd.Args = append(cmd.Args, "-v", voice)
	}
	
	// 添加要播报的文本
	cmd.Args = append(cmd.Args, text)

	// 执行命令
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute say command: %w", err)
	}

	return fmt.Sprintf("Voice notification sent: %s", text), nil
}
