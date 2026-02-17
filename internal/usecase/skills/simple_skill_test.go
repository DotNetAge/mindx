package skills

import (
	"mindx/internal/infrastructure/llama"
	"os"
	"path/filepath"
	"testing"
)

func getProjectRootForSkillTest() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// 从 internal/usecase/skills 回溯到根目录
	root := filepath.Join(wd, "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "skills")); err == nil {
		return root
	}
	// 如果找不到，尝试当前目录
	if _, err := os.Stat("skills"); err == nil {
		return wd
	}
	return wd
}

func TestExtractSkillWords(t *testing.T) {
	svc := llama.NewOllamaService("qwen3:0.6b")
	system := `用户会提供一份大模型调用工具的详细描述，你要用户的输入内容中精炼出这是一个什么工具，可用来做什么；
	例如：["查询天气","询问天气"]，又或者是["查询系统信息"]
	最后只输出一个json，格式如下：
	{
		"keywords": ["关键字1","关键字2"]
	}
	关键字可以有多个，也可以只有一个，关键是要精准。
	`
	content := filepath.Join(getProjectRootForSkillTest(), "skills", "imessage", "SKILL.md")

	data, err := os.ReadFile(content)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	resp, err := svc.ChatWithAgent(system, string(data))
	if err != nil {
		t.Fatalf("调用模型失败: %v", err)
	}
	t.Logf("模型响应: %s", resp)
}
