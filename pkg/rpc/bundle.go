package rpc

import "encoding/json"

// AgentExportParams are the params for agent.export（导出 Agent 分发包到指定路径）。
type AgentExportParams struct {
	Name    string `json:"name"`
	OutPath string `json:"out_path"`
}

// AgentImportParams are the params for agent.import（本地安装 Agent 分发包）。
type AgentImportParams struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// SkillExportParams are the params for skill.export（导出 Skill 分发包到指定路径）。
type SkillExportParams struct {
	Name    string `json:"name"`
	OutPath string `json:"out_path"`
}

// SkillImportParams are the params for skill.import（本地安装 Skill 分发包）。
type SkillImportParams struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// MarketListParams are the params for market.list。
// 全部字段可选：空参 = 存量全量清单语义（过渡兼容）；带分页/过滤参数 = 走服务端分页 API。
type MarketListParams struct {
	Page     int    `json:"page,omitempty"`      // 1 起始
	PageSize int    `json:"page_size,omitempty"` // 缺省 50，服务端上限 200
	Kind     string `json:"kind,omitempty"`      // agent | skill | team
	Q        string `json:"q,omitempty"`         // 对 name/description 查找
	Sort     string `json:"sort,omitempty"`      // likes | newest | downloads
	// OrgSlug 组织源 slug（空 = 公共资源社区；非空 = 浏览该组织的专用源）。
	OrgSlug string `json:"org_slug,omitempty"`
}

// MarketInstallParams are the params for market.install（下载市场分发包并安装）。
type MarketInstallParams struct {
	Kind string `json:"kind"` // "agent" | "skill"
	Name string `json:"name"`
	// Overwrite 目标已存在时覆盖安装（前端二次确认后才携带）。
	Overwrite bool `json:"overwrite,omitempty"`
	// SkipDeps 跳过依赖检查和安装。
	SkipDeps bool `json:"skip_deps,omitempty"`
	// AutoInstall 自动执行缺失依赖的安装脚本。
	AutoInstall bool `json:"auto_install,omitempty"`
	// OrgSlug 组织源 slug（空 = 公共资源社区；非空 = 从该组织的专用源下载安装）。
	OrgSlug string `json:"org_slug,omitempty"`
}

// MarketPackageReadParams are the params for market.package.read（市场技能包 SKILL.md 预览，不安装）。
type MarketPackageReadParams struct {
	Name string `json:"name"`
}

// MarketRateParams are the params for market.rate（包级评分，1-5 星）。
type MarketRateParams struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Score int64  `json:"score"` // 1-5
	// OrgSlug 组织源 slug（空 = 公共资源社区包；非空 = 评该组织专属空间的包，
	// 企业内实名口径）。
	OrgSlug string `json:"org_slug,omitempty"`
}

// MarketRatingGetParams are the params for market.rating.get（查询自己对该包的评分）。
type MarketRatingGetParams struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// OrgSlug 组织源 slug（空 = 公共资源社区包；非空 = 查该组织专属空间的包）。
	OrgSlug string `json:"org_slug,omitempty"`
}

// MarketUpdateProfileParams are the params for market.updateProfile（更新显示名，§2.5）。
type MarketUpdateProfileParams struct {
	NickName string `json:"nick_name"` // ≤32 字符，服务端校验
}

// MarketUploadAvatarParams are the params for market.uploadAvatar（头像上传，§2.5）。
type MarketUploadAvatarParams struct {
	// DataBase64 头像图片二进制的 base64 编码（PNG/JPEG ≤1MB，服务端统一缩放 128×128）。
	DataBase64 string `json:"data_base64"`
	// Filename 原始文件名（multipart 字段名；缺省 avatar.png）。
	Filename string `json:"filename,omitempty"`
}

// MarketPackParams are the params for market.pack（打包不入库，发布表单预填数据）。
type MarketPackParams struct {
	Kind string `json:"kind"` // agent | skill | team
	Name string `json:"name"`
}

// MarketPublishParams are the params for market.publish（打包并发布为新版本，§3）。
type MarketPublishParams struct {
	Kind string `json:"kind"` // agent | skill | team
	Name string `json:"name"`
	// PublishName 发布名覆盖（§4.2 衍生改名发布；缺省 = 本地包名 Name）。
	// 写入包内清单 name，服务端身份以清单为准；403 包名冲突后走衍生路径时携带新名。
	PublishName string `json:"publish_name,omitempty"`
	// Version 包版本（写入清单 version；服务端版本来源以清单为权威）。
	Version string `json:"version,omitempty"`
	// License 许可证标识（SPDX 简称，写入清单 license）。
	License string `json:"license,omitempty"`
	// DerivedKind/DerivedName/DerivedVersion 衍生谱系声明（§4，写入清单 derived_from；
	// 原创发布三者均留空，声明时 kind+name 必填、version 可选）。
	DerivedKind    string `json:"derived_kind,omitempty"`
	DerivedName    string `json:"derived_name,omitempty"`
	DerivedVersion string `json:"derived_version,omitempty"`
	// OrgSlug 组织源 slug（空 = 发布到公共资源社区；非空 = 发布到该组织的专用源）。
	OrgSlug string `json:"org_slug,omitempty"`
}

// MarketSubmitParams are the params for market.submit（投递进化版本到包主收件箱，§4）。
type MarketSubmitParams struct {
	Kind string `json:"kind"` // agent | skill | team
	Name string `json:"name"` // 投递目标 = 原包名（清单身份须与目标一致，服务端强制）
	// Version 投递版本号（服务端必填校验；写入清单 version，X-Pkg-Version 头兜底）。
	Version string `json:"version"`
	// Note 给包主的附言（可选）。
	Note string `json:"note,omitempty"`
	// DerivedKind/DerivedName/DerivedVersion 谱系声明透传（可选；指向投递目标自身
	// 会被拒绝——同名包的进化版本谱系随包延续，不重声明）。
	DerivedKind    string `json:"derived_kind,omitempty"`
	DerivedName    string `json:"derived_name,omitempty"`
	DerivedVersion string `json:"derived_version,omitempty"`
}

// MarketSubmissionsListParams are the params for market.submissions.list（包主收件箱）。
type MarketSubmissionsListParams struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// MarketSubmissionDecideParams are the params for market.submissions.decide（审核投递）。
type MarketSubmissionDecideParams struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	ID   int64  `json:"id"`
	// Accept true = 采纳为正式新版本；false = 拒绝。
	Accept bool `json:"accept"`
}

func (c *Client) AgentExport(name, outPath string) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.export", AgentExportParams{Name: name, OutPath: outPath})
}

func (c *Client) AgentImport(path string, overwrite bool) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.import", AgentImportParams{Path: path, Overwrite: overwrite})
}

func (c *Client) SkillExport(name, outPath string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.export", SkillExportParams{Name: name, OutPath: outPath})
}

func (c *Client) SkillImport(path string, overwrite bool) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.import", SkillImportParams{Path: path, Overwrite: overwrite})
}

func (c *Client) MarketList(params MarketListParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.list", params)
}

// MarketSources 拉取可用市场源清单（商户源可见性由 daemon 侧配置的访问令牌决定）。
func (c *Client) MarketSources() (json.RawMessage, error) {
	return c.CallWithTimeout("market.sources", map[string]any{})
}

// MarketOrgsListParams are the params for market.org.list（当前账号所在组织清单，空参）。
type MarketOrgsListParams struct{}

// MarketOrgs 拉取当前账号所在的组织清单（发布/浏览组织源的货架切换数据源）。
func (c *Client) MarketOrgs() (json.RawMessage, error) {
	return c.CallWithTimeout("market.org.list", MarketOrgsListParams{})
}

func (c *Client) MarketInstall(kind, name string, overwrite, skipDeps, autoInstall bool) (json.RawMessage, error) {
	return c.CallWithTimeout("market.install", MarketInstallParams{Kind: kind, Name: name, Overwrite: overwrite, SkipDeps: skipDeps, AutoInstall: autoInstall})
}

func (c *Client) MarketPackageRead(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.package.read", MarketPackageReadParams{Name: name})
}

// MarketRate 提交包级评分（org_slug 非空时评该组织专属空间的包）。
func (c *Client) MarketRate(p MarketRateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.rate", p)
}

// MarketRatingGet 查询自己对该包的评分。
func (c *Client) MarketRatingGet(kind, name string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.rating.get", MarketRatingGetParams{Kind: kind, Name: name})
}

// MarketDeviceLogin 发起设备授权登录（daemon 打开确认页 + 后台轮询，受理即返回）。
func (c *Client) MarketDeviceLogin() (json.RawMessage, error) {
	return c.CallWithTimeout("market.device.login", map[string]any{})
}

// MarketDeviceStatus 查询设备登录编排状态（idle/pending/done/error）。
func (c *Client) MarketDeviceStatus() (json.RawMessage, error) {
	return c.CallWithTimeout("market.device.status", map[string]any{})
}

// MarketExperienceList 经验库全量清单（Agent 经验库；q 为标题/摘要关键词）。
func (c *Client) MarketExperienceList(q string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.experience.list", map[string]any{"q": q})
}

// MarketExperienceGet 经验文章全文。
func (c *Client) MarketExperienceGet(id int64) (json.RawMessage, error) {
	return c.CallWithTimeout("market.experience.get", map[string]any{"id": id})
}

// MarketUpdateProfile 更新MindX 账号显示名。
func (c *Client) MarketUpdateProfile(nickName string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.updateProfile", MarketUpdateProfileParams{NickName: nickName})
}

// MarketUploadAvatar 上传MindX 账号头像（base64 承载）。
func (c *Client) MarketUploadAvatar(dataBase64, filename string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.uploadAvatar", MarketUploadAvatarParams{DataBase64: dataBase64, Filename: filename})
}

// MarketAccountCredentialsParams 用户名密码注册/登录参数（账号机制定案 2026-10-10）。
type MarketAccountCredentialsParams struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name,omitempty"` // 仅注册用（缺省取用户名）
}

// MarketAccountProfileSetParams Profile 三字段编辑参数（2026-10-20 定案：github 移出
// 手填白名单，由 GitHub 身份绑定自动获取；nil 指针语义在 handler 内以字符串指针表达）。
type MarketAccountProfileSetParams struct {
	NickName *string `json:"nick_name,omitempty"`
	JobTitle *string `json:"job_title,omitempty"`
	Phone    *string `json:"phone,omitempty"`
}

// MarketAccountRegister 用户名密码注册（成功即登录并绑定设备）。
func (c *Client) MarketAccountRegister(p MarketAccountCredentialsParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.register", p)
}

// MarketAccountLogin 用户名密码登录（成功即绑定设备）。
func (c *Client) MarketAccountLogin(p MarketAccountCredentialsParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.login", p)
}

// MarketAccountLogout 退出登录（服务端吊销 + 本地清除）。
func (c *Client) MarketAccountLogout() (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.logout", map[string]any{})
}

// MarketAccountProfileSet 编辑 Profile 四字段。
func (c *Client) MarketAccountProfileSet(p MarketAccountProfileSetParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.profile.set", p)
}

// MarketAccountDevices 用户名下设备列表。
func (c *Client) MarketAccountDevices() (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.devices", map[string]any{})
}

// MarketAccountGet 查询MindX 账号状态（匿名设备身份 / 已绑定用户信息）。
func (c *Client) MarketAccountGet() (json.RawMessage, error) {
	return c.CallWithTimeout("market.account.get", map[string]any{})
}

// MarketPack 打包分发包（不入库），返回清单摘要与包大小供发布表单预填。
func (c *Client) MarketPack(kind, name string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.pack", MarketPackParams{Kind: kind, Name: name})
}

// MarketPublish 打包并发布为新版本（§3）。
func (c *Client) MarketPublish(p MarketPublishParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.publish", p)
}

// MarketSubmit 投递进化版本到包主收件箱（§4）。
func (c *Client) MarketSubmit(p MarketSubmitParams) (json.RawMessage, error) {
	return c.CallWithTimeout("market.submit", p)
}

// MarketSubmissionsList 拉取名下包的投递收件箱（§4）。
func (c *Client) MarketSubmissionsList(kind, name string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.submissions.list", MarketSubmissionsListParams{Kind: kind, Name: name})
}

// MarketSubmissionDecide 审核投递（采纳 / 拒绝，§4）。
func (c *Client) MarketSubmissionDecide(kind, name string, id int64, accept bool) (json.RawMessage, error) {
	return c.CallWithTimeout("market.submissions.decide", MarketSubmissionDecideParams{Kind: kind, Name: name, ID: id, Accept: accept})
}
