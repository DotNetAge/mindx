package conv

// GroupKey 工具聚合类别（对齐 Desktop GroupKey）。
// 每种工具节点类型静态声明；GroupBuffer 对「相邻同 GroupKey 工具段」聚合为 GroupNode。
// nil 表示该类型不聚合（如 Skill 关键动作）。
type GroupKey string

const (
	GroupFSRead    GroupKey = "fs.read"
	GroupFSWrite   GroupKey = "fs.write"
	GroupFSBrowse  GroupKey = "fs.browse"
	GroupFSSearch  GroupKey = "fs.search"
	GroupCmd       GroupKey = "cmd"
	GroupWebFetch  GroupKey = "web.fetch"
	GroupWebSearch GroupKey = "web.search"
	GroupKBSearch  GroupKey = "kb.search"
	GroupTaskQuery GroupKey = "task.query"
	GroupTeamQuery GroupKey = "team.query"
	GroupSys       GroupKey = "sys"
)

// ToolDescriptor 工具登记项（对齐 Desktop ToolDescriptor）。
// 新增工具先补本表再写 builder 逻辑，杜绝实现期即兴对齐。
type ToolDescriptor struct {
	NodeType string    // 节点 Kind，如 "tool.bash"
	GroupKey *GroupKey // 聚合类别，nil = 不聚合
}

// ptr 辅助：取 GroupKey 地址（让 ToolDescriptors 的 GroupKey 字段能写 nil 字面量）。
func ptr(k GroupKey) *GroupKey { return &k }

// ToolDescriptors 27 个工具的完整对照表（与 Desktop tool-map.ts 一一对应）。
//
// TaskCreate / TaskUpdate / SubAgent / CollectResults / TeamCreate 不出现在本表：
// 它们是实体归一规则（见 TaskUpsertTools / SubagentTool / CollectTool / TeamCreateTool），
// 由 builder 主流程特判落实体卡而非工具节点。
var ToolDescriptors = map[string]ToolDescriptor{
	// 文件读取
	"Read":    {NodeType: "tool.read", GroupKey: ptr(GroupFSRead)},
	"ReadPro": {NodeType: "tool.read", GroupKey: ptr(GroupFSRead)},
	// 文件写入/编辑
	"Write": {NodeType: "tool.write", GroupKey: ptr(GroupFSWrite)},
	"Edit":  {NodeType: "tool.edit", GroupKey: ptr(GroupFSWrite)},
	// 目录浏览/文件匹配
	"Ls":    {NodeType: "tool.ls", GroupKey: ptr(GroupFSBrowse)},
	"LsPro": {NodeType: "tool.ls", GroupKey: ptr(GroupFSBrowse)},
	"Glob":  {NodeType: "tool.glob", GroupKey: ptr(GroupFSBrowse)},
	// 内容搜索
	"Grep": {NodeType: "tool.grep", GroupKey: ptr(GroupFSSearch)},
	// 命令执行
	"Bash":      {NodeType: "tool.bash", GroupKey: ptr(GroupCmd)},
	"RunScript": {NodeType: "tool.run_script", GroupKey: ptr(GroupCmd)},
	// 网页
	"WebFetch":  {NodeType: "tool.web_fetch", GroupKey: ptr(GroupWebFetch)},
	"WebSearch": {NodeType: "tool.web_search", GroupKey: ptr(GroupWebSearch)},
	// 知识库/记忆检索
	"QuickSearch":  {NodeType: "tool.kb_search", GroupKey: ptr(GroupKBSearch)},
	"MemorySearch": {NodeType: "tool.memory_search", GroupKey: ptr(GroupKBSearch)},
	// 关键动作（不聚合）
	"Skill": {NodeType: "tool.skill", GroupKey: nil},
	// 系统操作
	"Sleep": {NodeType: "tool.sleep", GroupKey: ptr(GroupSys)},
	// 任务查询
	"TaskList": {NodeType: "tool.task_query", GroupKey: ptr(GroupTaskQuery)},
	"TaskGet":  {NodeType: "tool.task_query", GroupKey: ptr(GroupTaskQuery)},
	// 团队查询
	"TeamDelete":   {NodeType: "tool.team_ops", GroupKey: ptr(GroupTeamQuery)},
	"TeamList":     {NodeType: "tool.team_ops", GroupKey: ptr(GroupTeamQuery)},
	"TeamGetTasks": {NodeType: "tool.team_ops", GroupKey: ptr(GroupTeamQuery)},
	// 定时/通知
	"Cron":        {NodeType: "tool.cron", GroupKey: ptr(GroupSys)},
	"SendMessage": {NodeType: "tool.notify", GroupKey: ptr(GroupSys)},
}

// TaskUpsertTools TaskCreate/Update 按 taskId 实体幂等 upsert（归一规则 3），不落工具节点。
var TaskUpsertTools = map[string]bool{
	"TaskCreate": true,
	"TaskUpdate": true,
}

// 实体协作工具名常量（归一规则 3/4/11）。
const (
	SubagentTool   = "SubAgent"
	CollectTool    = "CollectResults"
	TeamCreateTool = "TeamCreate"
)

// IsEntityTool 判断工具是否属于实体归一路径（不进 ToolDescriptors）。
func IsEntityTool(toolName string) bool {
	return TaskUpsertTools[toolName] ||
		toolName == SubagentTool ||
		toolName == CollectTool ||
		toolName == TeamCreateTool
}
