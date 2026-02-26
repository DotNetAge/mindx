package brain

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/capability"
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/embedding"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type mockScheduler struct {
	mu   sync.Mutex
	jobs map[string]*cron.Job
}

func (m *mockScheduler) Add(job *cron.Job) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("mock-job-%d", time.Now().UnixNano())
	job.ID = id
	m.jobs[id] = job
	return id, nil
}

func (m *mockScheduler) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, id)
	return nil
}

func (m *mockScheduler) List() ([]*cron.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]*cron.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (m *mockScheduler) Get(id string) (*cron.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id], nil
}

func (m *mockScheduler) Pause(id string) error  { return nil }
func (m *mockScheduler) Resume(id string) error { return nil }

func (m *mockScheduler) Update(id string, job *cron.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[id] = job
	return nil
}

func (m *mockScheduler) RunJob(id string) error { return nil }

func (m *mockScheduler) UpdateLastRun(id string, status cron.JobStatus, errMsg *string) error {
	return nil
}

func (m *mockScheduler) getJobs() []*cron.Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]*cron.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (m *mockScheduler) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = make(map[string]*cron.Job)
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
	cronMock   *mockScheduler
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

	installSkillsPath := getProjectRootSkillsDir(s.T())
	workspacePath, err := config.GetWorkspacePath()
	s.Require().NoError(err)

	// 初始化 embedding 和 llama 服务，支持向量搜索
	embeddingModel := os.Getenv("MINDX_TEST_EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "qllama/bge-small-zh-v1.5:latest"
	}
	var embeddingSvc *embedding.EmbeddingService
	provider, embErr := infraEmbedding.NewOllamaEmbedding("http://localhost:11434", embeddingModel)
	if embErr == nil {
		embeddingSvc = embedding.NewEmbeddingService(provider)
	} else {
		s.logger.Warn("embedding 模型不可用，向量搜索将退化为关键字搜索", logging.Err(embErr))
	}

	testModel := os.Getenv("MINDX_TEST_MODEL")
	if testModel == "" {
		testModel = "qwen3:0.6b"
	}
	llamaSvc := infraLlama.NewOllamaService(testModel)

	skillStore, err := persistence.NewStore("badger", filepath.Join(s.testData, "skill_vectors"), nil)
	s.Require().NoError(err)

	s.skillMgr, err = skills.NewSkillMgrWithStore(installSkillsPath, workspacePath, embeddingSvc, llamaSvc, skillStore, s.logger)
	s.Require().NoError(err)

	// 建立向量索引
	if embeddingSvc != nil {
		if reindexErr := s.skillMgr.ReIndex(); reindexErr != nil {
			s.logger.Warn("向量索引建立失败", logging.Err(reindexErr))
		} else {
			s.logger.Info("向量索引建立成功")
		}
	}
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

	toolCaller := NewToolCaller(s.skillMgr, s.logger)
	toolsRequest := func(keywords ...string) ([]*core.ToolSchema, error) {
		schemas, err := toolCaller.SearchTools(keywords)
		if err != nil {
			return nil, err
		}
		ptrs := make([]*core.ToolSchema, len(schemas))
		for i := range schemas {
			ptrs[i] = &schemas[i]
		}
		return ptrs, nil
	}

	capRequest := func(keywords ...string) (*entity.Capability, error) {
		return nil, fmt.Errorf("not implemented")
	}

	persona := &core.Persona{Name: "小柔", Gender: "女", Character: "温柔"}
	s.cronMock = newMockScheduler()
	s.brain, err = NewBrain(BrainDeps{
		Cfg:            s.srvCfg,
		Persona:        persona,
		Memory:         s.memory,
		SkillMgr:       s.skillMgr,
		ToolsRequest:   toolsRequest,
		CapRequest:     capRequest,
		HistoryRequest: historyRequest,
		Logger:         s.logger,
		TokenUsageRepo: tokenUsageRepo,
		CronScheduler:  s.cronMock,
	})
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

