package skills

import (
	"fmt"
	"mindx/internal/config"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/pkg/logging"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAllSkillsRealExecution 对所有已装载的技能进行真实的运行测试
func TestAllSkillsRealExecution(t *testing.T) {
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/skills_real_test.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   false,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable:     false,
			OutputPath: "/tmp/conversation.log",
		},
	}
	_ = logging.Init(logConfig)
	logger := logging.GetSystemLogger().Named("skills_real_test")

	testSkillsDir, err := config.GetInstallSkillsPath()
	assert.NoError(t, err)
	workspaceDir, err := config.GetWorkspacePath()
	assert.NoError(t, err)
	llamaSvc := infraLlama.NewOllamaService("qwen2.5:7b")
	mgr, err := NewSkillMgr(testSkillsDir, workspaceDir, nil, llamaSvc, logger)
	assert.NoError(t, err, "创建技能管理器应该成功")

	skills, err := mgr.GetSkills()
	assert.NoError(t, err, "获取技能应该成功")
	assert.Greater(t, len(skills), 0, "应该有技能")

	logger.Info("开始对所有技能进行真实运行测试", logging.Int("total_skills", len(skills)))

	testResults := make(map[string]*SkillTestResult)
	successCount := 0
	skipCount := 0
	failCount := 0

	for _, skill := range skills {
		skillName := skill.GetName()
		result := &SkillTestResult{
			Name:   skillName,
			Status: "unknown",
		}

		t.Run(fmt.Sprintf("测试技能: %s", skillName), func(t *testing.T) {
			logger.Info("开始测试技能", logging.String("skill", skillName))

			testParams := getTestParamsForSkill(skillName)

			err := mgr.Execute(skill, testParams)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				failCount++
				logger.Warn("技能执行失败",
					logging.String("skill", skillName),
					logging.Err(err),
					logging.String("params", fmt.Sprintf("%v", testParams)))
			} else {
				result.Status = "success"
				successCount++
				logger.Info("技能执行成功",
					logging.String("skill", skillName),
					logging.String("params", fmt.Sprintf("%v", testParams)))

				info, exists := mgr.GetSkillInfo(skillName)
				if exists {
					result.ExecutionTime = info.AvgExecutionMs
					result.LastRunTime = info.LastRunTime
				}
			}

			testResults[skillName] = result
		})
	}

	printTestSummary(logger, testResults, successCount, skipCount, failCount)
}

// SkillTestResult 技能测试结果
type SkillTestResult struct {
	Name          string
	Status        string
	Error         string
	ExecutionTime int64
	LastRunTime   any
}

// getTestParamsForSkill 为特定技能获取测试参数
func getTestParamsForSkill(skillName string) map[string]any {
	switch skillName {
	case "weather":
		return map[string]any{
			"city": "北京",
		}
	case "calculator":
		return map[string]any{
			"expression": "2+2",
		}
	case "finder":
		return map[string]any{
			"query": ".",
		}
	case "sysinfo":
		return map[string]any{
			"type": "overview",
		}
	case "calendar":
		return map[string]any{
			"action": "list",
		}
	case "contacts":
		return map[string]any{
			"action": "list",
		}
	case "notes":
		return map[string]any{
			"action": "list",
		}
	case "reminders":
		return map[string]any{
			"action": "list",
		}
	case "wifi":
		return map[string]any{
			"action": "status",
		}
	case "volume":
		return map[string]any{
			"action": "get",
		}
	case "search":
		return map[string]any{
			"action":  "files",
			"pattern": "test",
		}
	case "screenshot":
		return map[string]any{
			"type":     "screen",
			"filename": "/tmp/test_screenshot.png",
		}
	case "terminal":
		return map[string]any{
			"command": "echo 'hello'",
		}
	case "mail":
		return map[string]any{
			"action": "list",
		}
	case "openurl":
		return map[string]any{
			"url": "https://www.apple.com",
		}
	case "notify":
		return map[string]any{
			"title":   "测试通知",
			"message": "这是一个测试通知",
		}
	case "clipboard":
		return map[string]any{
			"action": "get",
		}
	case "imessage":
		return map[string]any{
			"action": "list",
		}
	case "voice":
		return map[string]any{
			"action": "status",
		}
	default:
		return map[string]any{
			"test": "true",
		}
	}
}

// printTestSummary 打印测试摘要
func printTestSummary(logger logging.Logger, results map[string]*SkillTestResult, successCount, skipCount, failCount int) {
	logger.Info("========================================")
	logger.Info("技能测试摘要")
	logger.Info("========================================")
	logger.Info("总技能数", logging.Int("total", len(results)))
	logger.Info("成功", logging.Int("success", successCount))
	logger.Info("跳过", logging.Int("skipped", skipCount))
	logger.Info("失败", logging.Int("failed", failCount))
	logger.Info("========================================")

	logger.Info("详细结果:")
	for name, result := range results {
		statusIcon := "✅"
		if result.Status == "failed" {
			statusIcon = "❌"
		} else if result.Status == "skipped" {
			statusIcon = "⏭️"
		}

		logger.Info(fmt.Sprintf("%s %s - %s", statusIcon, name, result.Status))
		if result.Error != "" {
			logger.Warn("  错误信息", logging.String("error", result.Error))
		}
		if result.ExecutionTime > 0 {
			logger.Info("  执行时间", logging.Int64("ms", result.ExecutionTime))
		}
	}
	logger.Info("========================================")

	失败技能列表 := make([]string, 0)
	成功技能列表 := make([]string, 0)
	for name, result := range results {
		if result.Status == "failed" {
			失败技能列表 = append(失败技能列表, name)
		} else if result.Status == "success" {
			成功技能列表 = append(成功技能列表, name)
		}
	}

	if len(失败技能列表) > 0 {
		logger.Warn("失败的技能:", logging.String("list", strings.Join(失败技能列表, ", ")))
	}
	if len(成功技能列表) > 0 {
		logger.Info("成功的技能:", logging.String("list", strings.Join(成功技能列表, ", ")))
	}
}

// TestSkillExecutionWithDetailedLogging 详细日志测试
func TestSkillExecutionWithDetailedLogging(t *testing.T) {
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/skills_detailed_test.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   false,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable:     false,
			OutputPath: "/tmp/conversation.log",
		},
	}
	_ = logging.Init(logConfig)
	logger := logging.GetSystemLogger().Named("skills_detailed_test")

	testSkillsDir, err := config.GetInstallSkillsPath()
	assert.NoError(t, err)
	workspaceDir, err := config.GetWorkspacePath()
	assert.NoError(t, err)
	llamaSvc := infraLlama.NewOllamaService("qwen2.5:7b")
	mgr, err := NewSkillMgr(testSkillsDir, workspaceDir, nil, llamaSvc, logger)
	assert.NoError(t, err, "创建技能管理器应该成功")

	skills, err := mgr.GetSkills()
	assert.NoError(t, err, "获取技能应该成功")

	logger.Info("开始详细日志测试", logging.Int("total_skills", len(skills)))

	for _, skill := range skills {
		skillName := skill.GetName()

		t.Run(fmt.Sprintf("详细测试: %s", skillName), func(t *testing.T) {
			logger.Info("========================================")
			logger.Info("开始详细测试", logging.String("skill", skillName))

			info, exists := mgr.GetSkillInfo(skillName)
			if exists {
				logger.Info("技能信息",
					logging.String("name", info.Def.Name),
					logging.String("description", info.Def.Description),
					logging.String("category", info.Def.Category),
					logging.Bool("enabled", info.Def.Enabled),
					logging.Bool("can_run", info.CanRun))
				if len(info.MissingBins) > 0 {
					logger.Warn("缺少的可执行文件", logging.Any("missing_bins", info.MissingBins))
				}
				if len(info.MissingEnv) > 0 {
					logger.Warn("缺少的环境变量", logging.Any("missing_env", info.MissingEnv))
				}
			}

			testParams := getTestParamsForSkill(skillName)
			logger.Info("测试参数", logging.String("params", fmt.Sprintf("%v", testParams)))

			err := mgr.Execute(skill, testParams)
			if err != nil {
				logger.Error("执行失败", logging.String("skill", skillName), logging.Err(err))
			} else {
				logger.Info("执行成功", logging.String("skill", skillName))

				info, exists := mgr.GetSkillInfo(skillName)
				if exists {
					logger.Info("执行统计",
						logging.Int("success_count", info.SuccessCount),
						logging.Int("error_count", info.ErrorCount),
						logging.Int64("avg_execution_ms", info.AvgExecutionMs),
						logging.Any("last_run_time", info.LastRunTime))
				}
			}

			logger.Info("========================================")
		})
	}
}
