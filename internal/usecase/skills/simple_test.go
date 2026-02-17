package skills

import (
	"mindx/internal/infrastructure/llama"
	"os"
	"path/filepath"
	"testing"
)

func getProjectRootForSimpleTest() string {
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

func TestExtractWords(t *testing.T) {
	svc := llama.NewOllamaService("qwen3:0.6b")
	system := `用户会提供一份大模型调用工具的详细描述，你要用户的输入内容中精炼出这个工具是用来做什么；
	例如：["查询天气","询问天气"]、["查询系统信息"]、["发送短信"]，关键字一定是[动词+名词]的形式；
	最后只输出一个json，格式如下：
	{
		"keywords": ["关键字1","关键字2"]
	}
	关键字可以有多个，也可以只有一个，关键是要精准。
	`
	content := filepath.Join(getProjectRootForSimpleTest(), "skills", "weather", "SKILL.md")

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
