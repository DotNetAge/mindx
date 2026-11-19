package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 插件发布打包：版本重写、指纹重算、图标与入口保持包内相对路径。
func TestBuildPluginBytesRewritesVersionAndFingerprint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	id := "com.test.demo"
	dir := filepath.Join(home, ".mindx", "plugins", id, "1.0.0")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"id": id, "name": "Demo", "title": "演示", "version": "1.0.0",
		"description": "测试插件", "author": "Ray", "author_name": "ray",
		"icon": "assets/icon.svg", "entry": "entry.mjs",
		"permissions": []string{}, "mxApiVersion": "1",
		"fingerprint": pluginFingerprint(id, "Ray", "1.0.0"),
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entry.mjs"), []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "icon.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := `{"schemaVersion":1,"plugins":[{"id":"com.test.demo","name":"Demo","enabled":true,"activeVersion":"1.0.0","versions":[],"origin":"market"}]}`
	if err := os.WriteFile(filepath.Join(home, ".mindx", "plugins", "installed.json"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}

	data, m, warnings, err := BuildPluginBytes(id, &PublishOverrides{Version: "1.1.0", License: "MIT"})
	if err != nil {
		t.Fatalf("打包失败：%v", err)
	}
	if m.Version != "1.1.0" {
		t.Fatalf("清单版本未重写：%q", m.Version)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("产物不是合法 zip：%v", err)
	}
	found := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		found[f.Name] = buf.Bytes()
	}
	for _, want := range []string{"manifest.json", "entry.mjs", "assets/icon.svg"} {
		if _, ok := found[want]; !ok {
			t.Fatalf("包内缺少 %s", want)
		}
	}
	var inner map[string]any
	if err := json.Unmarshal(found["manifest.json"], &inner); err != nil {
		t.Fatalf("包内清单非法：%v", err)
	}
	if inner["version"] != "1.1.0" {
		t.Fatalf("包内 manifest version = %v，应为 1.1.0", inner["version"])
	}
	if inner["license"] != "MIT" {
		t.Fatalf("license 未写入：%v", inner["license"])
	}
	if inner["icon"] != "assets/icon.svg" {
		t.Fatalf("icon 应保持包内相对路径（服务端发布时内联），实际 %v", inner["icon"])
	}
	if inner["fingerprint"] != pluginFingerprint(id, "Ray", "1.1.0") {
		t.Fatalf("指纹未按新版本重算：%v", inner["fingerprint"])
	}
	if len(warnings) != 0 {
		t.Fatalf("不应有告警：%v", warnings)
	}
}

// 未安装的插件必须给出可读报错，而不是空包。
func TestBuildPluginBytesMissingPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, _, _, err := BuildPluginBytes("com.test.absent", &PublishOverrides{Version: "1.0.0"}); err == nil {
		t.Fatal("未安装插件应当报错")
	}
}
