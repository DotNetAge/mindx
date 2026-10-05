package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
)

// MigrationLogger 是迁移器的日志依赖（结构化最小接口，兼容 *logging.ZapLogger）。
type MigrationLogger interface {
	Warn(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
}

// MigrateLegacyTeams 将旧"弱一致固定组队"（Agent frontmatter 的 team/members
// 字段 + 负责人目录内 TEAM.md）一次性迁移为一等 Team 实体，并清空 Agent 侧
// 已退役的键。启动时执行；幂等——team/members 全空时零开销。
//
// 迁移规则：
//   - 负责人（members 非空）：以 Meta.Team 为团队名（为空时以负责人名兜底）建队
//     （Leader=该 Agent、Members=其 members、Profile=其 TEAM.md 正文、
//     Hired 继承负责人雇佣态）；同名队已存在则合并成员集（首个建队者保持 Leader）；
//   - 成员（team 非空且 members 空）：指向已建团队时保真并入其成员集；
//     指向不存在的团队仅告警（没有负责人信息的队不建空队）；
//   - 所有被迁移的 Agent 重写 IDENTITY.md（team/members 清空后序列化天然删键），
//     旧 TEAM.md 改名 .bak 留档。
//
// 返回迁移发生的团队数（用于日志）。
func MigrateLegacyTeams(agents *agentstore.AgentStore, teams *teamstore.Store, logger MigrationLogger) int {
	if agents == nil || teams == nil {
		return 0
	}

	migrated := 0
	for _, agent := range agents.List() {
		hasMembers := len(agent.Meta.Members) > 0
		hasTeam := strings.TrimSpace(agent.Meta.Team) != ""
		if !hasMembers && !hasTeam {
			continue
		}

		rewriteIdentity := false

		if hasMembers {
			teamName := strings.TrimSpace(agent.Meta.Team)
			if teamName == "" {
				// 无团队名声明：以负责人名兜底命名，保证引用可追溯
				teamName = agent.Name()
			}

			profile := ""
			if agent.TeamDuty != "" {
				profile = agent.TeamDuty
				// 旧 TEAM.md 改名留档（仿 MigrateLegacyFiles 备份语义）
				if err := backupAgentTeamFile(agent.Dir); err != nil {
					logger.Warn("团队迁移: TEAM.md 留档失败", "agent", agent.Name(), "error", err)
				}
			}

			if existing := teams.Get(teamName); existing != nil {
				// 多负责人声明同一团队：首个建队者保持 Leader，后续成员并入
				logger.Warn("团队迁移: 多负责人声明同一团队，成员并入首个团队",
					"team", teamName, "first_leader", existing.Meta.Leader, "agent", agent.Name())
				existing.Meta.Members = mergeUnique(existing.Meta.Members, agent.Meta.Members)
				existing.Meta.Members = mergeUnique(existing.Meta.Members, []string{agent.Name()})
				if err := teams.Save(existing); err != nil {
					logger.Warn("团队迁移: 合并成员失败", "team", teamName, "error", err)
					continue
				}
			} else {
				team := &teamstore.Team{
					Meta: teamstore.TeamMeta{
						Name:        teamName,
						Description: agent.Meta.Description,
						Leader:      agent.Name(),
						Members:     mergeUnique([]string{agent.Name()}, agent.Meta.Members),
						Icon:        agent.Meta.Icon,
						Category:    agent.Meta.Category,
						Hired:       agent.Meta.Hired,
					},
					Profile: profile,
				}
				if err := teams.Save(team); err != nil {
					logger.Warn("团队迁移: 建队失败", "team", teamName, "error", err)
					continue
				}
				logger.Info("团队迁移: 由负责人 frontmatter 建队", "team", teamName, "leader", agent.Name())
			}
			rewriteIdentity = true
			migrated++
		} else if hasTeam {
			// 成员的 team 声明：指向已建团队则保真并入成员集
			teamName := strings.TrimSpace(agent.Meta.Team)
			if existing := teams.Get(teamName); existing != nil {
				existing.Meta.Members = mergeUnique(existing.Meta.Members, []string{agent.Name()})
				if err := teams.Save(existing); err != nil {
					logger.Warn("团队迁移: 成员并入失败", "team", teamName, "agent", agent.Name(), "error", err)
					continue
				}
				logger.Info("团队迁移: 成员并入团队", "team", teamName, "agent", agent.Name())
			} else {
				logger.Warn("团队迁移: 成员 team 指向不存在的团队，仅清除键",
					"team", teamName, "agent", agent.Name())
			}
			rewriteIdentity = true
		}

		if rewriteIdentity {
			// 清空已退役键后全量重写（renderIdentity 序列化天然删键）
			agent.Meta.Team = ""
			agent.Meta.Members = nil
			if err := agents.Save(agent); err != nil {
				logger.Warn("团队迁移: IDENTITY.md 重写失败", "agent", agent.Name(), "error", err)
			}
		}
	}
	return migrated
}

// backupAgentTeamFile 将 Agent 目录内旧 TEAM.md 改名 .bak 留档。
// 文件不存在视为无需备份（幂等）。
func backupAgentTeamFile(agentDir string) error {
	if agentDir == "" {
		return nil
	}
	src := filepath.Join(agentDir, "TEAM.md")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	return os.Rename(src, src+".bak")
}

// mergeUnique 追加元素并去重（保持原有顺序，新元素按追加顺序排在后面）。
func mergeUnique(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	combined := make([]string, 0, len(base)+len(extra))
	combined = append(combined, base...)
	combined = append(combined, extra...)
	for _, s := range combined {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
