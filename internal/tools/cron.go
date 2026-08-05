package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/google/uuid"
)

// Cron manages scheduled tasks via the scheduler store.
// It provides create, read, update, delete, and list operations for cron entries.
type Cron struct {
	store *scheduler.FileSchedulerStore
}

// NewCron creates a Cron tool backed by the given scheduler store.
func NewCron(store *scheduler.FileSchedulerStore) tools.FuncTool {
	return &Cron{store: store}
}

func (t *Cron) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Cron",
		Description: "管理定时任务。创建、列出、更新和删除按计划运行的定时任务。",
		Prompt: `管理定时任务。创建、列出、更新和删除按计划运行的定时任务。

典型用途：
- **定时委派**：让指定名称的 Agent 在特定时间自动执行任务（如每天定时生成报告、定期巡检）。
- **自我唤醒**：当需要等待较长时间或外部条件后才能继续时，可创建定时任务在稍后时间唤醒自己继续未完成的工作，避免长期阻塞式等待。

操作：
- **list**：列出所有定时任务及其状态和下次运行信息。
- **create**：创建新的定时任务。需要 agent、content（要发送的提示词）和 cron_expr（6 字段 cron 表达式，支持秒级，如 "0 0 9 * * *" 表示每天 9 点）。
- **update**：按 id 更新现有定时任务。只更新提供的字段。
- **delete**：按 id 删除定时任务。`,
		IsReadOnly: false,
		Parameters: []tools.Parameter{
			{
				Name:        "action",
				Type:        "string",
				Description: "操作类型：\"list\"、\"create\"、\"update\" 或 \"delete\"。",
				Required:    true,
				Enum:        []any{"list", "create", "update", "delete"},
			},
			{
				Name:        "id",
				Type:        "string",
				Description: "定时任务 ID。update 和 delete 必需。create 可选（省略时自动生成）。",
				Required:    false,
			},
			{
				Name:        "agent",
				Type:        "string",
				Description: "执行任务的代理名称（如 \"myagent\"）。create 必需。",
				Required:    false,
			},
			{
				Name:        "content",
				Type:        "string",
				Description: "任务运行时发送给代理的提示词内容。create 必需。",
				Required:    false,
			},
			{
				Name:        "cron_expr",
				Type:        "string",
				Description: "6 字段 cron 表达式，支持秒级（如 \"0 0 9 * * *\" 表示每天 9 点，\"0 */30 * * * *\" 表示每 30 分钟）。create 必需。",
				Required:    false,
			},
			{
				Name:        "enabled",
				Type:        "boolean",
				Description: "是否启用任务（默认：新建任务为 true）。",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "session_id",
				Type:        "string",
				Description: "关联的会话 ID。如果为空或 \"new\"，每次运行时创建新会话。",
				Required:    false,
			},
			{
				Name:        "project_dir",
				Type:        "string",
				Description: "任务执行的工作目录。",
				Required:    false,
			},
		},
	}
}

func (t *Cron) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.store == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("Cron", "调度器存储"))
	}

	action, err := tools.ValidateRequiredString("Cron", params, "action")
	if err != nil {
		return nil, err
	}

	switch action {
	case "list":
		return t.listEntries(ctx)
	case "create":
		return t.createEntry(ctx, params)
	case "update":
		return t.updateEntry(ctx, params)
	case "delete":
		return t.deleteEntry(ctx, params)
	default:
		return nil, fmt.Errorf("%s", tools.GuideInvalidValue("Cron", "action", action, "先自查：action 只支持 list / create / update / delete 四者之一，对照工具参数定义确认拼写后重新调用"))
	}
}

func (t *Cron) listEntries(ctx context.Context) (any, error) {
	entries, err := t.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			"获取定时任务列表时失败",
			tools.WithErrDetail("无法从调度器存储读取定时任务列表", err),
			"先自查：调度器存储目录/文件是否存在且可读写？确认存储可用后重试；若仍失败应告知用户",
		), err)
	}
	if entries == nil {
		entries = []scheduler.ScheduleEntry{}
	}
	return entries, nil
}

func (t *Cron) createEntry(ctx context.Context, params map[string]any) (any, error) {
	agent, err := tools.ValidateRequiredString("Cron", params, "agent")
	if err != nil {
		return nil, err
	}
	content, err := tools.ValidateRequiredString("Cron", params, "content")
	if err != nil {
		return nil, err
	}
	cronExpr, err := tools.ValidateRequiredString("Cron", params, "cron_expr")
	if err != nil {
		return nil, err
	}

	idRaw, _ := getParam(params, "id")
	id, _ := idRaw.(string)
	if id == "" {
		id = uuid.NewString()[:8]
	}

	enabled := true
	if raw, ok := getParam(params, "enabled"); ok {
		if v, ok := raw.(bool); ok {
			enabled = v
		}
	}

	sessionIDRaw, _ := getParam(params, "session_id")
	sessionID, _ := sessionIDRaw.(string)
	projectDirRaw, _ := getParam(params, "project_dir")
	projectDir, _ := projectDirRaw.(string)

	entry := &scheduler.ScheduleEntry{
		ID:         id,
		Agent:      agent,
		Content:    content,
		CronExpr:   cronExpr,
		Enabled:    enabled,
		SessionID:  sessionID,
		ProjectDir: projectDir,
		CreatedAt:  time.Now(),
	}

	if err := t.store.Save(ctx, entry); err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("保存定时任务 %q 时失败", id),
			tools.WithErrDetail("无法将定时任务写入调度器存储", err),
			"先自查：调度器存储目录/文件是否存在且可读写？cron_expr 是否为合法的 6 字段 cron 表达式？确认后重试；若仍失败应告知用户",
		), err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("定时任务 %q 创建成功", id),
		"entry":   entry,
	}, nil
}

func (t *Cron) updateEntry(ctx context.Context, params map[string]any) (any, error) {
	id, err := tools.ValidateRequiredString("Cron", params, "id")
	if err != nil {
		return nil, err
	}

	existing, err := t.store.Load(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s", tools.GuideNotFound("定时任务", id, "检查任务 ID 是否正确（可使用 list 查看现有任务 ID），确认后重新调用"))
	}

	if v, ok := getParam(params, "agent"); ok {
		if s, ok := v.(string); ok && s != "" {
			existing.Agent = s
		}
	}
	if v, ok := getParam(params, "content"); ok {
		if s, ok := v.(string); ok && s != "" {
			existing.Content = s
		}
	}
	if v, ok := getParam(params, "cron_expr"); ok {
		if s, ok := v.(string); ok && s != "" {
			existing.CronExpr = s
		}
	}
	if v, ok := getParam(params, "enabled"); ok {
		if s, ok := v.(bool); ok {
			existing.Enabled = s
		}
	}
	if v, ok := getParam(params, "session_id"); ok {
		if s, ok := v.(string); ok {
			existing.SessionID = s
		}
	}
	if v, ok := getParam(params, "project_dir"); ok {
		if s, ok := v.(string); ok {
			existing.ProjectDir = s
		}
	}

	if err := t.store.Save(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("更新定时任务 %q 时失败", id),
			tools.WithErrDetail("无法将更新后的定时任务写入调度器存储", err),
			"先自查：调度器存储目录/文件是否存在且可读写？确认存储可用后重新提交更新；若仍失败应告知用户",
		), err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("定时任务 %q 更新成功", id),
		"entry":   existing,
	}, nil
}

func (t *Cron) deleteEntry(ctx context.Context, params map[string]any) (any, error) {
	id, err := tools.ValidateRequiredString("Cron", params, "id")
	if err != nil {
		return nil, err
	}

	if err := t.store.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("删除定时任务 %q 时失败", id),
			tools.WithErrDetail("无法从调度器存储删除定时任务", err),
			"先自查：任务 ID 是否正确（可使用 list 查看现有任务 ID）？调度器存储目录/文件是否可访问？确认后重试；若仍失败应告知用户",
		), err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("定时任务 %q 删除成功", id),
	}, nil
}
