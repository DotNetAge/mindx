package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/tools"
)

// ---------------------------------------------------------------------------
// UI 呈现三工具 —— Agent-Driven UI 命令通道的工具化入口（对齐 mindx ui CLI）。
//
// 数据流：LLM 调用工具 → 本组工具过参数闸 → 经注入的广播回调交 daemon 广播
// JSON 事件（file_open / link_open / terminal_run）→ 各客户端订阅执行。
//
// 与 CLI 版共享同一套事件与客户端行为（见 mindx-work notes 主文档）；差异仅
// 在闸的位置：CLI 版闸在 CLI 进程 cwd，工具版闸在会话 ProjectDir（ToolContext
// 提供）。受理语义一致：Execute 返回即「已受理并广播」，不等客户端回执。
// ---------------------------------------------------------------------------

// UIBroadcast 是 daemon 广播回调的形状（svc.Daemon.broadcastUI 适配）。
type UIBroadcast func(event string, data map[string]any)

// Open 请求客户端按文件类型分发打开文件（对应事件 file_open）。
type Open struct {
	Broadcast UIBroadcast
}

// NewOpen 创建 Open 工具（broadcast 为空时执行报错，防静默失效）。
func NewOpen(broadcast UIBroadcast) tools.FuncTool {
	return &Open{Broadcast: broadcast}
}

func (t *Open) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Open",
		Description: "在用户客户端打开文件。按文件类型分发到对应视图（Markdown 文档、仪表板、画板、网页等）。",
		Prompt: `请求用户客户端打开指定文件，文件会按类型自动分发到对应视图。
受理即返回：工具返回「已受理」表示请求已提交广播，实际呈现发生在用户屏幕
上。仅在需要向用户呈现内容时使用（如生成了报告、仪表板、图表文件），不要
在无人查看的场景滥用。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "path",
				Type:        "string",
				Description: "要打开的文件路径（相对项目目录或绝对路径，必须位于项目目录内）。",
				Required:    true,
			},
		},
	}
}

func (t *Open) Execute(ctx context.Context, params map[string]any) (any, error) {
	raw, err := tools.ValidateRequiredString("Open", params, "path")
	if err != nil {
		return nil, err
	}
	abs, err := resolveSessionWorkspacePath(ctx, raw)
	if err != nil {
		return nil, err
	}
	if t.Broadcast == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("Open", "可用的 UI 广播通道（daemon 未装配）"))
	}
	t.Broadcast("file_open", map[string]any{"path": abs})
	return map[string]any{"accepted": true, "path": abs, "note": "已受理，文件将呈现在用户客户端"}, nil
}

// Visit 请求客户端经系统浏览器打开网页（对应事件 link_open）。
type Visit struct {
	Broadcast UIBroadcast
}

// NewVisit 创建 Visit 工具（broadcast 为空时执行报错，防静默失效）。
func NewVisit(broadcast UIBroadcast) tools.FuncTool {
	return &Visit{Broadcast: broadcast}
}

func (t *Visit) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Visit",
		Description: "在用户系统浏览器打开网页链接。用于需要用户交互的页面（如授权登录、外部文档）。",
		Prompt: `在用户界面内的浏览器打开网页链接。用于需要用户交互的页面（如授权登录、外部文档）
受理即返回：工具返回「已受理」表示请求已提交广播，实际打开发生在用户屏幕
上。仅传用户需要交互或查看的链接。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "url",
				Type:        "string",
				Description: "要打开的 http/https 链接。典型场景：需要用户登录获取授权令牌时，用本工具打开授权页面，用户在系统浏览器完成登录后返回结果。",
				Required:    true,
			},
		},
	}
}

func (t *Visit) Execute(ctx context.Context, params map[string]any) (any, error) {
	url, err := tools.ValidateRequiredString("Visit", params, "url")
	if err != nil {
		return nil, err
	}
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("url=%q 仅支持 http/https 链接", url),
			"链接必须以 http:// 或 https:// 开头",
			"改为传入完整的 http/https 链接；如需呈现本地文件请改用 Open 工具",
		))
	}
	if t.Broadcast == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("Visit", "可用的 UI 广播通道（daemon 未装配）"))
	}
	t.Broadcast("link_open", map[string]any{"url": url})
	return map[string]any{"accepted": true, "url": url, "note": "已受理，链接将在用户系统浏览器打开"}, nil
}

// TerminalRun 请求客户端在可见终端执行命令（对应事件 terminal_run）。
type TerminalRun struct {
	Broadcast UIBroadcast
}

// NewTerminalRun 创建 TerminalRun 工具（broadcast 为空时执行报错，防静默失效）。
func NewTerminalRun(broadcast UIBroadcast) tools.FuncTool {
	return &TerminalRun{Broadcast: broadcast}
}

func (t *TerminalRun) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "TerminalRun",
		Description: "交互式指令：在用户客户端的可见终端执行命令，用户可实时看到输出并与进程交互。",
		Prompt: `请求在用户客户端的可见终端执行命令（交互式指令）。命令在用户眼皮底下运行，
适合需要用户看到进度、或需要交互式进程（如 dev server、watch 任务）的场景。
与静默后台执行的 Bash 不同，本工具的执行对用户完全可见。
命令在客户端终端以会话项目目录为工作目录执行；cwd 参数可覆盖。

受理即返回：工具返回「已受理」表示命令已提交到终端，不代表执行完成。`,
		IsReadOnly: false,
		Parameters: []tools.Parameter{
			{
				Name:        "command",
				Type:        "string",
				Description: "要在客户端终端执行的命令。",
				Required:    true,
			},
			{
				Name:        "cwd",
				Type:        "string",
				Description: "可选工作目录（相对项目目录或绝对路径，默认为会话项目目录）。",
				Required:    false,
			},
		},
	}
}

func (t *TerminalRun) Execute(ctx context.Context, params map[string]any) (any, error) {
	command, err := tools.ValidateRequiredString("TerminalRun", params, "command")
	if err != nil {
		return nil, err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("%s", tools.GuideMissingParam("TerminalRun", "command"))
	}

	projectDir := toolProjectDir(ctx)
	cwd := projectDir
	if rawCwd, _ := params["cwd"].(string); strings.TrimSpace(rawCwd) != "" {
		cwd, err = resolveInProjectDir(projectDir, strings.TrimSpace(rawCwd))
		if err != nil {
			return nil, err
		}
	}

	if t.Broadcast == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("TerminalRun", "可用的 UI 广播通道（daemon 未装配）"))
	}
	t.Broadcast("terminal_run", map[string]any{"command": command, "cwd": cwd})
	return map[string]any{"accepted": true, "command": command, "cwd": cwd, "note": "已受理，命令将提交到用户可见终端执行"}, nil
}

// Notify 请求客户端弹出界面通知（对应事件 user_notify）。
type Notify struct {
	Broadcast UIBroadcast
}

// NewNotify 创建 Notify 工具（broadcast 为空时执行报错，防静默失效）。
func NewNotify(broadcast UIBroadcast) tools.FuncTool {
	return &Notify{Broadcast: broadcast}
}

func (t *Notify) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Notify",
		Description: "向用户客户端发送界面通知（右上角通知卡片，非阻塞，不打断当前工作）。",
		Prompt: `在用户界面右上角弹出一张通知卡片（标题 + 内容），用户自行决定何时处理，
不阻塞当前会话。level 表达紧急程度：high 需用户立即决策（红色卡片）、
medium 尽快关注（黄色）、low 仅知会（绿色）、normal 紧急程度不明（普通）。
仅在确有必要让用户知道的时候使用，不要滥用。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "title",
				Type:        "string",
				Description: "通知标题：几个字的概括（如「来信待决」「任务办结」）。",
				Required:    true,
			},
			{
				Name:        "message",
				Type:        "string",
				Description: "通知内容：一句完整的话，说清发生了什么、需要用户做什么。",
				Required:    true,
			},
			{
				Name:        "level",
				Type:        "string",
				Description: "紧急程度：high（红，立即处理）/ medium（黄，尽快处理）/ low（绿，仅知会）/ normal（普通，默认）。",
				Required:    false,
				Enum:        []any{"high", "medium", "low", "normal"},
			},
		},
	}
}

func (t *Notify) Execute(ctx context.Context, params map[string]any) (any, error) {
	title, err := tools.ValidateRequiredString("Notify", params, "title")
	if err != nil {
		return nil, err
	}
	message, err := tools.ValidateRequiredString("Notify", params, "message")
	if err != nil {
		return nil, err
	}
	level := ""
	if raw, _ := params["level"].(string); raw != "" {
		level = strings.TrimSpace(raw)
	}
	switch level {
	case "high", "medium", "low":
	case "", "normal":
		level = "normal"
	default:
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("level=%q 不是合法的紧急程度", level),
			"level 仅支持 high / medium / low / normal",
			"按事项实际紧急程度选择一级，或留空使用普通",
		))
	}
	if t.Broadcast == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("Notify", "可用的 UI 广播通道（daemon 未装配）"))
	}
	t.Broadcast("user_notify", map[string]any{"title": title, "message": message, "level": level})
	return map[string]any{"accepted": true, "level": level, "note": "已受理，通知将呈现在用户客户端"}, nil
}

// resolveSessionWorkspacePath 参数闸：path 解析为绝对路径后必须落在会话项目
// 目录内（工具版工作区闸；CLI 版闸在 CLI 进程 cwd，见 cmd/ui.go）。
func resolveSessionWorkspacePath(ctx context.Context, raw string) (string, error) {
	return resolveInProjectDir(toolProjectDir(ctx), raw)
}

// toolAgentName 从 ToolContext 取当前会话所属智能体名（缺失返回空串）。
func toolAgentName(ctx context.Context) string {
	tc := tools.GetToolContext(ctx)
	if tc == nil || tc.Session == nil {
		return ""
	}
	return tc.Session.AgentName()
}

// toolProjectDir 从 ToolContext 取会话项目目录（缺失时返回空串，由调用方报引导错误）。
func toolProjectDir(ctx context.Context) string {
	tc := tools.GetToolContext(ctx)
	if tc == nil || tc.Session == nil {
		return ""
	}
	return tc.Session.ProjectDir()
}

// resolveInProjectDir 解析 raw（相对 projectDir）并校验边界；projectDir 为空
// 视为缺少会话上下文。
func resolveInProjectDir(projectDir, raw string) (string, error) {
	if projectDir == "" {
		return "", fmt.Errorf("%s", tools.GuideMissingContext("UI 呈现工具", "包含 ProjectDir 的 ToolContext"))
	}
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("%s", tools.GuideMissingParam("UI 呈现工具", "path"))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(projectDir, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(projectDir, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("路径 %q 解析为 %q，位于项目目录 %q 之外", raw, p, projectDir),
			"工具只允许访问当前项目目录内的路径",
			"只允许打开当前项目目录内的文件；如需呈现其它位置的文件，先把文件复制进项目目录",
		))
	}
	return p, nil
}

// Handoff 把会话交接给另一个智能体（对应事件 session_handoff）。
//
// 语义：发起方 Agent 代用户打开目标 Agent 的会话，并把任务原文以用户身份发出；
// 客户端收到事件即切到该会话，此后是「用户 ↔ 目标 Agent」直接对话，发起方退出。
// 与 Open/Visit 同族：受理即返回，不等客户端回执。
type Handoff struct {
	Broadcast   UIBroadcast
	AgentExists func(string) bool
}

// NewHandoff 创建 Handoff 工具（broadcast 为空时执行报错；AgentExists 为空时跳过存在性校验）。
func NewHandoff(broadcast UIBroadcast, agentExists func(string) bool) tools.FuncTool {
	return &Handoff{Broadcast: broadcast, AgentExists: agentExists}
}

func (t *Handoff) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Handoff",
		Description: "把任务交接给另一个智能体，由它直接与用户继续对话（界面切到对方会话）。",
		Prompt: `把手上的任务交接给一个更合适的智能体，由它直接与用户交流。

适用：这件事需要另一位专职智能体长期负责，或你的职责只是找对人、把话带到。

task 要写成用户自己会说出口的那句话：替他把缺的上下文补齐，不改成任务书格式。

工具返回即完成：你这边不用继续这条任务，不要重复执行，也不要向用户复述——用户接下来是在与对方对话。`,
		IsReadOnly: false,
		Parameters: []tools.Parameter{
			{
				Name:        "agent",
				Type:        "string",
				Description: "接手任务的目标智能体名（必须是本机已安装的智能体，且不是你自己）。",
				Required:    true,
			},
			{
				Name:        "task",
				Type:        "string",
				Description: "交给对方的任务原文。以用户口吻陈述要做什么，可直接使用用户刚才的表述，不要改写成任务书格式。",
				Required:    true,
			},
			{
				Name:        "project_dir",
				Type:        "string",
				Description: "对方会话的工作目录（相对当前项目目录或绝对路径，必须在项目目录内；缺省用当前项目目录）。",
				Required:    false,
			},
			{
				Name:        "mode",
				Type:        "string",
				Description: "会话形态：new（默认，开新会话）| resume（复用该智能体在同一目录下已有的会话）。",
				Required:    false,
				Enum:        []any{"new", "resume"},
			},
			{
				Name:        "deliver",
				Type:        "string",
				Description: "交付方式：draft（默认，只把任务填进对方输入框，用户自己按发送键；用户没让你替他把话说死时一律用这个）| send（直接以用户身份发出，仅在用户明确要求「直接交给它办」时用）。",
				Required:    false,
				Enum:        []any{"draft", "send"},
			},
		},
	}
}

func (t *Handoff) Execute(ctx context.Context, params map[string]any) (any, error) {
	agent, err := tools.ValidateRequiredString("Handoff", params, "agent")
	if err != nil {
		return nil, err
	}
	agent = strings.TrimSpace(agent)
	task, err := tools.ValidateRequiredString("Handoff", params, "task")
	if err != nil {
		return nil, err
	}
	task = strings.TrimSpace(task)

	// 交接给自己无意义：自己就是当前会话的对话方
	if current := toolAgentName(ctx); current != "" && current == agent {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("agent=%q 就是当前会话的智能体", agent),
			"交接的目标必须是与当前会话不同的智能体",
			"改传另一位已安装智能体的名字；若要把活留给用户，直接正常回复即可",
		))
	}
	if t.AgentExists != nil && !t.AgentExists(agent) {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("agent=%q 未安装", agent),
			"只能交接给本机已安装的智能体",
			"先用技能/市场安装该智能体，或改交接给已安装的智能体",
		))
	}

	projectDir := toolProjectDir(ctx)
	dir := projectDir
	if raw, _ := params["project_dir"].(string); strings.TrimSpace(raw) != "" {
		dir, err = resolveInProjectDir(projectDir, strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
	}

	mode := "new"
	if raw, _ := params["mode"].(string); strings.TrimSpace(raw) != "" {
		mode = strings.TrimSpace(raw)
	}
	if mode != "new" && mode != "resume" {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("mode=%q 非法", mode),
			"mode 只有两种取值",
			"传 new（开新会话）或省略；需要接着对方旧会话时传 resume",
		))
	}

	deliver := "draft"
	if raw, _ := params["deliver"].(string); strings.TrimSpace(raw) != "" {
		deliver = strings.TrimSpace(raw)
	}
	if deliver != "draft" && deliver != "send" {
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("deliver=%q 非法", deliver),
			"deliver 只有两种取值",
			"传 draft（填进输入框由用户按发送键，默认）或 send（直接发出）",
		))
	}

	if t.Broadcast == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("Handoff", "可用的 UI 广播通道（daemon 未装配）"))
	}
	t.Broadcast("session_handoff", map[string]any{
		"agent": agent, "task": task, "project_dir": dir, "mode": mode, "deliver": deliver,
	})
	note := fmt.Sprintf("已转交给 %s，任务已填进对方输入框并切到该会话，等用户按发送键后对方才开始；你这边不用继续这条任务", agent)
	if deliver == "send" {
		note = fmt.Sprintf("已转交给 %s，任务已由其接手，你这边不用继续这条任务", agent)
	}
	return map[string]any{
		"accepted": true, "agent": agent, "project_dir": dir, "mode": mode, "deliver": deliver,
		"note": note,
	}, nil
}
