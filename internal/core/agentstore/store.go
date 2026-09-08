package agentstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// AgentStore 管理 agents/ 目录下的全部 Agent：
// 线程安全的内存缓存 + 目录读写。目录即注册表（全量加载），
// 雇佣过滤视图由 Hired() 提供。
type AgentStore struct {
	mu     sync.RWMutex
	dir    string
	agents map[string]*Agent
}

// Load 从 agentsDir 加载全部 Agent。
//
// 加载前自动执行旧单文件（{name}.md）的一次性迁移；迁移报告由
// MigrateReport 返回（文件名 → 是否成功），调用方据此记录日志。
// 目录不存在时返回空注册表（与"首次启动"语义一致）。
//
// 必传参数非空检查：dir 为空返回错误。
func Load(dir string) (*AgentStore, map[string]error, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, fmt.Errorf("agents 目录不能为空")
	}

	report := MigrateLegacyFiles(dir)

	store := &AgentStore{
		dir:    dir,
		agents: make(map[string]*Agent),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return store, report, nil
		}
		return nil, report, fmt.Errorf("读取 agents 目录 %s 失败: %w", dir, err)
	}

	var loadErrs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(dir, entry.Name())
		agent, err := loadAgentDir(dirPath)
		if err != nil {
			// 单个 Agent 损坏不拖垮整体加载，记录后继续
			loadErrs = append(loadErrs, fmt.Errorf("加载 %s: %w", dirPath, err))
			continue
		}
		store.agents[agent.Meta.Name] = agent
	}

	if len(loadErrs) > 0 {
		return store, report, fmt.Errorf("部分 Agent 加载失败: %v", loadErrs)
	}
	return store, report, nil
}

// Get 按名称返回 Agent（内存缓存），未找到返回 nil。
func (s *AgentStore) Get(name string) *Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[name]
}

// List 返回全部 Agent（按名称排序，保证输出稳定）。
func (s *AgentStore) List() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Agent, 0, len(s.agents))
	for _, a := range s.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta.Name < out[j].Meta.Name })
	return out
}

// Hired 返回雇佣视图：Hired == true 的 Agent（会话可用入口专用）。
func (s *AgentStore) Hired() []*Agent {
	var out []*Agent
	for _, a := range s.List() {
		if a.Meta.Hired {
			out = append(out, a)
		}
	}
	return out
}

// Exists 判断指定名称的 Agent 是否已注册。
func (s *AgentStore) Exists(name string) bool {
	if name == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.agents[name]
	return ok
}

// Dir 返回 agents 根目录（分发包安装等需要直接落盘的场景消费）。
func (s *AgentStore) Dir() string {
	return s.dir
}

// ExcludeToolsOf 返回指定 Agent 声明的排除工具集合，未找到时返回 nil。
// 作为会话创建参数下放给 goharness（WithExcludeTools 回调的数据源）。
func (s *AgentStore) ExcludeToolsOf(name string) []string {
	a := s.Get(name)
	if a == nil {
		return nil
	}
	return a.Meta.ExcludeTools
}

// agentDirPath 返回指定名称的 Agent 目录绝对路径（小写目录名，与旧单文件命名一致）。
func (s *AgentStore) agentDirPath(name string) string {
	return filepath.Join(s.dir, strings.ToLower(name))
}
