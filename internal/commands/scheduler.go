package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
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
		Name:        "job-add",
		Description: "添加计划任务",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Example:     `/job-add @writer sess_abc123 每日博客文章 expr="0 0 9 * * 1" dir="/Users/ray/workspaces/my-project"`,
		Params:      `@<agent-name> <session_id|new> <content> expr="<cron表达式>" [dir="<项目目录>"]`,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleJobAdd(ctx)
	})

	r.Register(Meta{
		Name:        "job-list",
		Description: "列出所有计划任务",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleJobList(ctx)
	})

	r.Register(Meta{
		Name:        "job-del",
		Description: "删除计划任务",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Params:      `id=<任务ID>`,
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
		return nil, fmt.Errorf("用法: /job-add @<agent-name> <session_id|new> <content> expr=\"<cron表达式>\"\n示例: /job-add @writer sess_abc123 每日博客文章 expr=\"0 0 9 * * 1\"")
	}

	agent, sessionID, content, cronExpr, projectDir, err := parseJobAddArgs(argsStr)
	if err != nil {
		return nil, err
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronExpr); err != nil {
		return nil, fmt.Errorf("无效的 cron 表达式: %w", err)
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
		return nil, fmt.Errorf("保存任务失败: %w", err)
	}

	sessInfo := sessionID
	if sessInfo == "" || sessInfo == "new" {
		sessInfo = "(auto)"
	}
	dirInfo := projectDir
	if dirInfo == "" {
		dirInfo = "(daemon default)"
	}
	return fmt.Sprintf("✅ 定时消息已创建:\n  ID: %s\n  目标: @%s\n  Session: %s\n  项目目录: %s\n  内容: %s\n  调度: %s",
		entry.ID, entry.Agent, sessInfo, dirInfo, truncateString(entry.Content, 50), entry.CronExpr), nil
}

func handleJobList(ctx *gateway.CommandContext) (any, error) {
	if schedulerDeps.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	entries, err := schedulerDeps.Scheduler().List()
	if err != nil {
		return nil, fmt.Errorf("获取任务列表失败: %w", err)
	}

	if len(entries) == 0 {
		return "(暂无定时消息任务)", nil
	}

	headers := []string{"ID", "目标Agent", "Session", "项目目录", "发送内容", "调度规则", "状态", "成功/失败"}
	rows := make([][]string, 0, len(entries))

	for _, entry := range entries {
		status := "❌ 已禁用"
		if entry.Enabled {
			status = "✅ 启用"
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

	ctx.RespondWithType(gateway.RespTable, "定时消息任务列表", map[string]interface{}{
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
		return nil, fmt.Errorf("缺少必要参数: id (任务ID)\n用法: /job-del id=<任务ID>")
	}

	entry, err := schedulerDeps.SchedulerDB().Load(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}

	if err := schedulerDeps.SchedulerDB().Delete(context.Background(), id); err != nil {
		return nil, fmt.Errorf("删除任务失败: %w", err)
	}

	return fmt.Sprintf("🗑️ 定时消息已删除:\n  ID: %s\n  目标: @%s\n  内容: %s", id, entry.Agent, truncateString(entry.Content, 50)), nil
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
	var agentIdx, exprIdx, dirIdx int = -1, -1, -1
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
		return "", "", "", "", "", fmt.Errorf("缺少目标智能体: 请使用 @<agent-name> 格式指定\n示例: /job-add @writer sess_abc123 每日博客文章 expr=\"0 0 9 * * 1\" dir=\"/path/to/project\"")
	}

	if exprValue == "" {
		return "", "", "", "", "", fmt.Errorf("缺少 cron 表达式: 请使用 expr=\"<cron表达式>\" 指定\n示例: /job-add @writer sess_abc123 每日提醒 expr=\"0 0 9 * * 1\"")
	}

	var contentParts []string
	for i, part := range parts {
		if i == agentIdx || i == exprIdx || i == dirIdx {
			continue
		}
		contentParts = append(contentParts, part)
	}

	if len(contentParts) == 0 {
		return "", "", "", "", "", fmt.Errorf("缺少发送内容: 请指定要定时发送给 @%s 的消息内容", agent)
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
