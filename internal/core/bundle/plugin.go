package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// 插件包（kind=plugin）与 agent/skill/team 的差异：
//   - 内层清单不是本包的 Manifest，而是 mindx-work 的插件 manifest.json
//     （身份 = 反向域 id + semver version + author，三键指纹），本文件不解释它；
//   - 已装产物在机器级根 ~/.mindx/plugins/<id>/<version>/，与 mindx-work 的
//     plugin-store 同一约定（App 主进程与 mw CLI 都写这里）；
//   - 发布新版本 = 取激活代际目录整包，改写 manifest 里的 version 并重算指纹。

// PluginPackRoot 已装插件的机器级根目录。
func PluginPackRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".mindx", "plugins")
}

// pluginFingerprint 与 mindx-work plugin-store / 服务端 manifestbuild.PluginFingerprint
// 逐字节一致：三键按字母序（encoding/json 对 map 排键）紧凑 JSON 的 sha256。
func pluginFingerprint(id, author, version string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // 与服务端 manifestbuild.PluginFingerprint 同参数：不转义 HTML 字符
	_ = enc.Encode(map[string]string{"author": author, "id": id, "version": version})
	sum := sha256.Sum256(bytes.TrimRight(buf.Bytes(), "\n"))
	return hex.EncodeToString(sum[:])
}

// PluginActiveVersion 取插件的生效版本：优先 installed.json 的 activeVersion，
// 缺失或指向不存在的目录时回退目录内最大 semver。
func PluginActiveVersion(id string) (string, error) {
	root := PluginPackRoot()
	if root == "" {
		return "", fmt.Errorf("无法定位用户主目录，读不到插件目录")
	}
	bookPath := filepath.Join(root, "installed.json")
	if data, err := os.ReadFile(bookPath); err == nil {
		var ledger struct {
			Plugins []struct {
				ID            string `json:"id"`
				ActiveVersion string `json:"activeVersion"`
			} `json:"plugins"`
		}
		if json.Unmarshal(data, &ledger) == nil {
			for _, p := range ledger.Plugins {
				if p.ID != id {
					continue
				}
				if p.ActiveVersion != "" {
					if _, statErr := os.Stat(filepath.Join(root, id, p.ActiveVersion, "manifest.json")); statErr == nil {
						return p.ActiveVersion, nil
					}
				}
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, id))
	if err != nil {
		return "", fmt.Errorf("插件 %q 未安装在本机（%s 下没有它的版本目录）", id, root)
	}
	best := ""
	for _, e := range entries {
		if e.IsDir() && comparePluginVersion(e.Name(), best) > 0 {
			best = e.Name()
		}
	}
	if best == "" {
		return "", fmt.Errorf("插件 %q 没有可用的安装代际", id)
	}
	return best, nil
}

// comparePluginVersion 比两段数字版本（a>b 返回 1）。非数字段按字典序兜底。
func comparePluginVersion(a, b string) int {
	if b == "" {
		return 1
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := "0", "0"
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		an, aerr := strconv.Atoi(av)
		bn, berr := strconv.Atoi(bv)
		if aerr == nil && berr == nil {
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
			continue
		}
		if c := strings.Compare(av, bv); c != 0 {
			return c
		}
	}
	return 0
}

// BuildPluginBytes 用已装产物打一个发布包：整包文件保持原相对结构，
// manifest.json 换成发布版本并重算指纹（版本变了指纹必然变，否则服务端拒绝）。
// 图标保持包内相对路径——服务端发布时统一内联成 data URI。
func BuildPluginBytes(id string, ov *PublishOverrides) ([]byte, *Manifest, []string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil, nil, fmt.Errorf("插件 id 不能为空")
	}
	root := PluginPackRoot()
	version, err := PluginActiveVersion(id)
	if err != nil {
		return nil, nil, nil, err
	}
	dir := filepath.Join(root, id, version)
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("读取插件清单失败：%w", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, nil, nil, fmt.Errorf("插件清单不是合法 JSON：%w", err)
	}
	publishVersion := strings.TrimSpace(ov.Version)
	if publishVersion == "" {
		publishVersion = version
	}
	author, _ := manifest["author"].(string)
	if strings.TrimSpace(author) == "" {
		return nil, nil, nil, fmt.Errorf("插件清单缺少 author，无法重算指纹")
	}
	manifest["version"] = publishVersion
	if lic := strings.TrimSpace(ov.License); lic != "" {
		manifest["license"] = lic
	}
	manifest["fingerprint"] = pluginFingerprint(id, author, publishVersion)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("重写插件清单失败：%w", err)
	}

	var warnings []string
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// 遍历整目录：先收集路径再按字典序写入，保证同内容包字节稳定（可复算 sha256）
	var files []string
	if err := filepath.Walk(dir, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("遍历插件目录失败：%w", err)
	}
	sort.Strings(files)
	for _, rel := range files {
		if rel == "manifest.json" {
			continue
		}
		content, rerr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if rerr != nil {
			return nil, nil, nil, fmt.Errorf("读取包内文件 %s 失败：%w", rel, rerr)
		}
		if err := zipBytesEntry(zw, rel, content); err != nil {
			return nil, nil, nil, err
		}
	}
	if err := zipBytesEntry(zw, "manifest.json", manifestBytes); err != nil {
		return nil, nil, nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, nil, nil, fmt.Errorf("收尾打包失败：%w", err)
	}
	if _, ok := manifest["icon"]; !ok {
		warnings = append(warnings, "插件清单没有 icon，社区卡片会回退首字母头像")
	}
	m := &Manifest{
		Format:  PackageFormat,
		Kind:    KindPlugin,
		Name:    id,
		Version: publishVersion,
	}
	if desc, ok := manifest["description"].(string); ok {
		m.Description = desc
	}
	if icon, ok := manifest["icon"].(string); ok {
		m.Icon = icon
	}
	if title, ok := manifest["title"].(string); ok {
		m.Role = title
	}
	return buf.Bytes(), m, warnings, nil
}

// PluginManifestVersion 读已装插件当前代际 manifest.json 里的 version（发布版本权威）。
// 读不到返回空串——是否放行由 ResolvePublishVersion 决定（会以表单版本兜底或报错）。
func PluginManifestVersion(id string) string {
	version, err := PluginActiveVersion(id)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(PluginPackRoot(), id, version, "manifest.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &manifest) != nil {
		return ""
	}
	return strings.TrimSpace(manifest.Version)
}
