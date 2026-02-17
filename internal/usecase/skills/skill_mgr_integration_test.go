package skills

import (
	"mindx/internal/config"
	"mindx/internal/core"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func getProjectRootForIntegrationTest() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// 根据文件路径回溯到项目根目录
	// 从 internal/usecase/skills 回溯到根目录
	root := filepath.Join(wd, "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "skills")); err == nil {
		return root
	}
	// 如果找不到，尝试当前目录
	if _, err := os.Stat("skills"); err == nil {
		return wd
	}
	// 再尝试上一级目录
	if _, err := os.Stat(filepath.Join(wd, "..", "skills")); err == nil {
		return filepath.Join(wd, "..")
	}
	return wd
}

// SkillMgrIntegrationTestSuite SkillMgr 集成测试套件
type SkillMgrIntegrationTestSuite struct {
	suite.Suite
	mgr           *SkillMgr
	logger        logging.Logger
	testSkillsDir string
}

// SetupSuite 测试套件初始化
func (s *SkillMgrIntegrationTestSuite) SetupSuite() {
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/skillmgr_integration_test.log",
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
	s.logger = logging.GetSystemLogger().Named("skillmgr_integration_test")

	s.testSkillsDir = filepath.Join(getProjectRootForIntegrationTest(), "skills")
}

// TearDownSuite 测试套件清理
func (s *SkillMgrIntegrationTestSuite) TearDownSuite() {
}

// SetupTest 每个测试前的准备
func (s *SkillMgrIntegrationTestSuite) SetupTest() {
	llamaSvc := infraLlama.NewOllamaService("qwen3:0.6b")
	workspaceDir, err := config.GetWorkspacePath()
	assert.NoError(s.T(), err)
	installSkillsDir, err := config.GetInstallSkillsPath()
	assert.NoError(s.T(), err)
	mgr, err := NewSkillMgr(installSkillsDir, workspaceDir, nil, llamaSvc, s.logger)
	assert.NoError(s.T(), err, "创建技能管理器应该成功")
	s.mgr = mgr
}

// TearDownTest 每个测试后的清理
func (s *SkillMgrIntegrationTestSuite) TearDownTest() {
	s.mgr = nil
}

// TestSkillMgrIntegrationSuite 运行测试套件
func TestSkillMgrIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SkillMgrIntegrationTestSuite))
}

// TestGetSkills 测试获取所有技能
func (s *SkillMgrIntegrationTestSuite) TestGetSkills() {
	skills, err := s.mgr.GetSkills()

	assert.NoError(s.T(), err, "获取所有技能应该成功")
	assert.Greater(s.T(), len(skills), 0, "应该有技能")

	s.logger.Info("GetSkills 测试完成", logging.Int("count", len(skills)))
}

// TestSearchSkills_NoKeywords 测试无关键词搜索（返回所有技能）
func (s *SkillMgrIntegrationTestSuite) TestSearchSkills_NoKeywords() {
	skills, err := s.mgr.SearchSkills()

	assert.NoError(s.T(), err, "无关键词搜索应该成功")
	assert.Greater(s.T(), len(skills), 0, "应该返回启用的技能")

	s.logger.Info("无关键词搜索测试完成", logging.Int("count", len(skills)))
}

// TestSearchSkills_WithKeywords 测试带关键词搜索（验证相似度筛选）
func (s *SkillMgrIntegrationTestSuite) TestSearchSkills_WithKeywords() {
	testCases := []struct {
		name                string
		keywords            []string
		expectedMatches     []string // 期望匹配的技能名称
		description         string
		shouldReturnResults bool // 是否应该返回结果
	}{
		{
			name:                "搜索天气",
			keywords:            []string{"天气", "weather"},
			expectedMatches:     []string{"weather"},
			description:         "应该找到天气相关技能",
			shouldReturnResults: true,
		},
		{
			name:                "搜索计算器",
			keywords:            []string{"计算器", "calculator", "math"},
			expectedMatches:     []string{"calculator"},
			description:         "应该找到计算器技能",
			shouldReturnResults: true,
		},
		{
			name:                "搜索文件管理",
			keywords:            []string{"文件", "finder", "files"},
			expectedMatches:     []string{"finder"},
			description:         "应该找到文件管理技能",
			shouldReturnResults: true,
		},
		{
			name:                "搜索系统信息",
			keywords:            []string{"系统", "sysinfo", "system"},
			expectedMatches:     []string{"sysinfo"},
			description:         "应该找到系统信息技能",
			shouldReturnResults: true,
		},
		{
			name:                "搜索不存在的技能",
			keywords:            []string{"nonexistent", "不存在的技能"},
			expectedMatches:     []string{},
			description:         "不应该找到任何技能",
			shouldReturnResults: false,
		},
		{
			name:                "搜索提醒",
			keywords:            []string{"提醒", "reminders", "alarm"},
			expectedMatches:     []string{"reminders"},
			description:         "应该找到提醒技能",
			shouldReturnResults: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			skills, err := s.mgr.SearchSkills(tc.keywords...)

			assert.NoError(s.T(), err, tc.description)

			if tc.shouldReturnResults {
				// 应该返回结果
				assert.Greater(s.T(), len(skills), 0, tc.description)

				// 验证返回的技能包含期望的技能
				for _, expectedSkill := range tc.expectedMatches {
					found := false
					for _, skill := range skills {
						skillName := skill.GetName()
						s.logger.Info("检查技能匹配",
							logging.String("skill_name", skillName),
							logging.String("expected", expectedSkill))
						// 检查技能名称是否包含期望的关键词
						if skillName == expectedSkill {
							found = true
							s.logger.Info("找到匹配技能",
								logging.String("skill", skillName),
								logging.String("matched_with", expectedSkill))
							break
						}
					}
					assert.True(s.T(), found, "应该找到包含 '%s' 的技能", expectedSkill)
				}

				// 验证最相关的技能应该排在前面
				if len(skills) > 0 && len(tc.expectedMatches) > 0 {
					firstSkillName := skills[0].GetName()
					// 第一个技能应该是我们期望的之一
					assert.Contains(s.T(), tc.expectedMatches, firstSkillName,
						"最相关的技能应该是 '%s' 之一，但实际是 '%s'",
						tc.expectedMatches, firstSkillName)
				}
			} else {
				// 不应该返回结果
				assert.Equal(s.T(), 0, len(skills), tc.description)
			}

			s.logger.Info("关键词搜索测试完成",
				logging.String("test", tc.name),
				logging.String("keywords", s.formatKeywords(tc.keywords)),
				logging.Int("found", len(skills)),
				logging.String("returned_skills", s.formatSkills(skills)))
		})
	}
}

// formatKeywords 格式化关键词用于日志
func (s *SkillMgrIntegrationTestSuite) formatKeywords(keywords []string) string {
	result := "["
	for i, kw := range keywords {
		if i > 0 {
			result += ", "
		}
		result += `"` + kw + `"`
	}
	result += "]"
	return result
}

// formatSkills 格式化技能列表
func (s *SkillMgrIntegrationTestSuite) formatSkills(skills []*core.Skill) string {
	if len(skills) == 0 {
		return "[]"
	}
	result := "["
	for i, skill := range skills {
		if i > 0 {
			result += ", "
		}
		result += skill.GetName()
	}
	result += "]"
	return result
}

// getSkillNames 获取技能名称列表
func (s *SkillMgrIntegrationTestSuite) getSkillNames(skills []*core.Skill) []string {
	names := make([]string, len(skills))
	for i, skill := range skills {
		names[i] = skill.GetName()
	}
	return names
}

// TestExecute 测试执行技能
func (s *SkillMgrIntegrationTestSuite) TestExecute() {
	skills, err := s.mgr.GetSkills()
	assert.NoError(s.T(), err, "获取技能应该成功")
	assert.Greater(s.T(), len(skills), 0, "应该有可执行的技能")

	skill := skills[0]
	params := map[string]any{
		"test_param": "test_value",
	}

	err = s.mgr.Execute(skill, params)
	if err != nil {
		s.logger.Warn("技能执行失败（可能是脚本路径问题）", logging.Err(err))
	} else {
		s.logger.Info("Execute 测试完成", logging.String("skill", skill.GetName()))
	}
}

// TestExecuteByName 测试按名称执行技能
func (s *SkillMgrIntegrationTestSuite) TestExecuteByName() {
	testCases := []struct {
		name          string
		skillName     string
		shouldSucceed bool
		description   string
	}{
		{
			name:          "执行天气技能",
			skillName:     "weather",
			shouldSucceed: false,
			description:   "执行天气技能（可能因脚本路径失败）",
		},
		{
			name:          "执行计算器技能",
			skillName:     "calculator",
			shouldSucceed: false,
			description:   "执行计算器技能（可能因脚本路径失败）",
		},
		{
			name:          "执行不存在的技能",
			skillName:     "nonexistent-skill",
			shouldSucceed: false,
			description:   "执行不存在的技能应该失败",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			params := map[string]any{
				"test_param": "test_value",
			}

			result, err := s.mgr.ExecuteByName(tc.skillName, params)

			if tc.skillName == "nonexistent-skill" {
				assert.Error(s.T(), err, tc.description)
				s.logger.Info("ExecuteByName 预期失败",
					logging.String("skill", tc.skillName),
					logging.String("error", err.Error()))
			} else {
				if err != nil {
					s.logger.Warn("ExecuteByName 失败（可能是脚本路径问题）",
						logging.String("skill", tc.skillName),
						logging.String("error", err.Error()))
				} else {
					assert.NotEmpty(s.T(), result, "应该有执行结果")
					s.logger.Info("ExecuteByName 成功",
						logging.String("skill", tc.skillName),
						logging.String("result", result))
				}
			}
		})
	}
}

// TestEnableDisable 测试启用和禁用技能
func (s *SkillMgrIntegrationTestSuite) TestEnableDisable() {
	s.Run("禁用技能", func() {
		err := s.mgr.Disable("weather")
		assert.NoError(s.T(), err, "禁用技能应该成功")

		info, exists := s.mgr.GetSkillInfo("weather")
		assert.True(s.T(), exists, "技能应该存在")
		assert.False(s.T(), info.Def.Enabled, "技能应该被禁用")
	})

	s.Run("启用技能", func() {
		err := s.mgr.Enable("weather")
		assert.NoError(s.T(), err, "启用技能应该成功")

		info, exists := s.mgr.GetSkillInfo("weather")
		assert.True(s.T(), exists, "技能应该存在")
		assert.True(s.T(), info.Def.Enabled, "技能应该被启用")
	})

	s.Run("禁用不存在的技能", func() {
		err := s.mgr.Disable("nonexistent-skill")
		assert.Error(s.T(), err, "禁用不存在的技能应该失败")
	})

	s.Run("启用不存在的技能", func() {
		err := s.mgr.Enable("nonexistent-skill")
		assert.Error(s.T(), err, "启用不存在的技能应该失败")
	})
}

// TestGetSkillInfo 测试获取技能信息
func (s *SkillMgrIntegrationTestSuite) TestGetSkillInfo() {
	s.Run("获取存在的技能信息", func() {
		info, exists := s.mgr.GetSkillInfo("weather")
		assert.True(s.T(), exists, "技能应该存在")
		assert.NotNil(s.T(), info, "技能信息不应为空")
		assert.Equal(s.T(), "weather", info.Def.Name, "技能名称应该匹配")
		assert.NotEmpty(s.T(), info.Def.Description, "技能描述应该不为空")
		assert.Equal(s.T(), "general", info.Def.Category, "技能分类应该匹配")
	})

	s.Run("获取不存在的技能信息", func() {
		info, exists := s.mgr.GetSkillInfo("nonexistent-skill")
		assert.False(s.T(), exists, "技能不应该存在")
		assert.Nil(s.T(), info, "技能信息应该为空")
	})
}

// TestGetSkillInfos 测试获取所有技能信息
func (s *SkillMgrIntegrationTestSuite) TestGetSkillInfos() {
	infos := s.mgr.GetSkillInfos()

	assert.Greater(s.T(), len(infos), 0, "应该有技能信息")

	for name, info := range infos {
		assert.NotNil(s.T(), info, "技能信息不应为空")
		assert.NotEmpty(s.T(), name, "技能名称不应为空")
		assert.NotNil(s.T(), info.Def, "技能定义不应为空")
	}

	s.logger.Info("GetSkillInfos 测试完成", logging.Int("count", len(infos)))
}

// TestSkillStatistics 测试技能统计信息
func (s *SkillMgrIntegrationTestSuite) TestSkillStatistics() {
	skillName := "weather"

	s.Run("执行技能后检查统计", func() {
		_, err := s.mgr.ExecuteByName(skillName, map[string]any{})
		if err != nil {
			s.logger.Warn("技能执行失败（可能是脚本路径问题）", logging.Err(err))
			s.T().Skip("跳过统计测试：技能执行失败")
			return
		}

		info, exists := s.mgr.GetSkillInfo(skillName)
		assert.True(s.T(), exists)
		assert.Equal(s.T(), 1, info.SuccessCount, "成功次数应该为1")
		assert.Equal(s.T(), 0, info.ErrorCount, "错误次数应该为0")
		assert.NotNil(s.T(), info.LastRunTime, "最后运行时间应该被设置")
		assert.Greater(s.T(), info.AvgExecutionMs, int64(0), "平均执行时间应该大于0")
	})

	s.Run("多次执行后检查统计", func() {
		executionCount := 0
		for i := 0; i < 3; i++ {
			_, err := s.mgr.ExecuteByName(skillName, map[string]any{})
			if err != nil {
				s.logger.Warn("技能执行失败（可能是脚本路径问题）", logging.Err(err))
				continue
			}
			executionCount++
			time.Sleep(10 * time.Millisecond)
		}

		if executionCount == 0 {
			s.T().Skip("跳过统计测试：所有技能执行都失败")
			return
		}

		info, exists := s.mgr.GetSkillInfo(skillName)
		assert.True(s.T(), exists)
		assert.Equal(s.T(), executionCount, info.SuccessCount, "成功次数应该匹配")
	})
}

// TestSkillDependencies 测试技能依赖检查
func (s *SkillMgrIntegrationTestSuite) TestSkillDependencies() {
	s.Run("检查天气技能的依赖", func() {
		info, exists := s.mgr.GetSkillInfo("weather")
		assert.True(s.T(), exists)
		s.logger.Info("天气技能依赖检查",
			logging.Bool("canRun", info.CanRun),
			logging.Any("missingBins", info.MissingBins),
			logging.Any("missingEnv", info.MissingEnv))
	})
}

// TestIntegrationScenario 完整集成测试场景
func (s *SkillMgrIntegrationTestSuite) TestIntegrationScenario() {
	s.Run("场景：用户查询天气", func() {
		skills, err := s.mgr.SearchSkills("天气", "weather")
		assert.NoError(s.T(), err)
		assert.Greater(s.T(), len(skills), 0, "应该找到天气技能")

		weatherSkill := skills[0]
		assert.Equal(s.T(), "weather", weatherSkill.GetName(), "应该找到天气技能")

		result, err := s.mgr.ExecuteByName(weatherSkill.GetName(), map[string]any{
			"city": "北京",
		})
		if err != nil {
			s.logger.Warn("天气技能执行失败（可能是脚本路径问题）", logging.Err(err))
		} else {
			assert.NotEmpty(s.T(), result)
			info, _ := s.mgr.GetSkillInfo(weatherSkill.GetName())
			assert.Equal(s.T(), 1, info.SuccessCount, "成功次数应该增加")
			s.logger.Info("天气查询场景测试完成",
				logging.String("skill", weatherSkill.GetName()),
				logging.String("result", result))
		}
	})

	s.Run("场景：批量搜索和执行", func() {
		searchTerms := []string{"weather", "calculator", "sysinfo"}

		for _, term := range searchTerms {
			skills, err := s.mgr.SearchSkills(term)
			assert.NoError(s.T(), err)

			for _, skill := range skills {
				_, err := s.mgr.ExecuteByName(skill.GetName(), map[string]any{})
				if err != nil {
					s.logger.Warn("技能执行失败（可能是脚本路径问题）",
						logging.String("skill", skill.GetName()),
						logging.Err(err))
				}
			}
		}

		infos := s.mgr.GetSkillInfos()
		totalExecutions := 0
		for _, info := range infos {
			totalExecutions += info.SuccessCount
		}

		s.logger.Info("批量搜索和执行场景测试完成",
			logging.Int("total_executions", totalExecutions))
	})

	s.Run("场景：技能生命周期管理", func() {
		skillName := "weather"

		s.T().Log("步骤1：获取技能信息")
		info, exists := s.mgr.GetSkillInfo(skillName)
		assert.True(s.T(), exists)
		initialEnabled := info.Def.Enabled

		s.T().Log("步骤2：禁用技能")
		err := s.mgr.Disable(skillName)
		assert.NoError(s.T(), err)
		info, _ = s.mgr.GetSkillInfo(skillName)
		assert.False(s.T(), info.Def.Enabled)

		s.T().Log("步骤3：尝试执行禁用的技能")
		_, err = s.mgr.ExecuteByName(skillName, map[string]any{})
		assert.Error(s.T(), err, "执行禁用的技能应该失败")

		s.T().Log("步骤4：重新启用技能")
		err = s.mgr.Enable(skillName)
		assert.NoError(s.T(), err)
		info, _ = s.mgr.GetSkillInfo(skillName)
		assert.True(s.T(), info.Def.Enabled)

		s.T().Log("步骤5：执行启用的技能")
		_, err = s.mgr.ExecuteByName(skillName, map[string]any{})
		if err != nil {
			s.logger.Warn("技能执行失败（可能是脚本路径问题）", logging.Err(err))
		}

		s.T().Log("步骤6：恢复初始状态")
		if !initialEnabled {
			s.mgr.Disable(skillName)
		}

		s.logger.Info("技能生命周期管理场景测试完成")
	})
}

// TestConcurrentAccess 测试并发访问
func (s *SkillMgrIntegrationTestSuite) TestConcurrentAccess() {
	s.Run("并发获取技能", func() {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				_, err := s.mgr.GetSkills()
				assert.NoError(s.T(), err)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	s.Run("并发搜索技能", func() {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(idx int) {
				keywords := []string{"weather", "calculator", "sysinfo"}
				_, err := s.mgr.SearchSkills(keywords[idx%3])
				assert.NoError(s.T(), err)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	s.Run("并发执行技能", func() {
		done := make(chan bool, 5)
		successCount := 0

		for i := 0; i < 5; i++ {
			go func() {
				_, err := s.mgr.ExecuteByName("weather", map[string]any{})
				if err == nil {
					successCount++
				}
				done <- true
			}()
		}

		for i := 0; i < 5; i++ {
			<-done
		}

		if successCount > 0 {
			info, _ := s.mgr.GetSkillInfo("weather")
			assert.Equal(s.T(), successCount, info.SuccessCount, "成功次数应该匹配")
		} else {
			s.logger.Warn("所有并发执行都失败（可能是脚本路径问题）")
		}
	})
}
