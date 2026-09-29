package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultMarketManifestURL 是 COS 静态市场的清单地址（bucket repo-1257961037，
// 与 mindx-app 发布桶同账号，公有读私有写；发布走 publish-market.mjs 脚本）。
const DefaultMarketManifestURL = "https://repo-1257961037.cos.ap-guangzhou.myqcloud.com/market/manifest.json"

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
	// File 包文件相对清单地址的路径（如 packages/xxx.mindpkg）。
	File string `json:"file"`
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

// MarketClient 消费 COS 静态市场：清单拉取（失败降级本地缓存）、分发包下载（sha256 校验）。
type MarketClient struct {
	httpClient  *http.Client
	manifestURL string
	cacheDir    string
}

// NewMarketClient 创建市场客户端。
// manifestURL 为空时使用 DefaultMarketManifestURL；cacheDir 为清单与包文件的本地缓存目录。
func NewMarketClient(manifestURL, cacheDir string) *MarketClient {
	if strings.TrimSpace(manifestURL) == "" {
		manifestURL = DefaultMarketManifestURL
	}
	return &MarketClient{
		httpClient:  &http.Client{Timeout: httpTimeout},
		manifestURL: manifestURL,
		cacheDir:    cacheDir,
	}
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
func (m *MarketClient) Download(pkg MarketPackage) (string, error) {
	if pkg.File == "" {
		return "", fmt.Errorf("分发包条目缺少 file 字段")
	}
	if len(pkg.Sha256) != 64 {
		return "", fmt.Errorf("分发包 %s/%s 的 sha256 缺失或格式非法", pkg.Kind, pkg.Name)
	}
	dest, err := m.cachePackagePath(pkg.File)
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

	resp, err := m.httpClient.Get(m.packageURL(pkg.File))
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
