package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// SchedulerDeps holds external dependencies for scheduler commands.
type SchedulerDeps struct {
	SchedulerDB func() *scheduler.FileSchedulerStore
	Scheduler   func() *scheduler.Scheduler
}

var schedulerDeps SchedulerDeps

// SetSchedulerDeps sets the dependencies for scheduler commands.
func SetSchedulerDeps(deps SchedulerDeps) {
	schedulerDeps = deps
}

func registerSchedulerCommands(r *Registry) {
	r.Register(Meta{
		Name:        "job.add",
		Description: i18n.T("cmd.scheduler.job.add.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Example:     i18n.T("cmd.scheduler.example"),
		Params:      i18n.T("cmd.scheduler.usage"),
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleJobAdd(ctx)
	})

	r.Register(Meta{
		Name:        "job.list",
		Description: i18n.T("cmd.scheduler.job.list.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleJobList(ctx)
	})

	r.Register(Meta{
		Name:        "job.del",
		Description: i18n.T("cmd.scheduler.job.del.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Params:      i18n.T("cmd.scheduler.job.del.param"),
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleJobDel(ctx)
	})
}

func handleJobAdd(ctx *gateway.CommandContext) (any, error) {
	if schedulerDeps.SchedulerDB == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	argsStr := strings.TrimSpace(ctx.Args)
	if argsStr == "" {
		return nil, errors.New(i18n.T("cmd.scheduler.usage") + "\n" + i18n.T("cmd.scheduler.example"))
	}

	agent, sessionID, content, cronExpr, projectDir, err := parseJobAddArgs(argsStr)
	if err != nil {
		return nil, err
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronExpr); err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.scheduler.invalid.cron"), err)
	}

	entry := &scheduler.ScheduleEntry{
		ID:         generateID(),
		Agent:      agent,
		SessionID:  sessionID,
		ProjectDir: projectDir,
		Content:    content,
		CronExpr:   cronExpr,
		Enabled:    true,
	}

	if err := schedulerDeps.SchedulerDB().Save(context.Background(), entry); err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.scheduler.save.failed"), err)
	}

	sessInfo := sessionID
	if sessInfo == "" || sessInfo == "new" {
		sessInfo = "(auto)"
	}
	dirInfo := projectDir
	if dirInfo == "" {
		dirInfo = "(daemon default)"
	}
	return fmt.Sprintf(i18n.T("cmd.scheduler.job.created")+"\n  ID: %s\n  "+i18n.T("cmd.scheduler.job.target")+": @%s\n  Session: %s\n  "+i18n.T("cmd.scheduler.job.projectdir")+": %s\n  "+i18n.T("cmd.scheduler.job.content")+": %s\n  "+i18n.T("cmd.scheduler.job.schedule")+": %s",
		entry.ID, entry.Agent, sessInfo, dirInfo, truncateString(entry.Content, 50), entry.CronExpr), nil
}

func handleJobList(ctx *gateway.CommandContext) (any, error) {
	if schedulerDeps.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	entries, err := schedulerDeps.Scheduler().List()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.scheduler.list.fetch.failed"), err)
	}

	if len(entries) == 0 {
		return i18n.T("cmd.scheduler.list.empty"), nil
	}

	headers := []string{i18n.T("cmd.scheduler.table.id"), i18n.T("cmd.scheduler.table.agent"), i18n.T("cmd.scheduler.table.session"), i18n.T("cmd.scheduler.table.projectdir"), i18n.T("cmd.scheduler.table.content"), i18n.T("cmd.scheduler.table.schedule"), i18n.T("cmd.scheduler.table.status"), i18n.T("cmd.scheduler.table.stats")}
	rows := make([][]string, 0, len(entries))

	for _, entry := range entries {
		status := i18n.T("cmd.scheduler.status.disabled")
		if entry.Enabled {
			status = i18n.T("cmd.scheduler.status.enabled")
		}
		sessDisplay := entry.SessionID
		if sessDisplay == "" || sessDisplay == "new" {
			sessDisplay = "(auto)"
		}
		dirDisplay := entry.ProjectDir
		if dirDisplay == "" {
			dirDisplay = "(default)"
		}
		rows = append(rows, []string{
			entry.ID,
			"@" + entry.Agent,
			sessDisplay,
			dirDisplay,
			truncateString(entry.Content, 30),
			entry.CronExpr,
			status,
			fmt.Sprintf("%d/%d", entry.SuccessCnt, entry.FailureCnt),
		})
	}

	ctx.RespondWithType(gateway.RespTable, i18n.T("cmd.scheduler.list.title"), map[string]interface{}{
		"headers": headers,
		"rows":    rows,
	})
	return nil, nil
}

func handleJobDel(ctx *gateway.CommandContext) (any, error) {
	if schedulerDeps.SchedulerDB == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	args := parseCommandArgs(ctx.Args)
	id := args["id"]
	if id == "" {
		return nil, errors.New(i18n.T("cmd.scheduler.missing.id") + "\n" + i18n.T("cmd.scheduler.del.usage"))
	}

	entry, err := schedulerDeps.SchedulerDB().Load(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.scheduler.job.notfound"), id)
	}

	if err := schedulerDeps.SchedulerDB().Delete(context.Background(), id); err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.scheduler.delete.failed"), err)
	}

	return fmt.Sprintf(i18n.T("cmd.scheduler.job.deleted")+"\n  ID: %s\n  "+i18n.T("cmd.scheduler.job.target")+": @%s\n  "+i18n.T("cmd.scheduler.job.content")+": %s", id, entry.Agent, truncateString(entry.Content, 50)), nil
}

func generateID() string {
	return uuid.New().String()[:8]
}

func parseCommandArgs(argsStr string) map[string]string {
	result := make(map[string]string)
	if argsStr == "" {
		return result
	}

	parts := splitArgs(argsStr)
	for _, part := range parts {
		idx := strings.Index(part, "=")
		if idx > 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}
	return result
}

func splitArgs(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\'' {
			if !inQuote {
				inQuote = true
				quoteChar = c
			} else if c == quoteChar {
				inQuote = false
				quoteChar = 0
			}
			current.WriteByte(c)
		} else if c == ' ' && !inQuote {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func parseJobAddArgs(argsStr string) (agent string, sessionID string, content string, cronExpr string, projectDir string, err error) {
	parts := splitArgs(argsStr)
	var agentIdx, exprIdx, dirIdx = -1, -1, -1
	var exprValue, dirValue string

	for i, part := range parts {
		if strings.HasPrefix(part, "@") && agentIdx == -1 {
			agentIdx = i
			agent = strings.TrimPrefix(part, "@")
		} else if strings.HasPrefix(part, "expr=") {
			exprIdx = i
			exprValue = strings.TrimPrefix(part, "expr=")
			exprValue = strings.Trim(exprValue, "\"'")
		} else if (strings.HasPrefix(part, "dir=") || strings.HasPrefix(part, "project=")) && dirIdx == -1 {
			dirIdx = i
			if strings.HasPrefix(part, "dir=") {
				dirValue = strings.TrimPrefix(part, "dir=")
			} else {
				dirValue = strings.TrimPrefix(part, "project=")
			}
			dirValue = strings.Trim(dirValue, "\"'")
		}
	}

	if agent == "" {
		return "", "", "", "", "", errors.New(i18n.T("cmd.scheduler.missing.agent") + "\n" + i18n.T("cmd.scheduler.add.example"))
	}

	if exprValue == "" {
		return "", "", "", "", "", errors.New(i18n.T("cmd.scheduler.missing.cron") + "\n" + i18n.T("cmd.scheduler.add.example"))
	}

	var contentParts []string
	for i, part := range parts {
		if i == agentIdx || i == exprIdx || i == dirIdx {
			continue
		}
		contentParts = append(contentParts, part)
	}

	if len(contentParts) == 0 {
		return "", "", "", "", "", fmt.Errorf(i18n.T("cmd.scheduler.missing.content"), agent)
	}

	sessionID = contentParts[0]
	content = strings.Join(contentParts[1:], " ")
	if content == "" {
		content = sessionID
		sessionID = "new"
	}

	if sessionID == "" {
		sessionID = "new"
	}

	return agent, sessionID, content, exprValue, dirValue, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
