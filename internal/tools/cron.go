package tools

import (
	"context"
	"fmt"
	"strings"
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
		Description: "管理定时任务与计划工作：定时/周期执行、延后自我唤醒，或安排无直属关系的工作并回看结果。",
		Prompt: `管理定时任务与计划工作。创建、列出、更新和删除按计划运行的定时任务。

三种用法：
- **定时执行**：让指定名称的 Agent 在特定时间自动执行任务（如每天定时生成报告、定期巡检）。周期性用 cron_expr，一次性用 scheduled_at。
- **自我唤醒**：当需要等待较长时间或外部条件后才能继续时，创建定时任务在稍后时间唤醒自己继续未完成的工作，避免长期阻塞式等待。
- **无直属关系的工作安排**：对方与你没有可指派的直属关系（无法派发子任务）、但你仍需要得知其结果时，借道定时任务在稍后以 MasterAgent 身份安排该 Agent 执行，结果回到该任务会话供你检查。

不适用：当下就能派发并收取结果的任务——直接派发并在本会话收取，不要绕道定时。

操作：
- **list**：列出所有定时任务及其状态和下次运行信息。
- **create**：创建新的定时任务。需要 agent、content（要发送的提示词），以及执行计划二者之一：cron_expr（6 字段 cron 表达式，支持秒级，如 "0 0 9 * * *" 表示每天 9 点）或 scheduled_at（一次性任务时间点，到点执行一次后自动禁用）。title 是任务名，也是绑定会话的标题，建议填写（省略时取 content 首行）。
- 会话节奏：重复任务（cron_expr）绑定一条专属会话，每期在同一会话续跑，历史连续；一次性任务（scheduled_at）每次执行新开一条会话。
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
				Name:        "title",
				Type:        "string",
				Description: "任务名，同时作为绑定会话的标题（侧栏任务与会话因此一一对应）。建议简短具体，如\"每日 AI 新闻\"。省略时取 content 首行。update 时传入即改名并同步会话标题。",
				Required:    false,
			},
			{
				Name:        "cron_expr",
				Type:        "string",
				Description: "6 字段 cron 表达式，支持秒级（如 \"0 0 9 * * *\" 表示每天 9 点，\"0 */30 * * * *\" 表示每 30 分钟）。与 scheduled_at 二选一。",
				Required:    false,
			},
			{
				Name:        "scheduled_at",
				Type:        "string",
				Description: "一次性任务的时间点，格式 YYYY-MM-DDTHH:mm:ss（本地时区）或 RFC3339。提供时任务只执行一次，到点后自动禁用，无需手动删除。与 cron_expr 二选一。",
				Required:    false,
			},
			{
				Name:        "expires_at",
				Type:        "string",
				Description: "有效期截止时间（RFC3339，如 2026-12-31T23:59:59+08:00）。省略或空表示长期有效；到期后任务不再触发，排期与留痕保留。",
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
				Description: "任务执行的工作目录（绝对路径）。省略时取当前会话的工作目录；当前会话也没有工作目录时创建会失败并要求显式提供。",
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

	cronExpr := ""
	if raw, ok := getParam(params, "cron_expr"); ok {
		if s, ok := raw.(string); ok {
			cronExpr = s
		}
	}
	scheduledAt := ""
	if raw, ok := getParam(params, "scheduled_at"); ok {
		if s, ok := raw.(string); ok {
			scheduledAt = s
		}
	}
	if cronExpr == "" && scheduledAt == "" {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			"创建定时任务需要指定执行计划",
			"cron_expr 与 scheduled_at 至少提供一个：循环任务用 cron_expr，一次性任务用 scheduled_at",
			"先自查：确认任务是一次性还是循环，补齐对应参数后重新调用",
		))
	}

	oneShot := false
	if scheduledAt != "" {
		expr, oneShotErr := scheduler.OneShotCron(scheduledAt, time.Now())
		if oneShotErr != nil {
			return nil, fmt.Errorf("%s", tools.BuildGuide(
				"创建一次性定时任务失败",
				tools.WithErrDetail(fmt.Sprintf("scheduled_at %q 无法生成一次性计划", scheduledAt), oneShotErr),
				"先自查：scheduled_at 需为未来的时间，格式 YYYY-MM-DDTHH:mm:ss（如 2026-10-13T14:30:00）",
			))
		}
		cronExpr = expr
		oneShot = true
	}
	if validateErr := scheduler.ValidateCronExpr(cronExpr); validateErr != nil {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("创建定时任务被拒绝，cron_expr %q 非法", cronExpr),
			tools.WithErrDetail("调度引擎为 6 字段秒级格式（秒 分 时 日 月 周）", validateErr),
			"先自查：cron_expr 是否写成了 5 字段的常见格式？按 6 字段补齐后重新调用，如 0 0 9 * * * 表示每天 9 点",
		))
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
	// 工作目录缺省时取当前会话的项目目录：调度到点会新建专属会话，会话必须归属
	// 一个工作目录（否则到点才炸「project_dir 不能为空」）；创建阶段就拿不到目录
	// 就当场拒绝并给出人话指引，禁止静默落库留给执行期报错。
	if projectDir == "" {
		if tc := tools.GetToolContext(ctx); tc != nil && tc.Session != nil {
			projectDir = tc.Session.ProjectDir()
		}
	}
	if projectDir == "" {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			"创建定时任务被拒绝：没有可用的工作目录",
			"定时任务到点会新建一个专属会话去执行，该会话必须归属某个工作目录才能落库",
			"先自查：本次对话是否已选择工作目录？没有的话，显式传 project_dir（绝对路径，如 /Users/ray/workspaces/xxx）后重新调用",
		))
	}

	titleRaw, _ := getParam(params, "title")
	title, _ := titleRaw.(string)
	title = strings.TrimSpace(title)
	if title == "" {
		title = scheduler.DeriveTitle(content)
	}

	expiresAt, expErr := parseExpiresAt(params)
	if expErr != nil {
		return nil, expErr
	}

	entry := &scheduler.ScheduleEntry{
		ID:         id,
		Agent:      agent,
		ExpiresAt:  expiresAt,
		Title:      title,
		Content:    content,
		CronExpr:   cronExpr,
		Enabled:    enabled,
		OneShot:    oneShot,
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
	if v, ok := getParam(params, "title"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			existing.Title = strings.TrimSpace(s)
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
	if v, ok := getParam(params, "expires_at"); ok {
		if s, ok := v.(string); ok {
			at, perr := parseExpiresAtString(s)
			if perr != nil {
				return nil, perr
			}
			existing.ExpiresAt = at
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
		if s, ok := v.(string); ok && s != "" {
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

// parseExpiresAt 读取 expires_at 参数：空值 = 长期有效（nil）。
func parseExpiresAt(params map[string]any) (*time.Time, error) {
	raw, _ := getParam(params, "expires_at")
	str, _ := raw.(string)
	return parseExpiresAtString(str)
}

func parseExpiresAtString(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			"创建定时任务被拒绝：expires_at 格式不正确",
			fmt.Sprintf("expires_at %q 无法按 RFC3339 解析", raw),
			"先自查：有效期写成 2026-12-31T23:59:59+08:00 这类带时区的完整时间，或省略表示长期有效",
		))
	}
	if !at.After(time.Now()) {
		return nil, fmt.Errorf("有效期必须晚于当前时间（expires_at=%s）", raw)
	}
	return &at, nil
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
