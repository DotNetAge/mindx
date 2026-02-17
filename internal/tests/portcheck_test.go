package kernel

import (
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/infrastructure/bootstrap"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func getProjectRootForPortcheckTest() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// 从 internal/tests 回溯到根目录
	root := filepath.Join(wd, "..", "..")
	if _, err := os.Stat(filepath.Join(root, "skills")); err == nil {
		return root
	}
	// 如果找不到，尝试当前目录
	if _, err := os.Stat("skills"); err == nil {
		return wd
	}
	return wd
}

func TestPortcheckSkill(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n 初始化失败: %v", err)
	}

	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/portcheck_test.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   false,
		},
		ConversationLogConfig: nil,
	}
	if err := logging.Init(logConfig); err != nil {
		t.Fatalf("日志初始化失败: %v", err)
	}

	projectRoot := getProjectRootForPortcheckTest()
	os.Setenv("WORKSPACE", projectRoot)
	os.Setenv("CONFIG_PATH", filepath.Join(projectRoot, "config"))

	app, err := bootstrap.Startup()
	if err != nil {
		t.Fatalf("启动系统失败: %v", err)
	}
	defer bootstrap.Shutdown()

	timeout := time.After(300 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("索引超时")
		case <-ticker.C:
			if !app.Skills.IsReIndexing() {
				t.Log("索引完成")
				goto indexed
			}
			t.Log("索引进行中...")
		}
	}

indexed:
	brain := app.Assistant.GetBrain()

	questions := []string{
		"查看端口11434是否被占用",
		"帮我查一下8080端口",
		"端口3000有没有被占用？",
	}

	for _, q := range questions {
		t.Logf("\n=== 问题: %s ===", q)

		req := &core.ThinkingRequest{
			Question: q,
			Timeout:  60,
		}

		resp, err := brain.Post(req)
		if err != nil {
			t.Logf("错误: %v", err)
			continue
		}

		t.Logf("响应: %s", resp.Answer)
		t.Logf("工具数量: %d", len(resp.Tools))

		for i, tool := range resp.Tools {
			t.Logf("工具 %d: %s", i+1, tool.Name)
		}

		var foundPortcheck bool
		for _, tool := range resp.Tools {
			if tool.Name == "portcheck" {
				foundPortcheck = true
				break
			}
		}

		if foundPortcheck {
			t.Logf("✓ 成功匹配 portcheck 技能")
		} else {
			t.Logf("✗ 未匹配到 portcheck 技能")
		}
	}
}
