package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultMarketBase 是官方市场源的服务根地址（源语义：一个源 = 一个独立部署的服务根，
// 其下约定目录 agents/、skills/、plugins/（包文件）与 /api/v1（浏览 API）；
// 更换源即更换 base URL）。
const DefaultMarketBase = "https://www.mindx.chat"

// 市场交互的体积与超时上限。
const (
	apiMaxSize     = 10 << 20 // API 响应上限 10MB
	packageMaxSize = 200 << 20
	httpTimeout    = 15 * time.Second
)

// MarketPackage 描述一个可安装的分发包条目。预留扩展字段，不做超前设计。
type MarketPackage struct {
	Kind        PackageKind `json:"kind"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	// Title 展示标题（kind=plugin；服务端 meta 快照透出。卡片主名用标题，
	// name 是反向域 id（com.mindxwork.gitgraph）不宜直接示人）。
	Title string `json:"title,omitempty"`
	// NickName 昵称（kind=agent，2-3 字外号，展示主名；统一显示规则 = 昵称 + Role 小字）。
	NickName string `json:"nick_name,omitempty"`
	// Role 岗位头衔（kind=agent，包内清单透传；市场卡片大标题）。
	Role string `json:"role,omitempty"`
	// Category 业务分类（kind=agent，中文，包内清单透传；市场陈列与本地注册表同口径）。
	Category string `json:"category,omitempty"`
	// Leader 负责人 Agent 名（kind=team，包内清单透传；市场卡片展示）。
	Leader string `json:"leader,omitempty"`
	// Members 包内成员 Agent 名（kind=team，含负责人；市场卡片显示成员数）。
	Members []string `json:"members,omitempty"`
	// Skills 包内技能名（kind=agent，包内清单透传；市场卡片展示技能装配）。
	Skills []string `json:"skills,omitempty"`
	// SkillNames 技能中文展示名映射（技能名 → metadata.name_zh；
	// Agent 包记录包内全部技能，Skill 包记录其自身，包内清单透传；
	// 未声明的技能由展示端回退原名）。
	SkillNames map[string]string `json:"skill_names,omitempty"`
	// SkillDescs 技能描述映射（技能名 → SKILL.md frontmatter description，
	// 包内清单透传；收录语义同 SkillNames）。市场详情页展示用。
	SkillDescs map[string]string `json:"skill_descs,omitempty"`
	// Version 包版本（kind=skill 取自 SKILL.md frontmatter metadata.version，发布时录入）。
	Version string `json:"version,omitempty"`
	// File 包文件的缓存相对路径（内容寻址布局 packages/<kind>/<name>.mindpkg；
	// 仅用于本地缓存定位，不再承担清单拼接语义）。
	File string `json:"file,omitempty"`
	// DownloadURL 包文件下载直连地址（服务端 API 下发，安装直连）。
	DownloadURL string `json:"download_url,omitempty"`
	// Sha256 包文件内容的十六进制摘要。
	Sha256 string `json:"sha256"`
	// Size 包文件字节数（列表展示用）。
	Size int64 `json:"size,omitempty"`
	// DownloadCount 累计下载次数（服务端下载经手即计数，§1 口径；卡片信息行展示）。
	DownloadCount int64 `json:"download_count,omitempty"`
	// RatingAvg 评分均值 0-5（rating_count=0 时为 0，卡片不显示星标）。
	RatingAvg float64 `json:"rating_avg,omitempty"`
	// RatingCount 评分条数。
	RatingCount int64 `json:"rating_count,omitempty"`
	// 插件元数据（kind=plugin；安装确认弹窗与插件页卡片的信任呈现依据）。
	Author      string   `json:"author,omitempty"`
	URL         string   `json:"url,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	License     string   `json:"license,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	// Fingerprint 插件唯一指纹（sha256(canonical({author,id,version}))，服务端发布时校验，
	// 版本代际身份标识；kind=plugin 必带）。
	Fingerprint string `json:"fingerprint,omitempty"`
	// PublishedAt 最新版本发布时间（RFC3339，服务端下发；插件卡片展示发布日期）。
	PublishedAt string `json:"published_at,omitempty"`
	// DerivedFrom 衍生谱系（§4）：本包声明进化自哪个包（服务端下发；原创为 nil）。
	DerivedFrom *DerivedRef `json:"derived_from,omitempty"`
	// DerivedCount 在架衍生包数（仅详情 API 下发；「衍生 N 个」展示用）。
	DerivedCount int64 `json:"derived_count,omitempty"`
}

// MarketQuery 是分页浏览市场的查询参数（契约 §2.2，全部可选）。
type MarketQuery struct {
	Page     int    `json:"page,omitempty"`      // 1 起始，缺省 1
	PageSize int    `json:"page_size,omitempty"` // 缺省 50，服务端上限 200
	Kind     string `json:"kind,omitempty"`      // agent | skill | team
	Q        string `json:"q,omitempty"`         // 对 name/description 查找
	Sort     string `json:"sort,omitempty"`      // likes | newest | downloads
}

// MarketPage 是分页浏览的结果（服务端 GET /api/v1/packages 响应归一化）。
type MarketPage struct {
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	Packages []MarketPackage `json:"packages"`
	Source   string          `json:"source"`
}

// MarketClient 消费市场：全部数据走服务端分页 API（无清单回退）；
// 分发包下载带 sha256 校验。pat 非空时所有出站请求携带 Authorization（商户源鉴权）；
// clientID/authToken 非空时出站请求附 X-Client-Id 与用户身份（评分/绑定等低价值写操作，
// 服务端 RequireActor 双通道归一）。
type MarketClient struct {
	httpClient *http.Client
	// base 源服务根地址（源语义：一个源 = 一个独立部署的服务根，换源 = 换此地址）
	base string
	// apiBase 派生的 API 基址（<base>/api/v1）
	apiBase  string
	cacheDir string
	// pat 专用源访问令牌（AT，网页签发经设置粘贴；空 = 匿名。字段名 pat 历史沿用）
	pat string
	// authToken 用户登录令牌（MindX 账号绑定后录入；空 = 未绑定，评分走匿名设备通道）
	authToken string
	// clientID 设备身份 UUIDv4（daemon 首启生成并持久化进凭据存储；评分/绑定的
	// X-Client-Id 通道；服务端校验 clients 表登记）
	clientID string
	// registered 标记设备身份是否已在当前源登记（懒登记：首个身份动作前幂等注册）
	registered bool
}

// SetPAT 设置专用源访问令牌（AT；换令牌后由调用方重建客户端或直接复用本实例均可）。
// 方法名 pat 历史沿用（其本义是 market 访问令牌/AT，非对端认证 PAT）。
func (m *MarketClient) SetPAT(pat string) {
	m.pat = strings.TrimSpace(pat)
}

// SetAuthToken 设置用户登录令牌（MindX 账号绑定/解绑后由调用方同步）。
func (m *MarketClient) SetAuthToken(token string) {
	m.authToken = strings.TrimSpace(token)
	m.registered = false // 身份语义变化后重新登记（幂等）
}

// SetClientID 设置设备身份标识（daemon 首启生成；换源后同一身份在不同源各自登记）。
func (m *MarketClient) SetClientID(id string) {
	m.clientID = strings.TrimSpace(id)
	m.registered = false
}

// authedGet 带凭证的 GET：Authorization 优先 authToken（用户身份）回退 pat（商户源）；
// clientID 非空时附 X-Client-Id（RequireActor 匿名设备通道）。
func (m *MarketClient) authedGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	m.attachIdentity(req)
	return m.httpClient.Do(req)
}

// attachIdentity 为出站请求附加身份头。
func (m *MarketClient) attachIdentity(req *http.Request) {
	token := m.authToken
	if token == "" {
		token = m.pat
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if m.clientID != "" {
		req.Header.Set("X-Client-Id", m.clientID)
	}
}

// ensureIdentity 身份动作（评分/绑定）前的懒登记：设备身份幂等注册到当前源，
// 已登记过则跳过（内存标记；换源/SetClientID 后重置）。
func (m *MarketClient) ensureIdentity() error {
	if m.clientID == "" || m.registered {
		return nil
	}
	if err := m.RegisterClient(m.clientID); err != nil {
		return err
	}
	m.registered = true
	return nil
}

// MarketUserIdentity 账号绑定的社区登录身份（GET /me identities 行；社区账号分组渲染源）。
type MarketUserIdentity struct {
	Provider  string `json:"provider"`
	Login     string `json:"login"`
	NickName  string `json:"nick_name"`
	AvatarURL string `json:"avatar_url"`
	IsPrimary bool   `json:"is_primary"`
	CreatedAt string `json:"created_at"`
}

// MarketUser MindX 账号用户信息（服务端 GET /api/v1/me 响应）。
type MarketUser struct {
	// ID 用户唯一标识（TEXT UUID；2026-10-26 用户 ID 全链换 UUID 防枚举）
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Login     string `json:"login"`
	NickName  string `json:"nick_name"`
	AvatarURL string `json:"avatar_url"`
	Points    int64  `json:"points"`
	TOSAccept bool   `json:"tos_accepted"`
	// 账号机制字段（用户名密码登录 + Profile 四字段）
	Username string `json:"username"`
	JobTitle string `json:"job_title"`
	Phone    string `json:"phone"`
	Github   string `json:"github"`
	// Email 邮箱（2026-10-24 定案：第三方授权带回即同步，Profile 可编辑）
	Email string `json:"email"`
	// 多社区身份（2026-10-20 定案）：绑定身份全表 + 服务端可用提供商清单
	Identities         []MarketUserIdentity `json:"identities"`
	AvailableProviders []string             `json:"available_providers"`
}

// RegisterClient 登记设备身份（服务端幂等：已存在不覆盖绑定状态）。
func (m *MarketClient) RegisterClient(clientID string) error {
	return m.postJSON(m.apiBase+"/clients", map[string]string{"client_id": clientID}, nil)
}

// Me 拉取当前登录用户信息（AuthToken 校验通道；401/无令牌返回 ErrMarketUnauthorized 语义错误）。
func (m *MarketClient) Me() (*MarketUser, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	resp, err := m.authedGet(m.apiBase + "/me")
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("令牌校验失败（HTTP %d）", resp.StatusCode)
	}
	var u MarketUser
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, fmt.Errorf("解析用户信息失败：%w", err)
	}
	return &u, nil
}

// Bind 将本设备绑定到 AuthToken 对应用户（服务端单事务：user_id 落值 + 匿名评分迁移）。
// deviceName/platform 为设备元数据上报（设备列表渲染用；空值不覆盖）。幂等、单向；返回迁移的匿名评分条数。
func (m *MarketClient) Bind(deviceName, platform string) (int64, error) {
	if m.authToken == "" {
		return 0, fmt.Errorf("尚未登录社区")
	}
	if err := m.ensureIdentity(); err != nil {
		return 0, err
	}
	var out struct {
		Bound    bool  `json:"bound"`
		Migrated int64 `json:"migrated"`
	}
	body := map[string]string{"client_id": m.clientID, "device_name": deviceName, "platform": platform}
	if err := m.postJSON(m.apiBase+"/clients/bind", body, &out); err != nil {
		return 0, err
	}
	return out.Migrated, nil
}

// Register 用户名密码注册（账号机制定案 2026-10-10）：成功下发 AuthToken。
func (m *MarketClient) Register(username, password, displayName string) (string, error) {
	var out struct {
		AuthToken string `json:"auth_token"`
	}
	body := map[string]string{"username": username, "password": password, "display_name": displayName}
	if err := m.postJSON(m.apiBase+"/auth/register", body, &out); err != nil {
		return "", err
	}
	if out.AuthToken == "" {
		return "", fmt.Errorf("注册响应缺少令牌")
	}
	return out.AuthToken, nil
}

// Login 用户名密码登录：成功下发 AuthToken。
func (m *MarketClient) Login(username, password string) (string, error) {
	var out struct {
		AuthToken string `json:"auth_token"`
	}
	if err := m.postJSON(m.apiBase+"/auth/login", map[string]string{"username": username, "password": password}, &out); err != nil {
		return "", err
	}
	if out.AuthToken == "" {
		return "", fmt.Errorf("登录响应缺少令牌")
	}
	return out.AuthToken, nil
}

// Logout 吊销当前 AuthToken（服务端令牌作废；本地凭据清除由调用方负责）。
func (m *MarketClient) Logout() error {
	if m.authToken == "" {
		return nil
	}
	return m.postJSON(m.apiBase+"/auth/logout", map[string]any{}, nil)
}

// MarketDevice 用户名下设备条目（服务端 GET /api/v1/devices）。
type MarketDevice struct {
	ClientID   string `json:"client_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	BoundAt    string `json:"bound_at"`
	LastSeenAt string `json:"last_seen_at"`
}

// TrustTicket 同账号互信凭据（服务端 POST /devices/trust/start 下发）：
// nonce 绑定本机登记设备身份（client_id）与账号（user_id，TEXT UUID），5 分钟内幂等可核验。
type TrustTicket struct {
	Nonce     string `json:"nonce"`
	ClientID  string `json:"client_id"`
	UserID    string `json:"user_id"`
	ExpiresIn int    `json:"expires_in"`
}

// DeviceTrustStart 领取同账号互信凭据（跨端连接第二步）：凭 AuthToken 签发，
// 绑定本机登记设备身份（client_id）；未登录或设备未绑定账号时服务端拒绝。
func (m *MarketClient) DeviceTrustStart() (*TrustTicket, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	if m.clientID == "" {
		return nil, fmt.Errorf("设备身份未初始化")
	}
	var out TrustTicket
	if err := m.postJSON(m.apiBase+"/devices/trust/start", map[string]string{"client_id": m.clientID}, &out); err != nil {
		return nil, err
	}
	if out.Nonce == "" {
		return nil, fmt.Errorf("互信凭据响应缺少 nonce")
	}
	return &out, nil
}

// TrustVerdict 互信凭据核验结论材料：同账号判定在本地完成
// （IssuerUserID == CallerUserID 即同账号；IssuerClientID 须与对端自报身份一致）。
// 用户 ID 为 TEXT UUID（空串 = 未绑定）。
type TrustVerdict struct {
	IssuerUserID   string `json:"issuer_user_id"`
	IssuerClientID string `json:"issuer_client_id"`
	CallerUserID   string `json:"caller_user_id"`
}

// SameAccount 本地判定：凭据签发方与调用方是否同一账号（服务端背书双方身份）。
func (v *TrustVerdict) SameAccount() bool {
	return v.IssuerUserID != "" && v.IssuerUserID == v.CallerUserID
}

// DeviceTrustConfirm 核验对端出示的互信凭据，换回签发方与调用方双方身份：
// 同账号判定（SameAccount）与设备身份核对由调用方本地完成。
// 凭据不存在或已过期时返回错误（错误文本注明），调用方回落既有配对流程。
func (m *MarketClient) DeviceTrustConfirm(nonce string) (*TrustVerdict, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	body, err := json.Marshal(map[string]string{"nonce": nonce})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, m.apiBase+"/devices/trust/confirm", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("互信凭据无效或已过期")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
		return nil, apiErrorMessage(data, resp.StatusCode)
	}
	var out TrustVerdict
	if err := json.NewDecoder(io.LimitReader(resp.Body, apiMaxSize)).Decode(&out); err != nil {
		return nil, fmt.Errorf("解析互信凭据响应失败：%w", err)
	}
	return &out, nil
}

// Devices 拉取登录用户名下设备列表。
func (m *MarketClient) Devices() ([]MarketDevice, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	resp, err := m.authedGet(m.apiBase + "/devices")
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("拉取设备列表失败（HTTP %d）", resp.StatusCode)
	}
	var out struct {
		Devices []MarketDevice `json:"devices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析设备列表失败：%w", err)
	}
	return out.Devices, nil
}

// MoveExperience 挪挂载点 + 兄弟重排（PATCH /api/v1/experiences/{id}/position）：
// parentID 空 = 挂回该空间根层级；siblings 为挪动完成后目标层级的完整有序 ID 列表
// （含被挪文自身）。权限与防环由服务端裁决：组织树限文章作者本人或组织管理员，
// 公共树会员一律拒绝（官方走管理通道）；挂载点不合法/兄弟清单不符返回 422，
// 错误消息透传服务端的人话说明。
func (m *MarketClient) MoveExperience(id int64, parentID *int64, siblings []int64) error {
	body := map[string]any{"parent_id": parentID, "siblings": siblings}
	return m.patchJSON(fmt.Sprintf("%s/experiences/%d/position", m.apiBase, id), body, nil)
}

// IdentityLinkStart 发起社区账号绑定（POST /api/v1/me/identities/link）：
// 服务端暂存绑定请求并返回一次性授权入口 URL（由 daemon 经 link_open 广播开浏览器）。
func (m *MarketClient) IdentityLinkStart(provider string) (linkID, authURL string, err error) {
	if m.authToken == "" {
		return "", "", fmt.Errorf("尚未登录社区")
	}
	var out struct {
		LinkID  string `json:"link_id"`
		AuthURL string `json:"auth_url"`
	}
	if err := m.postJSON(m.apiBase+"/me/identities/link", map[string]string{"provider": provider}, &out); err != nil {
		return "", "", err
	}
	if out.LinkID == "" || out.AuthURL == "" {
		return "", "", fmt.Errorf("绑定发起响应缺少授权地址")
	}
	return out.LinkID, out.AuthURL, nil
}

// IdentityLinkStatus 查询绑定进度（GET /api/v1/me/identities/link/{id}/status）：
// pending | done | failed（failed 时 errMsg 带原因）。
func (m *MarketClient) IdentityLinkStatus(linkID string) (status, errMsg string, err error) {
	if m.authToken == "" {
		return "", "", fmt.Errorf("尚未登录社区")
	}
	resp, err := m.authedGet(m.apiBase + "/me/identities/link/" + url.PathEscape(linkID) + "/status")
	if err != nil {
		return "", "", fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("查询绑定进度失败（HTTP %d）", resp.StatusCode)
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", fmt.Errorf("解析绑定进度失败：%w", err)
	}
	return out.Status, out.Error, nil
}

// IdentityUnlink 解绑社区账号（DELETE /api/v1/me/identities/{provider}；
// 主身份与最后一条身份由服务端拒绝）。
func (m *MarketClient) IdentityUnlink(provider string) error {
	if m.authToken == "" {
		return fmt.Errorf("尚未登录社区")
	}
	req, err := http.NewRequest(http.MethodDelete, m.apiBase+"/me/identities/"+url.PathEscape(provider), nil)
	if err != nil {
		return err
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return fmt.Errorf("%s", e.Error.Message)
	}
	return fmt.Errorf("解绑失败（HTTP %d）", resp.StatusCode)
}

// DeleteVersion 作者下架自己包的某版本（DELETE /api/v1/packages/{kind}/{name}/versions/{version}；
// 服务端软删：清单不再列出，文件与审计保留；仅包主可操作）。管理员整包下架走
// 服务端 admin 通道（X-Admin-Token），不进本客户端——daemon 以用户身份运行，不持有管理凭据。
func (m *MarketClient) DeleteVersion(kind, name, version string) error {
	if m.authToken == "" {
		return fmt.Errorf("尚未登录社区")
	}
	req, err := http.NewRequest(http.MethodDelete, m.apiBase+"/packages/"+url.PathEscape(kind)+"/"+url.PathEscape(name)+"/versions/"+url.PathEscape(version), nil)
	if err != nil {
		return err
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return fmt.Errorf("%s", e.Error.Message)
	}
	return fmt.Errorf("下架失败（HTTP %d）", resp.StatusCode)
}

// OrgCreate 创建组织（POST /api/v1/orgs：仅需名称与别名，创建者即所有者）。
func (m *MarketClient) OrgCreate(name, slug string) (orgID int64, err error) {
	if m.authToken == "" {
		return 0, fmt.Errorf("尚未登录社区")
	}
	var out struct {
		OrgID int64  `json:"org_id"`
		Slug  string `json:"slug"`
	}
	if err := m.postJSON(m.apiBase+"/orgs", map[string]string{"name": name, "slug": slug}, &out); err != nil {
		return 0, err
	}
	return out.OrgID, nil
}

// OrgDelete 删除组织（DELETE /api/v1/orgs/{slug}；仅所有者，且组织须为空：
// 无其他成员且无已发布包，否则服务端 409 报原因）。
func (m *MarketClient) OrgDelete(slug string) error {
	if m.authToken == "" {
		return fmt.Errorf("尚未登录社区")
	}
	req, err := http.NewRequest(http.MethodDelete, m.apiBase+"/orgs/"+url.PathEscape(slug), nil)
	if err != nil {
		return err
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return fmt.Errorf("%s", e.Error.Message)
	}
	return fmt.Errorf("删除组织失败（HTTP %d）", resp.StatusCode)
}

// OrgSubscription 组织订阅状态（GET /api/v1/orgs/{slug}/subscription；owner/admin）。
// 免费开放期 status = none；组织中心据 status 渲染「免费开放中」或订阅明细。
type OrgSubscription struct {
	Status      string `json:"status"`
	SeatCount   int64  `json:"seat_count"`
	TotalAmount int64  `json:"total_amount"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Seats       struct {
		Total    int `json:"total"`
		Occupied int `json:"occupied"`
		Open     int `json:"open"`
	} `json:"seats"`
}

// OrgSubscription 拉取组织订阅状态（免费开放期返回 status=none，不报错）。
func (m *MarketClient) OrgSubscription(orgSlug string) (*OrgSubscription, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) + "/subscription")
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取订阅状态失败（HTTP %d）", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	var out OrgSubscription
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析订阅状态失败：%w", err)
	}
	return &out, nil
}

// UpdateProfileFields 更新 Profile 字段（PATCH /api/v1/me；nil = 不更新。
// 2026-10-20 定案：github 移出手填白名单，由 GitHub 身份绑定自动获取；
// 2026-10-24 定案：email 加入白名单，第三方授权带回即同步 + 用户可自改自清）。
func (m *MarketClient) UpdateProfileFields(nickName, jobTitle, phone, email *string) (*MarketUser, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("尚未登录社区")
	}
	body := map[string]any{}
	if nickName != nil {
		body["nick_name"] = *nickName
	}
	if jobTitle != nil {
		body["job_title"] = *jobTitle
	}
	if phone != nil {
		body["phone"] = *phone
	}
	if email != nil {
		body["email"] = *email
	}
	if len(body) == 0 {
		return m.Me()
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, m.apiBase+"/me", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, apiErrorMessage(data, resp.StatusCode)
	}
	return m.Me()
}

// Base 返回市场源服务根地址（设备登录确认页 URL 的拼接基址；与登录/绑定
// 走同一 base，保证令牌签发源与校验源一致）。
func (m *MarketClient) Base() string { return m.base }

// DeviceLoginStart 设备授权登录发起结果（服务端 POST /auth/device/start 响应）。
type DeviceLoginStart struct {
	DeviceCode      string `json:"device_code"`
	VerificationURL string `json:"verification_url"`    // 确认页 URL（已带 code，客户端直接打开）
	LoginURL        string `json:"login_url,omitempty"` // 直达授权 URL（带 provider 时下发；回调后自动完成设备授权）
	ExpiresIn       int    `json:"expires_in"`          // 会话有效期（秒）
	Interval        int    `json:"interval"`            // 建议轮询间隔（秒）
}

// DeviceStart 发起设备授权登录（服务端创建 pending 会话）。provider 非空时
// 服务端额外下发直达该提供商授权页的 LoginURL（直通链路，免确认页点击）。
func (m *MarketClient) DeviceStart(provider string) (*DeviceLoginStart, error) {
	body := map[string]any{}
	if provider = strings.TrimSpace(provider); provider != "" {
		body["provider"] = provider
	}
	var out DeviceLoginStart
	if err := m.postJSON(m.apiBase+"/auth/device/start", body, &out); err != nil {
		return nil, err
	}
	if out.DeviceCode == "" || out.VerificationURL == "" {
		return nil, fmt.Errorf("设备登录响应缺少 device_code/verification_url")
	}
	return &out, nil
}

// LoginProviders 当前部署已配置的登录方式清单（GET /auth/providers，匿名），
// 供客户端登录页渲染第三方登录按钮（未登录场景拿不到 /me）。
func (m *MarketClient) LoginProviders() ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, m.apiBase+"/auth/providers", nil)
	if err != nil {
		return nil, err
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取登录方式失败（HTTP %d）", resp.StatusCode)
	}
	var out struct {
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, apiMaxSize)).Decode(&out); err != nil {
		return nil, err
	}
	return out.Providers, nil
}

// DevicePollResult 设备授权轮询响应形态。
type DevicePollResult struct {
	Status    string `json:"status"`     // pending | authorized
	AuthToken string `json:"auth_token"` // authorized 时下发（明文仅此一次）
}

// DevicePoll 轮询设备授权状态。返回 status（pending/authorized）与签发的
// AuthToken；会话不存在/过期（服务端 expired_token）返回错误，调用方应终止轮询。
func (m *MarketClient) DevicePoll(deviceCode string) (*DevicePollResult, error) {
	var out DevicePollResult
	if err := m.postJSON(m.apiBase+"/auth/device/poll", map[string]string{"device_code": deviceCode}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RatingAggregate 包级评分聚合（评分响应回传）。
type RatingAggregate struct {
	Avg   float64 `json:"rating_avg"`
	Count int64   `json:"rating_count"`
}

// UpdateProfile 更新当前用户显示名（PATCH /api/v1/me；昵称 ≤32 字符由服务端校验）。
func (m *MarketClient) UpdateProfile(nickName string) (string, error) {
	if m.authToken == "" {
		return "", fmt.Errorf("尚未登录社区")
	}
	body, err := json.Marshal(map[string]string{"nick_name": nickName})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPatch, m.apiBase+"/me", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", apiErrorMessage(data, resp.StatusCode)
	}
	var out struct {
		NickName string `json:"nick_name"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}
	return out.NickName, nil
}

// UploadAvatar 上传头像（POST /api/v1/me/avatar，multipart 字段 avatar）：
// PNG/JPEG ≤1MB、≤1024×1024，服务端统一缩放 128×128，返回档案头像 URL。
func (m *MarketClient) UploadAvatar(data []byte, filename string) (string, error) {
	if m.authToken == "" {
		return "", fmt.Errorf("尚未登录社区")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("头像数据为空")
	}
	if len(data) > 1<<20 {
		return "", fmt.Errorf("头像超过 1MB 上限")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("avatar", filename)
	if err != nil {
		return "", err
	}
	if _, fwErr := fw.Write(data); fwErr != nil {
		return "", fwErr
	}
	if closeErr := w.Close(); closeErr != nil {
		return "", closeErr
	}
	req, err := http.NewRequest(http.MethodPost, m.apiBase+"/me/avatar", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respData, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", apiErrorMessage(respData, resp.StatusCode)
	}
	var out struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(respData, &out); err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}
	return out.AvatarURL, nil
}

// ─── 发布与投递（市场生态 §3/§4）───

// PublishResult 发布响应（服务端 POST .../versions）。
type PublishResult struct {
	PackageID int64  `json:"package_id"`
	Version   string `json:"version"`
	Sha256    string `json:"sha256"`
	Created   bool   `json:"created"`
}

// postRaw 原始字节 POST（发布/投递共用传输形态：octet-stream + 附加头）。
func (m *MarketClient) postRaw(u string, data []byte, headers map[string]string, out any) error {
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respData, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiErrorMessage(respData, resp.StatusCode)
	}
	if out != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// Publish 发布分发包新版本（§3）：原始 mindpkg 字节直传 + sha256 完整性承诺，
// 服务端复算校验；同 (kind,name,version) 同 sha256 幂等返回 created=false。
// 版本号以包内清单 version 为权威（version 参数作为 X-Pkg-Version 头兜底，
// 供清单未写版本的旧包使用）。publishName 非空 = 衍生改名发布（§4.2）：
// URL 与清单身份一致使用新名（包内清单 name 已随打包覆盖）。
func (m *MarketClient) Publish(kind, name string, data []byte, version, publishName string) (*PublishResult, error) {
	return m.publishTo(m.apiBase+"/packages", kind, name, data, version, publishName)
}

// PublishToOrg 发布分发包新版本至组织源（M7 组织化协作最后一环）：
// 与 Publish 全链路同语义（sha256 承诺、版本不可变、幂等），仅目标端点不同——
// 服务端校验本人在册成员身份，产物落组织专属源（orgs/{org_id}/），对公共货架不可见。
// orgSlug 为组织别名（发布弹窗目标选择的数据源来自 MeOrgs）。
func (m *MarketClient) PublishToOrg(orgSlug, kind, name string, data []byte, version, publishName string) (*PublishResult, error) {
	return m.publishTo(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/packages", kind, name, data, version, publishName)
}

// publishTo 发布共用传输（公共货架与组织源仅 URL 前缀不同）。
func (m *MarketClient) publishTo(prefix, kind, name string, data []byte, version, publishName string) (*PublishResult, error) {
	if m.authToken == "" && m.pat == "" {
		return nil, fmt.Errorf("发布前请先绑定 MindX 账号或配置商户授权")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("分发包内容为空")
	}
	if pn := strings.TrimSpace(publishName); pn != "" {
		name = pn
	}
	sum := sha256.Sum256(data)
	headers := map[string]string{"X-Pkg-Sha256": hex.EncodeToString(sum[:])}
	if v := strings.TrimSpace(version); v != "" {
		headers["X-Pkg-Version"] = v
	}
	var out PublishResult
	u := prefix + "/" + url.PathEscape(kind) + "/" + url.PathEscape(name) + "/versions"
	if err := m.postRaw(u, data, headers, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Submission 投递记录（收件箱条目，服务端 GET .../submissions 响应）。
// FromUserID 为投递者用户 UUID（TEXT）。
type Submission struct {
	ID         int64  `json:"id"`
	FromUserID string `json:"from_user_id"`
	FromLogin  string `json:"from_login"`
	Version    string `json:"version"`
	Sha256     string `json:"sha256"`
	Note       string `json:"note"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

// SubmitResult 投递响应（服务端 POST .../submissions）。
type SubmitResult struct {
	SubmissionID int64  `json:"submission_id"`
	Version      string `json:"version"`
	Sha256       string `json:"sha256"`
	Created      bool   `json:"created"`
}

// Submit 投递进化版本到包主私有收件箱（§4 回流路径：不经货架，owner 采纳后
// 并入正式新版本）。version 为投递版本号（写入清单为主，头兜底）；
// note 为给包主的附言（可选）。
func (m *MarketClient) Submit(kind, name, note, version string, data []byte) (*SubmitResult, error) {
	if m.authToken == "" && m.pat == "" {
		return nil, fmt.Errorf("投递前请先绑定 MindX 账号")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("分发包内容为空")
	}
	sum := sha256.Sum256(data)
	headers := map[string]string{"X-Pkg-Sha256": hex.EncodeToString(sum[:])}
	if v := strings.TrimSpace(version); v != "" {
		headers["X-Pkg-Version"] = v
	}
	u := m.apiBase + "/packages/" + url.PathEscape(kind) + "/" + url.PathEscape(name) + "/submissions"
	if note = strings.TrimSpace(note); note != "" {
		u += "?note=" + url.QueryEscape(note)
	}
	var out SubmitResult
	if err := m.postRaw(u, data, headers, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Submissions 拉取名下包的投递收件箱（仅包主可见；服务端校验 owner 身份）。
func (m *MarketClient) Submissions(kind, name string) ([]Submission, error) {
	resp, err := m.authedGet(m.apiBase + "/packages/" + url.PathEscape(kind) + "/" + url.PathEscape(name) + "/submissions")
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiErrorMessage(data, resp.StatusCode)
	}
	var body struct {
		Submissions []Submission `json:"submissions"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析收件箱失败：%w", err)
	}
	return body.Submissions, nil
}

// DecideResult 投递决策响应（accept 携带发布结果，reject 仅回状态）。
type DecideResult struct {
	SubmissionID int64  `json:"submission_id"`
	PkgID        int64  `json:"pkg_id,omitempty"`
	Version      string `json:"version,omitempty"`
	Sha256       string `json:"sha256,omitempty"`
	Created      bool   `json:"created,omitempty"`
	Status       string `json:"status,omitempty"`
}

// DecideSubmission 包主审核投递（§4）：accept 采纳为正式新版本（服务端代转发布
// 管线），reject 拒绝。重复决策由服务端以 409 invalid_state 拒绝。
func (m *MarketClient) DecideSubmission(kind, name string, submissionID int64, accept bool) (*DecideResult, error) {
	if m.authToken == "" && m.pat == "" {
		return nil, fmt.Errorf("审核投递前请先绑定 MindX 账号")
	}
	action := "reject"
	if accept {
		action = "accept"
	}
	u := m.apiBase + "/packages/" + url.PathEscape(kind) + "/" + url.PathEscape(name) +
		"/submissions/" + strconv.FormatInt(submissionID, 10) + "/" + action
	var out DecideResult
	if err := m.postRaw(u, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ErrRatingNotFound 查询自己评分时无记录（服务端 404 语义），调用方按未评分处理。
var ErrRatingNotFound = fmt.Errorf("尚未评分")

// Rate 对包（kind+name 包级，不区分版本）提交 1-5 星评分；返回重算后的聚合。
func (m *MarketClient) Rate(kind, name string, score int64) (*RatingAggregate, error) {
	return m.rateAt("/packages", kind, name, score)
}

// RateInOrg 对组织包提交评分（M8 组织回路；企业内实名口径，仅本人在册组织）。
func (m *MarketClient) RateInOrg(orgSlug, kind, name string, score int64) (*RatingAggregate, error) {
	return m.rateAt("/orgs/"+url.PathEscape(orgSlug)+"/packages", kind, name, score)
}

// rateAt 评分共用提交（prefix 区分公共 /orgs/{slug} 回路）。
func (m *MarketClient) rateAt(prefix, kind, name string, score int64) (*RatingAggregate, error) {
	if score < 1 || score > 5 {
		return nil, fmt.Errorf("score 须为 1-5 整数")
	}
	if err := m.ensureIdentity(); err != nil {
		return nil, err
	}
	var agg RatingAggregate
	if err := m.postJSON(m.apiBase+prefix+"/"+url.PathEscape(kind)+"/"+url.PathEscape(name)+"/rating",
		map[string]int64{"score": score}, &agg); err != nil {
		return nil, err
	}
	return &agg, nil
}

// MyRating 查询自己（当前身份通道）对包的评分；未评分返回 errRatingNotFound。
func (m *MarketClient) MyRating(kind, name string) (int64, error) {
	return m.myRatingAt("/packages", kind, name)
}

// MyRatingInOrg 查询自己对组织包的评分（M8 组织回路）。
func (m *MarketClient) MyRatingInOrg(orgSlug, kind, name string) (int64, error) {
	return m.myRatingAt("/orgs/"+url.PathEscape(orgSlug)+"/packages", kind, name)
}

// myRatingAt 我的评分共用查询（prefix 区分公共 /orgs/{slug} 回路）。
func (m *MarketClient) myRatingAt(prefix, kind, name string) (int64, error) {
	resp, err := m.authedGet(m.apiBase + prefix + "/" + url.PathEscape(kind) + "/" + url.PathEscape(name) + "/rating")
	if err != nil {
		return 0, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return 0, ErrRatingNotFound
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("查询评分失败（HTTP %d）", resp.StatusCode)
	}
	var body struct {
		Score int64 `json:"score"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return 0, fmt.Errorf("解析评分失败：%w", err)
	}
	return body.Score, nil
}

// postJSON 通用 JSON POST：body 编码发出，out 非 nil 时解析响应；非 2xx 映射为含状态的错误。
func (m *MarketClient) postJSON(url string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiErrorMessage(data, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// patchJSON 通用 JSON PATCH（与 postJSON 同构：编码发出、非 2xx 映射为含状态的错误）。
func (m *MarketClient) patchJSON(url string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiErrorMessage(data, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// NewMarketClient 创建市场客户端。
// baseURL 为市场源服务根地址（空 = 官方 DefaultMarketBase；本地调试/自托管/换源显式传入）；
// 其下约定包目录 agents/、skills/、plugins/ 与 API /api/v1。
// cacheDir 为缓存根目录：内部按 baseURL 哈希开子目录（market-cache/<sha8>），
// 换源（= 换 base URL）时包缓存互不串扰。
func NewMarketClient(baseURL, cacheDir string) *MarketClient {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultMarketBase
	}
	sum := sha256.Sum256([]byte(base))
	cacheDir = filepath.Join(cacheDir, hex.EncodeToString(sum[:])[:8])
	return &MarketClient{
		httpClient: &http.Client{Timeout: httpTimeout},
		base:       base,
		apiBase:    base + "/api/v1",
		cacheDir:   cacheDir,
	}
}

// apiPackage 服务端包条目的原始响应结构（列表/详情一致）：基础字段在顶层，
// 展示元数据（昵称/负责人/成员/技能装配）在嵌套 meta 对象中。
type apiPackage struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Category      string         `json:"category"`
	DownloadURL   string         `json:"download_url"`
	Sha256        string         `json:"sha256"`
	Size          int64          `json:"size"`
	DownloadCount int64          `json:"download_count"`
	RatingAvg     float64        `json:"rating_avg"`
	RatingCount   int64          `json:"rating_count"`
	Version       string         `json:"version"`
	PublishedAt   string         `json:"published_at"`
	DerivedFrom   *DerivedRef    `json:"derived_from"`
	DerivedCount  int64          `json:"derived_count"`
	Meta          apiPackageMeta `json:"meta"`
}

// toMarketPackage 转为统一包条目（分类归一；版本优先取 meta.version，回退顶层）。
func (ap apiPackage) toMarketPackage() MarketPackage {
	version := ap.Meta.Version
	if version == "" {
		version = ap.Version
	}
	return MarketPackage{
		Kind: PackageKind(ap.Kind), Name: ap.Name, Description: ap.Description,
		Icon: ap.Meta.Icon, Title: ap.Meta.Title, NickName: ap.Meta.NickName, Role: ap.Meta.Role,
		Category: normalizeCategory(ap.Category), Leader: ap.Meta.Leader, Members: ap.Meta.Members,
		Skills: ap.Meta.Skills, SkillNames: ap.Meta.SkillNames, SkillDescs: ap.Meta.SkillDescs,
		Version: version, DownloadURL: ap.DownloadURL, Sha256: ap.Sha256, Size: ap.Size,
		DownloadCount: ap.DownloadCount, RatingAvg: ap.RatingAvg, RatingCount: ap.RatingCount,
		PublishedAt: ap.PublishedAt,
		Author:      ap.Meta.Author, URL: ap.Meta.URL, Repo: ap.Meta.Repo, License: ap.Meta.License,
		Permissions: ap.Meta.Permissions, Fingerprint: ap.Meta.Fingerprint,
		DerivedFrom: ap.DerivedFrom, DerivedCount: ap.DerivedCount,
	}
}

// MarketSource 市场源条目（服务端 GET /api/v1/sources 下发）。
// BaseURL 为源服务根地址（空 = 本服务，即下发该列表的服务本身）。
type MarketSource struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Tier    string `json:"tier"`
}

// Sources 拉取可用市场源清单（商户用户携带 PAT 可见商户源；default 为空 id 语义）。
func (m *MarketClient) Sources() ([]MarketSource, error) {
	resp, err := m.authedGet(m.apiBase + "/sources")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("源列表响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	var body struct {
		Sources []MarketSource `json:"sources"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析源列表失败：%w", err)
	}
	return body.Sources, nil
}

// ResolveSourceBase 校验源 id 并返回其服务根地址：空或 "default" = 服务端声明的
// 默认源（源列表首位，base 随服务端下发——自托管/镜像部署下即本服务地址，而非
// 客户端硬编码的官方地址）；其余 id 必须出现在源列表中，返回该源 base_url
// （源列表拉取失败或 id 不在列表时报错，调用方回退官方源）。
func (m *MarketClient) ResolveSourceBase(sourceID string) (string, error) {
	sources, err := m.Sources()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sourceID) == "" || sourceID == "default" {
		if len(sources) > 0 {
			return sources[0].BaseURL, nil
		}
		return "", nil
	}
	for _, s := range sources {
		if strconv.FormatInt(s.ID, 10) == sourceID {
			return s.BaseURL, nil
		}
	}
	return "", fmt.Errorf("社区源 %s 不可用", sourceID)
}

// ListPage 分页浏览市场（服务端 GET /api/v1/packages 唯一数据路径）；
// API 不可达直接报错——静态清单机制已废除，不再有任何回退。
func (m *MarketClient) ListPage(q MarketQuery) (*MarketPage, error) {
	return m.fetchAPIPage(q)
}

// apiPackageMeta 服务端包条目的嵌套元数据（meta 对象，列表/详情共用结构）。
type apiPackageMeta struct {
	Icon        string            `json:"icon"`
	Title       string            `json:"title"`
	NickName    string            `json:"nick_name"`
	Role        string            `json:"role"`
	Leader      string            `json:"leader"`
	Members     []string          `json:"members"`
	Skills      []string          `json:"skills"`
	SkillNames  map[string]string `json:"skill_names"`
	SkillDescs  map[string]string `json:"skill_descs"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	URL         string            `json:"url"`
	Repo        string            `json:"repo"`
	License     string            `json:"license"`
	Permissions []string          `json:"permissions"`
	Fingerprint string            `json:"fingerprint"`
}

// fetchAPIPage 走服务端分页 API（响应条目为顶层基础字段 + meta 嵌套元数据）。
func (m *MarketClient) fetchAPIPage(q MarketQuery) (*MarketPage, error) {
	return m.fetchPageAt(m.apiBase+"/packages", q)
}

// ListOrgPackages 浏览组织源货架（M7）：仅本人在册组织可见，响应形态与公共货架
// 一致；download_url 由服务端下发为组织下载端点（Download 无需感知组织差异）。
func (m *MarketClient) ListOrgPackages(orgSlug string, q MarketQuery) (*MarketPage, error) {
	return m.fetchPageAt(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/packages", q)
}

// fetchPageAt 分页浏览共用实现（公共货架与组织源仅 URL 前缀不同）。
func (m *MarketClient) fetchPageAt(prefix string, q MarketQuery) (*MarketPage, error) {
	params := url.Values{}
	params.Set("page", strconv.Itoa(maxInt(q.Page, 1)))
	if q.PageSize > 0 {
		params.Set("page_size", strconv.Itoa(q.PageSize))
	}
	for k, v := range map[string]string{"kind": q.Kind, "q": q.Q, "sort": q.Sort} {
		if v != "" {
			params.Set(k, v)
		}
	}
	resp, err := m.authedGet(prefix + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("分页 API 响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	var body struct {
		Total    int          `json:"total"`
		Page     int          `json:"page"`
		Packages []apiPackage `json:"packages"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析分页响应失败：%w", err)
	}
	packages := make([]MarketPackage, 0, len(body.Packages))
	for _, ap := range body.Packages {
		packages = append(packages, ap.toMarketPackage())
	}
	normalizePackages(packages)
	return &MarketPage{Total: body.Total, Page: body.Page, Packages: packages, Source: "api"}, nil
}

// PackageByKindName 从服务端查单包详情（安装/预览的定位路径）；
// API 不可达直接报错——静态清单机制已废除，不再有全量清单线性定位回退。
func (m *MarketClient) PackageByKindName(kind, name string) (*MarketPackage, error) {
	return m.packageAt(m.apiBase+"/packages/"+url.PathEscape(kind)+"/"+url.PathEscape(name), kind, name)
}

// OrgPackageByKindName 从组织源查单包详情（M7）：仅命中本人在册组织的包；
// 包不存在或归属他组织服务端一律 404（不泄露跨组织信息）。
func (m *MarketClient) OrgPackageByKindName(orgSlug, kind, name string) (*MarketPackage, error) {
	return m.packageAt(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/packages/"+url.PathEscape(kind)+"/"+url.PathEscape(name), kind, name)
}

// packageAt 单包详情共用实现（公共货架与组织源仅 URL 不同）。
func (m *MarketClient) packageAt(u, kind, name string) (*MarketPackage, error) {
	resp, err := m.authedGet(u)
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("社区中不存在 %s 类型的包 %q", kind, name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("包详情 API 响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	return packageFromDetail(data)
}

// packageFromDetail 从服务端单包详情响应组装 MarketPackage
// （基础字段 + meta 与列表条目同构；sha256/size 取最新未删除版本）。
func packageFromDetail(data []byte) (*MarketPackage, error) {
	var body struct {
		apiPackage
		Versions []struct {
			Version     string `json:"version"`
			Sha256      string `json:"sha256"`
			Size        int64  `json:"size"`
			Deleted     bool   `json:"deleted"`
			PublishedAt string `json:"published_at"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析包详情失败：%w", err)
	}
	pkg := body.toMarketPackage()
	for _, v := range body.Versions {
		if !v.Deleted {
			// 最新未删除版本：与安装校验链对齐
			pkg.Version, pkg.Sha256, pkg.Size = v.Version, v.Sha256, v.Size
			break
		}
	}
	return &pkg, nil
}

// ─── 组织源（M7 组织化协作最后一环）───

// MarketOrg 我的组织条目（GET /api/v1/me/orgs 响应行；发布弹窗目标选择数据源）。
type MarketOrg struct {
	OrgID    int64  `json:"org_id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	LogoURL  string `json:"logo_url"`
	Role     string `json:"org_role"` // owner / admin / member
	Status   string `json:"status"`   // active / frozen（冻结组织客户端置灰）
	JoinedAt string `json:"joined_at"`
}

// MeOrgs 拉取本人在册的全部组织（登录态必需；从未加入组织返回空列表）。
// 冻结组织同样返回（带 status），由界面置灰；向冻结组织发布服务端会拒绝。
func (m *MarketClient) MeOrgs() ([]MarketOrg, error) {
	if m.authToken == "" {
		return nil, fmt.Errorf("获取组织列表前请先绑定 MindX 账号")
	}
	resp, err := m.authedGet(m.apiBase + "/me/orgs")
	if err != nil {
		return nil, fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("登录状态已失效，请重新绑定账号")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("组织列表 API 响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return nil, err
	}
	var body struct {
		Orgs []MarketOrg `json:"orgs"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析组织列表失败：%w", err)
	}
	return body.Orgs, nil
}

// categoryAliasZh 分类归一表：非规范写法（发布目录 slug / 旧译中文）→ 规范中文显示名
// （与服务端 categories 种子同一词汇表）。部分历史包的清单 category 直填了发布目录
// slug（线上 20 个 3c 类包），finance 旧译「商业研究」亦随正名归一；规范中文原样通过。
var categoryAliasZh = map[string]string{
	"dev":        "产品研发",
	"3c":         "智能硬件",
	"data":       "数据分析",
	"finance":    "财务",
	"office":     "办公提效",
	"marketing":  "市场营销",
	"contents":   "内容创作",
	"management": "经营管理",
	"industry":   "工业",
	"商业研究":       "财务", // finance 旧译
}

// normalizeCategory 将非规范分类写法归一为规范中文显示名。
func normalizeCategory(category string) string {
	if zh, ok := categoryAliasZh[strings.TrimSpace(category)]; ok {
		return zh
	}
	return category
}

// normalizePackages 批量归一包分类（API 解析后调用，展示口径统一）。
func normalizePackages(packages []MarketPackage) {
	for i := range packages {
		packages[i].Category = normalizeCategory(packages[i].Category)
	}
}

// maxInt 返回较大值。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cachePackagePath 返回包文件的本地缓存路径（File 为内容寻址相对路径，拒绝穿越）。
func (m *MarketClient) cachePackagePath(file string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(file))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("包文件缓存路径非法：%q", file)
	}
	return filepath.Join(m.cacheDir, clean), nil
}

// Download 下载分发包到缓存目录并做 sha256 完整性校验，返回本地文件路径。
// 缓存中已有校验通过的副本时直接复用，不重复下载。
// 下载地址取服务端 API 下发的 DownloadURL（清单拼接机制已废除）。
func (m *MarketClient) Download(pkg MarketPackage) (string, error) {
	if pkg.DownloadURL == "" {
		return "", fmt.Errorf("分发包 %s/%s 缺少下载地址（服务端未下发 download_url）", pkg.Kind, pkg.Name)
	}
	if len(pkg.Sha256) != 64 {
		return "", fmt.Errorf("分发包 %s/%s 的 sha256 缺失或格式非法", pkg.Kind, pkg.Name)
	}
	// 内容寻址缓存布局：packages/<kind>/<name>.mindpkg（File 缺省时按此推导）
	file := pkg.File
	if file == "" {
		file = "packages/" + string(pkg.Kind) + "/" + pkg.Name + ".mindpkg"
	}
	dest, err := m.cachePackagePath(file)
	if err != nil {
		return "", err
	}

	// 缓存命中且校验通过：直接复用
	if ok, _ := verifyFileSha256(dest, pkg.Sha256); ok {
		return dest, nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("创建缓存目录失败：%w", err)
	}

	resp, err := m.authedGet(pkg.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("下载分发包失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载分发包失败：响应状态 %d", resp.StatusCode)
	}
	// 官网把未知路径 fallback 成 200 的 index.html（如服务端缺下载路由），
	// 提前识破给人话，避免落进大小比对报出不知所云的字节数
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/html") {
		return "", fmt.Errorf("服务端的安装包文件缺失，请稍后重试")
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败：%w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	size, err := io.Copy(tmp, io.LimitReader(resp.Body, packageMaxSize))
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("下载分发包失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("关闭临时文件失败：%w", err)
	}
	if pkg.Size > 0 && size != pkg.Size {
		return "", fmt.Errorf("分发包大小不符：期望 %d 字节，实际 %d 字节", pkg.Size, size)
	}

	// sha256 传输完整性校验，不通过即拒装（失败不落缓存）
	got, err := fileSha256(tmpName)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(got, pkg.Sha256) {
		return "", fmt.Errorf("分发包校验失败：sha256 不匹配（期望 %s，实际 %s），已拒绝安装", pkg.Sha256, got)
	}

	if err := os.Rename(tmpName, dest); err != nil {
		return "", fmt.Errorf("缓存分发包失败：%w", err)
	}
	return dest, nil
}

// verifyFileSha256 校验文件内容摘要是否匹配（文件不存在返回 false）。
func verifyFileSha256(path, want string) (bool, error) {
	got, err := fileSha256(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(got, want), nil
}

// fileSha256 计算文件内容的 sha256 十六进制摘要。
func fileSha256(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
