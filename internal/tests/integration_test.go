package kernel

import (
	"fmt"
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

func getProjectRootForIntegrationTest() string {
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

var testApp *bootstrap.App

func setupTestSuite(t *testing.T) *bootstrap.App {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	if testApp != nil {
		return testApp
	}

	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n 初始化失败: %v", err)
	}

	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/integration_test.log",
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

	projectRoot := getProjectRootForIntegrationTest()
	os.Setenv("WORKSPACE", projectRoot)
	os.Setenv("CONFIG_PATH", filepath.Join(projectRoot, "config"))

	var err error
	testApp, err = bootstrap.Startup()
	if err != nil {
		t.Fatalf("启动系统失败: %v", err)
	}

	timeout := time.After(600 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("索引超时")
		case <-ticker.C:
			if !testApp.Skills.IsReIndexing() {
				t.Log("索引完成")
				return testApp
			}
			t.Log("索引进行中...")
		}
	}
}

func teardownTestSuite() {
	if testApp != nil {
		bootstrap.Shutdown()
		testApp = nil
	}
}

type SkillTestCase struct {
	Name         string
	Question     string
	ExpectedTool string
}

var skillTests = []SkillTestCase{
	{"weather", "明天广州的天气如何？", "weather"},
	{"sysinfo", "查看系统CPU和内存使用情况", "sysinfo"},
	{"screenshot", "帮我截取屏幕截图", "screenshot"},
	{"terminal", "帮我运行终端命令 ls", "terminal"},
	{"calculator", "计算 123 加 456 等于多少", "calculator"},
	{"calendar", "查看我明天的日程安排", "calendar"},
	{"clipboard", "读取剪贴板内容", "clipboard"},
	{"contacts", "搜索联系人张三", "contacts"},
	{"file_search", "搜索文件名为test的文件", "file_search"},
	{"finder", "打开Finder查找文件", "finder"},
	{"mail", "发送邮件给张三", "mail"},
	{"notes", "创建一条新笔记", "notes"},
	{"notify", "发送一个通知提醒我", "notify"},
	{"open", "打开计算器应用", "open"},
	{"open_url", "打开网址 https://www.baidu.com", "open_url"},
	{"reminders", "添加一个明天上午9点的提醒", "reminders"},
	{"volume", "获取当前音量", "volume"},
	{"web_search", "搜索关于人工智能的最新新闻", "web_search"},
	{"wifi", "查看当前WiFi连接状态", "wifi"},
	{"write_file", "写入文件内容到test.txt", "write_file"},
	{"portcheck", "查看端口11434是否被占用", "portcheck"},
}

func TestAllSkills(t *testing.T) {
	app := setupTestSuite(t)
	defer teardownTestSuite()

	brain := app.Assistant.GetBrain()

	results := make(map[string]bool)
	toolFoundResults := make(map[string]bool)
	toolCalledResults := make(map[string]bool)

	for _, tc := range skillTests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("=== 测试技能: %s ===", tc.Name)
			t.Logf("用户输入: %s", tc.Question)

			req := &core.ThinkingRequest{
				Question: tc.Question,
				Timeout:  60,
			}

			resp, err := brain.Post(req)
			if err != nil {
				t.Logf("Brain.Post 失败: %v", err)
				results[tc.Name] = false
				toolFoundResults[tc.Name] = false
				toolCalledResults[tc.Name] = false
				return
			}

			t.Logf("响应: %s", resp.Answer)
			t.Logf("工具数量: %d", len(resp.Tools))

			var foundTool bool
			for _, tool := range resp.Tools {
				if tool.Name == tc.ExpectedTool {
					foundTool = true
					break
				}
			}

			toolFoundResults[tc.Name] = foundTool

			if !foundTool {
				t.Logf("可用的工具: %v", resp.Tools)
				t.Logf("✗ 未找到期望的工具: %s", tc.ExpectedTool)
				results[tc.Name] = false
				toolCalledResults[tc.Name] = false
				return
			}

			t.Logf("✓ 找到工具: %s", tc.ExpectedTool)

			if resp.Answer == "" {
				t.Logf("✗ 响应为空")
				results[tc.Name] = false
				toolCalledResults[tc.Name] = false
				return
			}

			results[tc.Name] = true
			toolCalledResults[tc.Name] = true
			t.Logf("✓ 测试通过")
		})
	}

	t.Log("\n\n========== 技能测试汇总 ==========")
	passCount := 0
	failCount := 0
	for _, tc := range skillTests {
		status := "✗ FAIL"
		if results[tc.Name] {
			status = "✓ PASS"
			passCount++
		} else {
			failCount++
		}
		toolFound := "未找到"
		if toolFoundResults[tc.Name] {
			toolFound = "已找到"
		}
		toolCalled := "未调用"
		if toolCalledResults[tc.Name] {
			toolCalled = "已调用"
		}
		t.Logf("%-15s %s  [工具: %s, 执行: %s]", tc.Name, status, toolFound, toolCalled)
	}
	t.Logf("\n总计: %d 通过, %d 失败", passCount, failCount)
}

func TestSkillDetail(t *testing.T) {
	app := setupTestSuite(t)
	defer teardownTestSuite()

	brain := app.Assistant.GetBrain()

	for _, tc := range skillTests {
		t.Run(fmt.Sprintf("Detail_%s", tc.Name), func(t *testing.T) {
			req := &core.ThinkingRequest{
				Question: tc.Question,
				Timeout:  60,
			}

			resp, err := brain.Post(req)
			if err != nil {
				t.Logf("错误: %v", err)
				return
			}

			var toolNames []string
			for _, tool := range resp.Tools {
				toolNames = append(toolNames, tool.Name)
			}

			t.Logf("问题: %s", tc.Question)
			t.Logf("期望工具: %s", tc.ExpectedTool)
			t.Logf("实际工具: %v", toolNames)
			t.Logf("响应: %s", resp.Answer)

			var found bool
			for _, name := range toolNames {
				if name == tc.ExpectedTool {
					found = true
					break
				}
			}
			if !found {
				t.Logf("警告: 未找到期望的工具 %s", tc.ExpectedTool)
			}
		})
	}
}
