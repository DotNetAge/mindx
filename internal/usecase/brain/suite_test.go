package brain

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/capability"
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type mockScheduler struct {
	jobs map[string]*cron.Job
}

func (m *mockScheduler) Add(job *cron.Job) (string, error) {
	id := fmt.Sprintf("mock-job-%d", time.Now().UnixNano())
	m.jobs[id] = job
	return id, nil
}

func (m *mockScheduler) Delete(id string) error {
	delete(m.jobs, id)
	return nil
}

func (m *mockScheduler) List() ([]*cron.Job, error) {
	jobs := make([]*cron.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (m *mockScheduler) Get(id string) (*cron.Job, error) {
	return m.jobs[id], nil
}

func (m *mockScheduler) Pause(id string) error {
	return nil
}

func (m *mockScheduler) Resume(id string) error {
	return nil
}

func (m *mockScheduler) Update(id string, job *cron.Job) error {
	m.jobs[id] = job
	return nil
}

func (m *mockScheduler) RunJob(id string) error {
	return nil
}

func (m *mockScheduler) UpdateLastRun(id string, status cron.JobStatus, errMsg *string) error {
	return nil
}

func newMockScheduler() *mockScheduler {
	return &mockScheduler{
		jobs: make(map[string]*cron.Job),
	}
}

// BrainIntegrationSuite Brain 集成测试套件
// 使用真实组件（Memory、SkillMgr），不使用 Mock
// 注意：这些测试必须串行执行，因为 Ollama 无法并行处理大量请求
type BrainIntegrationSuite struct {
	suite.Suite
	brain      *core.Brain
	memory     *memory.Memory
	skillMgr   *skills.SkillMgr
	capMgr     *capability.CapabilityManager
	sessionMgr *session.SessionMgr
	srvCfg     *config.GlobalConfig
	testData   string
	testLogs   string
	logger     logging.Logger
}

// SetupSuite 初始化测试套件
func (s *BrainIntegrationSuite) SetupSuite() {
	s.logger = logging.GetSystemLogger().Named("brain_integration_test")
	s.testData = filepath.Join(os.TempDir(), fmt.Sprintf("bot_brain_test_%d_%d", time.Now().Unix(), os.Getpid()))
	s.testLogs = filepath.Join(s.testData, "logs")

	err := os.MkdirAll(s.testData, 0755)
	s.Require().NoError(err)
	err = os.MkdirAll(s.testLogs, 0755)
	s.Require().NoError(err)

	s.logger.Info("测试数据目录", logging.String("path", s.testData))

	if err := config.EnsureWorkspace(); err != nil {
		s.Require().NoError(err)
	}
	srvCfg, _, _, _, err := config.InitVippers()
	s.Require().NoError(err)
	s.srvCfg = srvCfg

	sessionStorage := session.NewFileSessionStorage(filepath.Join(s.testData, "sessions"))
	s.sessionMgr = session.NewSessionMgr(4096, sessionStorage, s.logger)
	err = s.sessionMgr.RestoreSession()
	if err != nil {
		s.logger.Warn("恢复会话失败", logging.Err(err))
	}

	store, err := persistence.NewStore(srvCfg.VectorStore.Type, filepath.Join(s.testData, "memory"), nil)
	s.Require().NoError(err)

	s.srvCfg.VectorStore.DataPath = filepath.Join(s.testData, "memory")
	s.memory, err = memory.NewMemory(s.srvCfg, nil, s.logger, store, nil)
	s.Require().NoError(err)
	s.logger.Info("Memory 初始化成功")

	installSkillsPath, err := config.GetInstallSkillsPath()
	s.Require().NoError(err)
	workspacePath, err := config.GetWorkspacePath()
	s.Require().NoError(err)

	s.skillMgr, err = skills.NewSkillMgr(installSkillsPath, workspacePath, nil, nil, s.logger)
	s.Require().NoError(err)
	s.logger.Info("SkillMgr 初始化成功")

	tokenUsageRepo, err := persistence.NewSQLiteTokenUsageRepository(filepath.Join(s.testData, "token_usage.db"))
	s.Require().NoError(err)

	historyRequest := func(maxCount int) ([]*core.DialogueMessage, error) {
		messages := s.sessionMgr.GetHistory()
		dialogueMessages := make([]*core.DialogueMessage, len(messages))
		for i, msg := range messages {
			dialogueMessages[i] = &core.DialogueMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}
		if maxCount > 0 && len(dialogueMessages) > maxCount {
			dialogueMessages = dialogueMessages[len(dialogueMessages)-maxCount:]
		}
		return dialogueMessages, nil
	}

	toolsRequest := func(keywords ...string) ([]*core.ToolSchema, error) {
		return []*core.ToolSchema{}, nil
	}

	capRequest := func(keywords ...string) (*entity.Capability, error) {
		return nil, fmt.Errorf("not implemented")
	}

	persona := &core.Persona{Name: "小柔", Gender: "女", Character: "温柔"}
	mockSched := newMockScheduler()
	s.brain, err = NewBrain(
		s.srvCfg,
		persona,
		s.memory,
		s.skillMgr,
		toolsRequest,
		capRequest,
		historyRequest,
		s.logger,
		tokenUsageRepo,
		mockSched,
	)
	s.Require().NoError(err)

	s.recordTestMemories()
}

// TearDownSuite 清理测试套件
func (s *BrainIntegrationSuite) TearDownSuite() {
	if s.memory != nil {
		s.memory.Close()
	}
	os.RemoveAll(s.testData)
	s.logger.Info("清理测试数据完成", logging.String("path", s.testData))
}

// recordTestMemories 记录测试记忆
func (s *BrainIntegrationSuite) recordTestMemories() {
	mem1 := core.MemoryPoint{
		Keywords:       []string{"编程", "代码", "开发"},
		Content:        "用户是一名程序员，喜欢使用 Go 语言进行开发",
		Summary:        "用户喜欢编程",
		TimeWeight:     1.0,
		RepeatWeight:   1.0,
		EmphasisWeight: 0.2,
		TotalWeight:    1.0,
		CreatedAt:      time.Now().Add(-24 * time.Hour),
	}
	err := s.memory.Record(mem1)
	s.Require().NoError(err)

	s.logger.Info("记录测试记忆成功")
}

// postWithHistory 包装 Post 方法，自动记录对话到会话
func (s *BrainIntegrationSuite) postWithHistory(req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
	err := s.sessionMgr.RecordMessage(entity.Message{
		Role:    "user",
		Content: req.Question,
		Time:    time.Now(),
	})
	if err != nil {
		s.logger.Warn("记录用户消息失败", logging.Err(err))
	}

	resp, err := s.brain.Post(req)
	if err != nil {
		return nil, err
	}

	err = s.sessionMgr.RecordMessage(entity.Message{
		Role:    "assistant",
		Content: resp.Answer,
		Time:    time.Now(),
	})
	if err != nil {
		s.logger.Warn("记录助手消息失败", logging.Err(err))
	}

	return resp, nil
}

// TestBrainIntegrationSuite 运行集成测试套件
func TestBrainIntegrationSuite(t *testing.T) {
	suite.Run(t, new(BrainIntegrationSuite))
}

// TestContextConsistencySuite 运行上下文一致性测试
func TestContextConsistencySuite(t *testing.T) {
	suite.Run(t, new(ContextConsistencySuite))
}

// TestMemoryReferenceSuite 运行记忆参考测试
func TestMemoryReferenceSuite(t *testing.T) {
	suite.Run(t, new(MemoryReferenceSuite))
}

// TestSkillExecutionSuite 运行技能执行测试
func TestSkillExecutionSuite(t *testing.T) {
	suite.Run(t, new(SkillExecutionSuite))
}

// TestLongInputSuite 运行超长文测试
func TestLongInputSuite(t *testing.T) {
	suite.Run(t, new(LongInputSuite))
}
