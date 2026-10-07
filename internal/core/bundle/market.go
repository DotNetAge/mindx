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

// DefaultMarketManifestURL 是 COS 静态市场的清单地址（bucket repo-1257961037，
// 与 mindx-app 发布桶同账号，公有读私有写；发布走 publish-market.mjs 脚本）。
// 过渡兼容：静态清单是文件目录时代的形态，新消费路径走 DefaultMarketAPIBase 分页 API。
const DefaultMarketManifestURL = "https://repo-1257961037.cos.ap-guangzhou.myqcloud.com/market/manifest.json"

// DefaultMarketAPIBase 是市场服务端分页 API 的基址（市场消费主入口，契约 §2.2）。
const DefaultMarketAPIBase = "https://www.mindx.chat/api/v1"

// 市场交互的体积与超时上限。
const (
	manifestMaxSize = 10 << 20 // 清单上限 10MB
	packageMaxSize  = 200 << 20
	httpTimeout     = 15 * time.Second
)

// MarketManifest 是市场清单（与包内 Manifest 不同：这是"货架"，描述可安装的分发包）。
// sha256 仅用于传输完整性校验，不构成版本管理（与"Skill 暂无版本管理策略"一致）。
type MarketManifest struct {
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Packages  []MarketPackage `json:"packages"`
}

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
	// Version 包版本（kind=skill 取自 SKILL.md frontmatter metadata.version，打包时写入清单）。
	Version string `json:"version,omitempty"`
	// File 包文件相对清单地址的路径（如 packages/xxx.mindpkg；API 形态下可为空）。
	File string `json:"file"`
	// DownloadURL 包文件下载直连地址（服务端分页 API 下发，安装直连无需清单拼接）。
	DownloadURL string `json:"download_url,omitempty"`
	// Sha256 包文件内容的十六进制摘要。
	Sha256 string `json:"sha256"`
	// Size 包文件字节数（列表展示用）。
	Size int64 `json:"size,omitempty"`
}

// MarketListResult 是一次清单拉取的结果。
type MarketListResult struct {
	Manifest *MarketManifest `json:"manifest"`
	// Source 数据来源：remote（在线清单）/ cache（降级本地缓存）。
	Source string `json:"source"`
	// Warning 降级原因（Source=cache 时非空），供前端提示。
	Warning string `json:"warning,omitempty"`
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
	// Source 数据来源：api（分页 API）/ manifest（回退全量清单过滤）。
	Source  string `json:"source"`
	Warning string `json:"warning,omitempty"`
}

// MarketClient 消费市场：分页 API 优先（服务端主入口），静态清单回退（过渡兼容）；
// 分发包下载带 sha256 校验。
type MarketClient struct {
	httpClient  *http.Client
	manifestURL string
	apiBase     string
	cacheDir    string
}

// NewMarketClient 创建市场客户端。
// manifestURL 为空时使用 DefaultMarketManifestURL；cacheDir 为清单与包文件的本地缓存目录。
// apiBase 由 manifestURL 同源推导（清单目录 + /api/v1）：服务端清单形如
// https://www.mindx.chat/manifest → https://www.mindx.chat/api/v1；COS/测试地址推导出的
// API 路径不存在时由调用方快速回退清单路径，不产生长等待。
func NewMarketClient(manifestURL, cacheDir string) *MarketClient {
	if strings.TrimSpace(manifestURL) == "" {
		manifestURL = DefaultMarketManifestURL
	}
	apiBase := DefaultMarketAPIBase
	if i := strings.LastIndex(manifestURL, "/"); i >= 0 {
		apiBase = manifestURL[:i+1] + "api/v1"
	}
	return &MarketClient{
		httpClient:  &http.Client{Timeout: httpTimeout},
		manifestURL: manifestURL,
		apiBase:     apiBase,
		cacheDir:    cacheDir,
	}
}

// ListPage 分页浏览市场：优先分页 API（市场消费主入口）；API 不可达时回退
// 全量清单（COS 过渡兼容）在内存中过滤分页，并在结果上标注来源与降级原因。
func (m *MarketClient) ListPage(q MarketQuery) (*MarketPage, error) {
	if page, err := m.fetchAPIPage(q); err == nil {
		return page, nil
	} else if q.Q == "" && q.Kind == "" && q.Sort == "" && (q.Page <= 1) {
		// 未带过滤条件且要第一页：旧全量清单可直接兜底（保持存量语义）
		res, listErr := m.List()
		if listErr != nil {
			return nil, fmt.Errorf("分页 API 与清单均不可达：%v；%v", err, listErr)
		}
		warning := "市场分页 API 不可达，已回退静态清单（过渡兼容）"
		if res.Warning != "" {
			warning = res.Warning
		}
		return &MarketPage{
			Total: len(res.Manifest.Packages), Page: 1,
			Packages: res.Manifest.Packages, Source: "manifest", Warning: warning,
		}, nil
	} else {
		// 带过滤条件的请求：拉全量清单在内存过滤分页（过渡兼容，服务端上线后自然消失）
		res, listErr := m.List()
		if listErr != nil {
			return nil, fmt.Errorf("分页 API 与清单均不可达：%v；%v", err, listErr)
		}
		filtered := filterPackages(res.Manifest.Packages, q)
		page, pageSize := normalizePage(q)
		start := (page - 1) * pageSize
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + pageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		return &MarketPage{
			Total: len(filtered), Page: page,
			Packages: filtered[start:end], Source: "manifest",
			Warning: "市场分页 API 不可达，已回退静态清单过滤（过渡兼容）",
		}, nil
	}
}

// fetchAPIPage 走服务端分页 API。
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
	resp, err := m.httpClient.Get(m.apiBase + "/packages?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("分页 API 响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, manifestMaxSize))
	if err != nil {
		return nil, err
	}
	var body struct {
		Total    int             `json:"total"`
		Page     int             `json:"page"`
		Packages []MarketPackage `json:"packages"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("解析分页响应失败：%w", err)
	}
	return &MarketPage{Total: body.Total, Page: body.Page, Packages: body.Packages, Source: "api"}, nil
}

// PackageByKindName 从服务端查单包详情（安装/预览的定位路径，替代「拉全量清单线性查找」）。
// API 不可达时回退全量清单定位（过渡兼容）。
func (m *MarketClient) PackageByKindName(kind, name string) (*MarketPackage, error) {
	resp, err := m.httpClient.Get(m.apiBase + "/packages/" + kind + "/" + name)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, manifestMaxSize))
			if readErr != nil {
				return nil, readErr
			}
			return packageFromDetail(data)
		}
	}
	// 回退：全量清单线性定位（COS 过渡兼容）
	res, listErr := m.List()
	if listErr != nil {
		return nil, fmt.Errorf("市场 API 与清单均不可达：%v；%v", err, listErr)
	}
	for i := range res.Manifest.Packages {
		p := &res.Manifest.Packages[i]
		if string(p.Kind) == kind && p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("市场中不存在 %s 类型包 %q", kind, name)
}

// packageFromDetail 从服务端单包详情响应组装 MarketPackage。
func packageFromDetail(data []byte) (*MarketPackage, error) {
	var body struct {
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		DownloadURL string `json:"download_url"`
		Meta        struct {
			Icon       string            `json:"icon"`
			NickName   string            `json:"nick_name"`
			Role       string            `json:"role"`
			Leader     string            `json:"leader"`
			Members    []string          `json:"members"`
			Skills     []string          `json:"skills"`
			SkillNames map[string]string `json:"skill_names"`
			SkillDescs map[string]string `json:"skill_descs"`
			Version    string            `json:"version"`
		} `json:"meta"`
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
	pkg := &MarketPackage{
		Kind: PackageKind(body.Kind), Name: body.Name, Description: body.Description,
		Icon: body.Meta.Icon, NickName: body.Meta.NickName, Role: body.Meta.Role,
		Category: body.Category, Leader: body.Meta.Leader, Members: body.Meta.Members,
		Skills: body.Meta.Skills, SkillNames: body.Meta.SkillNames, SkillDescs: body.Meta.SkillDescs,
		DownloadURL: body.DownloadURL,
	}
	for _, v := range body.Versions {
		if !v.Deleted {
			// 最新未删除版本：与安装校验链对齐
			pkg.Version, pkg.Sha256, pkg.Size = v.Version, v.Sha256, v.Size
			break
		}
	}
	return pkg, nil
}

// filterPackages 全量清单的内存过滤（回退路径专用）。
func filterPackages(packages []MarketPackage, q MarketQuery) []MarketPackage {
	out := make([]MarketPackage, 0, len(packages))
	needle := strings.ToLower(q.Q)
	for _, p := range packages {
		if q.Kind != "" && string(p.Kind) != q.Kind {
			continue
		}
		if needle != "" &&
			!strings.Contains(strings.ToLower(p.Name), needle) &&
			!strings.Contains(strings.ToLower(p.Description), needle) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// normalizePage 归一化分页参数。
func normalizePage(q MarketQuery) (page, pageSize int) {
	page = maxInt(q.Page, 1)
	pageSize = q.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	return page, pageSize
}

// maxInt 返回较大值。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// List 拉取市场清单：优先在线，网络失败降级本地缓存并附降级原因；
// 在线与缓存均不可用时返回错误（不静默吞掉）。
func (m *MarketClient) List() (*MarketListResult, error) {
	manifest, err := m.fetchRemote()
	if err == nil {
		// 在线成功即刷新缓存；写失败仅影响下次降级质量，不影响本次结果
		_ = m.writeCache(manifest)
		return &MarketListResult{Manifest: manifest, Source: "remote"}, nil
	}

	remoteErr := err
	cached, cacheErr := m.readCache()
	if cacheErr == nil {
		return &MarketListResult{
			Manifest: cached,
			Source:   "cache",
			Warning:  fmt.Sprintf("在线清单拉取失败，已展示本地缓存（%v）", remoteErr),
		}, nil
	}
	return nil, fmt.Errorf("在线清单拉取失败且本地缓存不可用：%v；缓存读取错误：%v", remoteErr, cacheErr)
}

// fetchRemote 在线拉取清单。
func (m *MarketClient) fetchRemote() (*MarketManifest, error) {
	resp, err := m.httpClient.Get(m.manifestURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("清单响应状态 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, manifestMaxSize))
	if err != nil {
		return nil, err
	}
	var manifest MarketManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析清单失败：%w", err)
	}
	return &manifest, nil
}

// writeCache 将清单写入本地缓存（原子替换）。
func (m *MarketClient) writeCache(manifest *MarketManifest) error {
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	tmp := filepath.Join(m.cacheDir, ".manifest.json.tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, m.cacheManifestPath())
}

// readCache 读取本地缓存的清单。
func (m *MarketClient) readCache() (*MarketManifest, error) {
	data, err := os.ReadFile(m.cacheManifestPath())
	if err != nil {
		return nil, err
	}
	var manifest MarketManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("缓存清单损坏：%w", err)
	}
	return &manifest, nil
}

// cacheManifestPath 返回缓存清单路径。
func (m *MarketClient) cacheManifestPath() string {
	return filepath.Join(m.cacheDir, "manifest.json")
}

// cachePackagePath 返回包文件的本地缓存路径（File 为清单声明的相对路径）。
func (m *MarketClient) cachePackagePath(file string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(file))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("清单中的包文件路径非法：%q", file)
	}
	return filepath.Join(m.cacheDir, clean), nil
}

// packageURL 拼接包文件的下载地址（清单地址所在目录 + File 相对路径）。
func (m *MarketClient) packageURL(file string) string {
	base := m.manifestURL
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[:i+1]
	}
	return base + file
}

// Download 下载分发包到缓存目录并做 sha256 完整性校验，返回本地文件路径。
// 缓存中已有校验通过的副本时直接复用，不重复下载。
// 下载地址优先 DownloadURL（服务端 API 直连），回退清单 File 相对路径拼接（过渡兼容）。
func (m *MarketClient) Download(pkg MarketPackage) (string, error) {
	if pkg.DownloadURL == "" && pkg.File == "" {
		return "", fmt.Errorf("分发包条目缺少下载地址（download_url / file 均为空）")
	}
	if len(pkg.Sha256) != 64 {
		return "", fmt.Errorf("分发包 %s/%s 的 sha256 缺失或格式非法", pkg.Kind, pkg.Name)
	}
	file := pkg.File
	if file == "" {
		// API 形态：按服务端内容寻址布局推导缓存路径（与 file 常规值一致）
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

	downloadURL := pkg.DownloadURL
	if downloadURL == "" {
		downloadURL = m.packageURL(pkg.File)
	}
	resp, err := m.httpClient.Get(downloadURL)
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
