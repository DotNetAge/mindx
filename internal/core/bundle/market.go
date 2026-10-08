package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultMarketAPIBase 是市场服务端 API 的基址（市场消费唯一入口：分页货架、
// 单包详情、源列表均出自此处；静态清单机制已废除，不再有任何清单回退路径）。
const DefaultMarketAPIBase = "https://www.mindx.chat/api/v1"

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
	// 插件元数据（kind=plugin；安装确认弹窗的信任呈现依据）。
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
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
// 分发包下载带 sha256 校验。pat 非空时所有出站请求携带 Authorization（商户源鉴权）。
type MarketClient struct {
	httpClient *http.Client
	// apiBase 市场服务端 API 基址（默认官方；本地调试/自托管经配置覆盖）
	apiBase string
	// sourceID 商户源标识（空/default = 官方公开货架，请求不带 source 参数）
	sourceID string
	cacheDir string
	// pat 商户访问令牌（用户在网页签发的 PAT，经设置粘贴；空 = 匿名）
	pat string
}

// SetPAT 设置商户访问令牌（换令牌后由调用方重建客户端或直接复用本实例均可）。
func (m *MarketClient) SetPAT(pat string) {
	m.pat = strings.TrimSpace(pat)
}

// authedGet 带凭证的 GET（pat 非空时附 Authorization 头）。
func (m *MarketClient) authedGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if m.pat != "" {
		req.Header.Set("Authorization", "Bearer "+m.pat)
	}
	return m.httpClient.Do(req)
}

// NewMarketClient 创建市场客户端。
// apiBase 为市场服务端 API 基址（空 = 官方 DefaultMarketAPIBase；本地调试/自托管显式传入）；
// sourceID 为选中市场源 id（空/default = 官方公开货架）。
// cacheDir 为缓存根目录：内部按 apiBase+sourceID 哈希开子目录（market-cache/<sha8>），
// 多源/多基址切换时包缓存互不串扰。
func NewMarketClient(apiBase, sourceID, cacheDir string) *MarketClient {
	apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if apiBase == "" {
		apiBase = DefaultMarketAPIBase
	}
	sum := sha256.Sum256([]byte(apiBase + "|" + sourceID))
	cacheDir = filepath.Join(cacheDir, hex.EncodeToString(sum[:])[:8])
	return &MarketClient{
		httpClient: &http.Client{Timeout: httpTimeout},
		apiBase:    apiBase,
		sourceID:   sourceID,
		cacheDir:   cacheDir,
	}
}

// apiPackage 服务端包条目的原始响应结构（列表/详情一致）：基础字段在顶层，
// 展示元数据（昵称/负责人/成员/技能装配）在嵌套 meta 对象中。
type apiPackage struct {
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	DownloadURL string         `json:"download_url"`
	Sha256      string         `json:"sha256"`
	Size        int64          `json:"size"`
	Version     string         `json:"version"`
	Meta        apiPackageMeta `json:"meta"`
}

// toMarketPackage 转为统一包条目（分类归一；版本优先取 meta.version，回退顶层）。
func (ap apiPackage) toMarketPackage() MarketPackage {
	version := ap.Meta.Version
	if version == "" {
		version = ap.Version
	}
	return MarketPackage{
		Kind: PackageKind(ap.Kind), Name: ap.Name, Description: ap.Description,
		Icon: ap.Meta.Icon, NickName: ap.Meta.NickName, Role: ap.Meta.Role,
		Category: normalizeCategory(ap.Category), Leader: ap.Meta.Leader, Members: ap.Meta.Members,
		Skills: ap.Meta.Skills, SkillNames: ap.Meta.SkillNames, SkillDescs: ap.Meta.SkillDescs,
		Version: version, DownloadURL: ap.DownloadURL, Sha256: ap.Sha256, Size: ap.Size,
		Author: ap.Meta.Author, License: ap.Meta.License, Permissions: ap.Meta.Permissions,
	}
}

// MarketSource 市场源条目（服务端 GET /api/v1/sources 下发）。
type MarketSource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"`
}

// Sources 拉取可用市场源清单（商户用户携带 PAT 可见商户源；default 为空 id 语义）。
func (m *MarketClient) Sources() ([]MarketSource, error) {
	resp, err := m.authedGet(m.apiBase + "/sources")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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

// ResolveSource 校验源 id 有效性：空或 "default" 视为官方公开源直接通过；
// 其余 id 必须出现在源列表中（源列表拉取失败或 id 不在列表时报错，调用方回退官方源）。
func (m *MarketClient) ResolveSource(sourceID string) error {
	if strings.TrimSpace(sourceID) == "" || sourceID == "default" {
		return nil
	}
	sources, err := m.Sources()
	if err != nil {
		return err
	}
	for _, s := range sources {
		if strconv.FormatInt(s.ID, 10) == sourceID {
			return nil
		}
	}
	return fmt.Errorf("市场源 %s 不存在或不可用", sourceID)
}

// ListPage 分页浏览市场（服务端 GET /api/v1/packages 唯一数据路径）；
// API 不可达直接报错——静态清单机制已废除，不再有任何回退。
func (m *MarketClient) ListPage(q MarketQuery) (*MarketPage, error) {
	return m.fetchAPIPage(q)
}

// apiPackageMeta 服务端包条目的嵌套元数据（meta 对象，列表/详情共用结构）。
type apiPackageMeta struct {
	Icon        string            `json:"icon"`
	NickName    string            `json:"nick_name"`
	Role        string            `json:"role"`
	Leader      string            `json:"leader"`
	Members     []string          `json:"members"`
	Skills      []string          `json:"skills"`
	SkillNames  map[string]string `json:"skill_names"`
	SkillDescs  map[string]string `json:"skill_descs"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	Permissions []string          `json:"permissions"`
}

// fetchAPIPage 走服务端分页 API（响应条目为顶层基础字段 + meta 嵌套元数据）。
func (m *MarketClient) fetchAPIPage(q MarketQuery) (*MarketPage, error) {
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
	// 商户源货架：透传 source id（服务端按源的 registry 过滤）
	if m.sourceID != "" && m.sourceID != "default" {
		params.Set("source", m.sourceID)
	}
	resp, err := m.authedGet(m.apiBase + "/packages?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("市场服务不可达：%w", err)
	}
	defer resp.Body.Close()
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
	resp, err := m.authedGet(m.apiBase + "/packages/" + url.PathEscape(kind) + "/" + url.PathEscape(name))
	if err != nil {
		return nil, fmt.Errorf("市场服务不可达：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("市场中不存在 %s 类型包 %q", kind, name)
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
	pkg := body.apiPackage.toMarketPackage()
	for _, v := range body.Versions {
		if !v.Deleted {
			// 最新未删除版本：与安装校验链对齐
			pkg.Version, pkg.Sha256, pkg.Size = v.Version, v.Sha256, v.Size
			break
		}
	}
	return &pkg, nil
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载分发包失败：响应状态 %d", resp.StatusCode)
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
	defer in.Close()
	h := sha256.New()
	if _, err := io.Copy(h, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
