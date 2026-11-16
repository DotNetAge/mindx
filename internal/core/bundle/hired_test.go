package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripHiredField(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "删除 true 行，其余原样",
			in:   "---\nname: a\nhired: true\nrole: R\n---\n正文\n",
			want: "---\nname: a\nrole: R\n---\n正文\n",
		},
		{
			name: "删除 false 行（值无关，字段本身不该进包）",
			in:   "---\nname: a\nhired: false\n---\n",
			want: "---\nname: a\n---\n",
		},
		{
			name: "无该字段不动",
			in:   "---\nname: a\n---\n正文\n",
			want: "---\nname: a\n---\n正文\n",
		},
		{
			name: "无 frontmatter 不动（正文里的 hired 行不是字段）",
			in:   "正文\nhired: true\n",
			want: "正文\nhired: true\n",
		},
		{
			name: "只处理 frontmatter 区，正文中的同名行保留",
			in:   "---\nhired: true\n---\nhired: true 这句是正文\n",
			want: "---\n---\nhired: true 这句是正文\n",
		},
		{
			name: "CRLF 保留",
			in:   "---\r\nname: a\r\nhired: true\r\n---\r\n正文\r\n",
			want: "---\r\nname: a\r\n---\r\n正文\r\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(stripHiredField([]byte(c.in))); got != c.want {
				t.Fatalf("stripHiredField:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestFrontmatterHired(t *testing.T) {
	if !frontmatterHired([]byte("---\nhired: true\n---\n")) {
		t.Fatal("应读到 true")
	}
	if frontmatterHired([]byte("---\nhired: false\n---\n")) {
		t.Fatal("false 不应读成 true")
	}
	if frontmatterHired([]byte("---\nname: a\n---\n")) {
		t.Fatal("缺省应为 false")
	}
	if frontmatterHired([]byte("正文\nhired: true\n")) {
		t.Fatal("无 frontmatter 应为 false")
	}
}

func TestWithHiredTrue(t *testing.T) {
	if got := string(withHiredTrue([]byte("---\nname: a\n---\n"))); got != "---\nname: a\nhired: true\n---\n" {
		t.Fatalf("插入失败：%q", got)
	}
	if got := string(withHiredTrue([]byte("---\nhired: false\nname: a\n---\n"))); got != "---\nhired: true\nname: a\n---\n" {
		t.Fatalf("改写失败：%q", got)
	}
	// 无 frontmatter：补块，正文保持完整
	if got := string(withHiredTrue([]byte("正文"))); got != "---\nhired: true\n---\n正文" {
		t.Fatalf("补 frontmatter 失败：%q", got)
	}
}

// TestExportStripsHired 发布/导出打包必须剥掉 hired：包是交付物，不该带安装者状态。
func TestExportStripsHired(t *testing.T) {
	_, agents, skills := newTestStores(t)
	mustSaveAgent(t, agents, "alpha", "身份正文", nil)
	if err := agents.SetHired("alpha", true); err != nil {
		t.Fatalf("置雇佣失败: %v", err)
	}
	agent := agents.Get("alpha")
	if agent == nil {
		t.Fatal("测试 Agent 未加载")
	}

	data, _, _, err := BuildAgentBytes(agent, skills, &PublishOverrides{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("打包失败: %v", err)
	}
	pkgPath := filepath.Join(t.TempDir(), "alpha.mindpkg")
	if err := os.WriteFile(pkgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readPackageEntries(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	var packed string
	for _, e := range entries {
		if e.Name == zipPrefixAgent+identityFileName {
			packed = string(e.Content)
		}
	}
	if packed == "" {
		t.Fatal("包内缺少 IDENTITY.md")
	}
	if frontmatterHired([]byte(packed)) {
		t.Fatalf("包内 IDENTITY.md 仍带 hired：%q", packed)
	}
	if !strings.Contains(packed, "alpha") {
		t.Fatalf("剥字段不应损坏其余元数据：%q", packed)
	}
}

// TestInstallIgnoresPackageHired 安装时包里的 hired 不生效（雇佣是本机决定）。
func TestInstallIgnoresPackageHired(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "beta.mindpkg")
	manifest := &Manifest{Format: PackageFormat, Kind: KindAgent, Name: "beta", Version: "1.0.0"}
	if err := buildZip(pkg, map[string][]byte{
		zipPathManifest:                   mustManifestJSON(t, manifest),
		zipPrefixAgent + identityFileName: []byte("---\nname: beta\nrole: 测试\nhired: true\n---\n正文\n"),
	}); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(root, "agents")
	installed, err := Install(pkg, InstallOptions{AgentsDir: agentsDir, GlobalDir: filepath.Join(root, "skills")})
	if err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	if installed.Name != "beta" {
		t.Fatalf("installed = %+v", installed)
	}
	data, err := os.ReadFile(filepath.Join(agentsDir, "beta", identityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if frontmatterHired(data) {
		t.Fatalf("新装 Agent 不应被包里的 hired 带成已雇佣：%q", string(data))
	}
}

// TestInstallPreservesLocalHired 覆盖安装（升级）必须保住本地雇佣态。
func TestInstallPreservesLocalHired(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, "agents")
	agentDir := filepath.Join(agentsDir, "gamma")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 本地已雇佣的旧版本
	if err := os.WriteFile(
		filepath.Join(agentDir, identityFileName),
		[]byte("---\nname: gamma\nrole: 旧\nhired: true\n---\n旧正文\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "gamma.mindpkg")
	manifest := &Manifest{Format: PackageFormat, Kind: KindAgent, Name: "gamma", Version: "2.0.0"}
	if err := buildZip(pkg, map[string][]byte{
		zipPathManifest:                   mustManifestJSON(t, manifest),
		zipPrefixAgent + identityFileName: []byte("---\nname: gamma\nrole: 新\n---\n新正文\n"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(pkg, InstallOptions{
		AgentsDir: agentsDir,
		GlobalDir: filepath.Join(root, "skills"),
		Overwrite: true,
	}); err != nil {
		t.Fatalf("覆盖安装失败: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(agentDir, identityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !frontmatterHired(data) {
		t.Fatalf("升级后应保留已雇佣：%q", string(data))
	}
	if !strings.Contains(string(data), "role: 新") {
		t.Fatalf("升级应替换为包内元数据：%q", string(data))
	}
}

// mustManifestJSON 构造包内 manifest.json 字节。
func mustManifestJSON(t *testing.T, m *Manifest) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("清单序列化失败: %v", err)
	}
	return b
}
