package skills

import (
	"mindx/internal/config"
	"mindx/internal/core"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"

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
	llamaSvc := infraLlama.NewOllamaService(getTestModelName())
	workspaceDir, err := config.GetWorkspacePath()
	if !assert.NoError(s.T(), err) {
		return
	}
	mgr, err := NewSkillMgr(s.testSkillsDir, workspaceDir, nil, llamaSvc, s.logger)
	if !assert.NoError(s.T(), err, "创建技能管理器应该成功") {
		return
	}
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

// TestExecuteByName_NotFound 测试按名称执行不存在的技能
func (s *SkillMgrIntegrationTestSuite) TestExecuteByName_NotFound() {
	_, err := s.mgr.ExecuteByName("nonexistent-skill", map[string]any{})
	assert.Error(s.T(), err, "执行不存在的技能应该失败")
	assert.Contains(s.T(), err.Error(), "not found")
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

// TestSkillsHaveTags 验证加载的技能有 tags（tags 会被注入到 SkillKeywords 用于意图识别）
func (s *SkillMgrIntegrationTestSuite) TestSkillsHaveTags() {
	infos := s.mgr.GetSkillInfos()
	assert.Greater(s.T(), len(infos), 0, "应该有技能")

	skillsWithTags := 0
	for _, info := range infos {
		if info.Def != nil && len(info.Def.Tags) > 0 {
			skillsWithTags++
		}
	}
	assert.Greater(s.T(), skillsWithTags, 0, "至少应有一个技能带有 tags")
}

// TestConcurrentAccess 测试并发访问
func (s *SkillMgrIntegrationTestSuite) TestConcurrentAccess() {
	s.Run("并发获取技能", func() {
		mgr := s.mgr
		if mgr == nil {
			s.T().Skip("SkillMgr 初始化失败，跳过并发测试")
		}
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				_, err := mgr.GetSkills()
				assert.NoError(s.T(), err)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	s.Run("并发搜索技能", func() {
		mgr := s.mgr
		if mgr == nil {
			s.T().Skip("SkillMgr 初始化失败，跳过并发测试")
		}
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(idx int) {
				keywords := []string{"weather", "calculator", "sysinfo"}
				_, err := mgr.SearchSkills(keywords[idx%3])
				assert.NoError(s.T(), err)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
