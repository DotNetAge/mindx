package teamstore

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Store 管理 teams/ 目录下的全部团队：
// 线程安全的内存缓存 + 目录读写。目录即注册表（全量加载），
// 雇佣过滤视图由 Hired() 提供。
type Store struct {
	mu    sync.RWMutex
	dir   string
	teams map[string]*Team
}

// Load 从 teamsDir 加载全部团队。
//
// 目录即注册表：只认子目录，单个目录损坏不拖垮整体加载（记入报告）。
// 目录不存在时返回空注册表（与"首次启动"语义一致）。
//
// 必传参数非空检查：dir 为空返回错误。
func Load(dir string) (*Store, map[string]error, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, fmt.Errorf("teams 目录不能为空")
	}

	store := &Store{
		dir:   dir,
		teams: make(map[string]*Team),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil, nil
		}
		return nil, nil, fmt.Errorf("读取 teams 目录 %s 失败: %w", dir, err)
	}

	var loadErrs = make(map[string]error)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(dir, entry.Name())
		team, err := loadTeamDir(dirPath)
		if err != nil {
			// 单个团队损坏不拖垮整体加载，记录后继续
			loadErrs[entry.Name()] = err
			continue
		}
		store.teams[team.Meta.Name] = team
	}

	if len(loadErrs) > 0 {
		return store, loadErrs, fmt.Errorf("部分团队加载失败")
	}
	return store, loadErrs, nil
}

// Get 按名称返回团队（内存缓存），未找到返回 nil。
func (s *Store) Get(name string) *Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.teams[name]
}

// List 返回全部团队（按名称排序，保证输出稳定）。
func (s *Store) List() []*Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Team, 0, len(s.teams))
	for _, t := range s.teams {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta.Name < out[j].Meta.Name })
	return out
}

// Hired 返回雇佣视图：Hired == true 的团队（会话可用入口专用）。
func (s *Store) Hired() []*Team {
	var out []*Team
	for _, t := range s.List() {
		if t.Meta.Hired {
			out = append(out, t)
		}
	}
	return out
}

// Exists 判断指定名称的团队是否已注册。
func (s *Store) Exists(name string) bool {
	if name == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.teams[name]
	return ok
}

// Dir 返回 teams 根目录。
func (s *Store) Dir() string {
	return s.dir
}

// FindByLeader 返回以指定代理为负责人的团队（多个时按名称排序取首个，
// 负责人唯一性由上层迁移器与写入路径保证），未找到返回 nil。
// 仅用于配置态展示（is_leader 投影、选择器去重），不驱动注入——
// 注入由 Session 绑定的 TeamName 驱动。
func (s *Store) FindByLeader(agentName string) *Team {
	if agentName == "" {
		return nil
	}
	for _, t := range s.List() {
		if t.Meta.Leader == agentName {
			return t
		}
	}
	return nil
}

// TeamOf 返回指定代理作为成员或负责人所属的团队列表（按名称排序）。
// 同一代理可被多个团队引用；不属于任何团队时返回空。
func (s *Store) TeamOf(agentName string) []*Team {
	var out []*Team
	for _, t := range s.List() {
		if t.Meta.Leader == agentName || slices.Contains(t.Meta.Members, agentName) {
			out = append(out, t)
		}
	}
	return out
}

// teamDirPath 返回指定名称的团队目录绝对路径（小写目录名，与 agentstore 一致）。
func (s *Store) teamDirPath(name string) string {
	return filepath.Join(s.dir, strings.ToLower(name))
}
