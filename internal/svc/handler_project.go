package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// projectListResult 是 project.list 的返回投影：项目维度的会话聚合。
//   - Leader：项目会话 Sponsor 中命中团队负责人（agentstore IsLeader 派生）者，
//     多个时取其会话最近活跃的一个；空 = 未委派负责人（用户亲自 handle）。
//   - LastAgent / LastActivityAt：项目内最近活跃会话的 Sponsor 与活动时间。
type projectListResult struct {
	Name           string    `json:"name"`                 // 项目名（project_dir 基名）
	ProjectDir     string    `json:"project_dir"`          // 项目工作目录（去重键）
	Leader         string    `json:"leader,omitempty"`     // 委派的团队负责人；空 = 用户亲自负责
	LastAgent      string    `json:"last_agent,omitempty"` // 最后活跃会话的 Sponsor；空 = 用户
	LastActivityAt time.Time `json:"last_activity_at"`     // 项目最后活跃时间
	SessionCount   int       `json:"session_count"`        // 项目下会话数
}

// projectAggregate 是单个 project_dir 的聚合中间态。
type projectAggregate struct {
	dir       string
	leader    string
	leaderAt  time.Time
	lastAgent string
	lastAt    time.Time
	count     int
}

// handleProjectList 枚举全部工作目录分片的会话（RoutedSessionStore 由
// session_dirs.json 清单驱动，天然覆盖全系统），按 ProjectDir 去重聚合为
// 项目列表，按最后活跃时间降序。
func (d *Daemon) handleProjectList(_ context.Context, _ json.RawMessage) (any, error) {
	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	sessions, err := goharnesssession.ListSessions(context.Background(), sessDB)
	if err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}

	// 团队负责人候选集（Members 非空即负责人，派生值不落盘）：
	// 项目会话的 Sponsor 命中候选集即视为该项目已委派负责人。
	leaderAgents := make(map[string]bool)
	if ag := d.app.Agents(); ag != nil {
		for _, a := range ag.List() {
			if a.IsLeader() {
				leaderAgents[a.Name()] = true
			}
		}
	}

	byDir := make(map[string]*projectAggregate)
	for _, s := range sessions {
		if strings.TrimSpace(s.ProjectDir) == "" {
			continue // 防御：无归属工作目录的会话不参与项目聚合
		}
		agg, ok := byDir[s.ProjectDir]
		if !ok {
			agg = &projectAggregate{dir: s.ProjectDir}
			byDir[s.ProjectDir] = agg
		}
		agg.count++
		if s.LastActivityAt.After(agg.lastAt) {
			agg.lastAt = s.LastActivityAt
			agg.lastAgent = s.Sponsor
		}
		if leaderAgents[s.Sponsor] && s.LastActivityAt.After(agg.leaderAt) {
			agg.leaderAt = s.LastActivityAt
			agg.leader = s.Sponsor
		}
	}

	result := make([]projectListResult, 0, len(byDir))
	for _, agg := range byDir {
		result = append(result, projectListResult{
			Name:           filepath.Base(agg.dir),
			ProjectDir:     agg.dir,
			Leader:         agg.leader,
			LastAgent:      agg.lastAgent,
			LastActivityAt: agg.lastAt,
			SessionCount:   agg.count,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastActivityAt.After(result[j].LastActivityAt)
	})
	return result, nil
}
