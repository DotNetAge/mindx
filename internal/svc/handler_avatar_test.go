package svc

// 头像库 RPC 的行为断言：清单自然排序、缩略前缀、原图取用、路径穿越拒绝。
// 目录走 DefaultUserPrefsDir()（读 HOME），测试用 t.Setenv 指向临时目录。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestJPEG(t *testing.T, path string, size int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("编码测试图失败: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("写入测试图失败: %v", err)
	}
}

func prepareAvatarHome(t *testing.T, ids ...string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, group := range []string{"avatars", "teams"} {
		dir := filepath.Join(home, ".mindx", "assets", group)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("创建头像目录失败: %v", err)
		}
		for i, id := range ids {
			writeTestJPEG(t, filepath.Join(dir, id+".jpg"), 256, color.RGBA{R: uint8(20 * i), G: 100, B: 150, A: 255})
		}
	}
}

func TestAvatarListAndGet(t *testing.T) {
	prepareAvatarHome(t, "10", "2", "1")

	d := &Daemon{}
	got, err := d.handleAvatarList(context.TODO(), json.RawMessage(`{"group":"agent"}`))
	if err != nil {
		t.Fatalf("avatar.list 失败: %v", err)
	}
	res := got.(avatarListResult)
	if len(res.Items) != 3 {
		t.Fatalf("期望 3 项，得到 %d", len(res.Items))
	}
	// 自然排序：1, 2, 10
	if res.Items[0].ID != "1" || res.Items[1].ID != "2" || res.Items[2].ID != "10" {
		t.Fatalf("排序错误: %v", []string{res.Items[0].ID, res.Items[1].ID, res.Items[2].ID})
	}
	for _, it := range res.Items {
		if it.Group != "agent" || !strings.HasPrefix(it.Thumb, avatarDataPrefix) {
			t.Fatalf("条目字段错误: %+v", it)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(it.Thumb, avatarDataPrefix))
		if err != nil {
			t.Fatalf("缩略图 base64 解码失败: %v", err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("缩略图不是 JPEG: %v", err)
		}
		if cfg.Width != avatarThumbSize || cfg.Height != avatarThumbSize {
			t.Fatalf("缩略尺寸应为 %d，实际 %dx%d", avatarThumbSize, cfg.Width, cfg.Height)
		}
	}

	gotGet, err := d.handleAvatarGet(context.TODO(), json.RawMessage(`{"group":"agent","id":"2"}`))
	if err != nil {
		t.Fatalf("avatar.get 失败: %v", err)
	}
	g := gotGet.(avatarGetResult)
	if g.Group != "agent" || g.ID != "2" {
		t.Fatalf("返回身份错误: %+v", g)
	}
	if !strings.HasPrefix(g.Data, avatarDataPrefix) {
		t.Fatalf("原图不是 data URI")
	}
}

func TestAvatarGetRejectsTraversal(t *testing.T) {
	prepareAvatarHome(t, "1")
	d := &Daemon{}
	if _, err := d.handleAvatarGet(context.TODO(), json.RawMessage(`{"group":"agent","id":"../secret"}`)); err == nil {
		t.Fatalf("路径穿越 id 应被拒绝")
	}
	if _, err := d.handleAvatarGet(context.TODO(), json.RawMessage(`{"group":"bogus","id":"1"}`)); err == nil {
		t.Fatalf("未知分组应被拒绝")
	}
}

func TestAvatarListEmptyWhenAssetsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	d := &Daemon{}
	got, err := d.handleAvatarList(context.TODO(), json.RawMessage(`{"group":"team"}`))
	if err != nil {
		t.Fatalf("目录缺失时应返回空清单而非错误: %v", err)
	}
	if res := got.(avatarListResult); len(res.Items) != 0 {
		t.Fatalf("期望空清单，得到 %d 项", len(res.Items))
	}
}
