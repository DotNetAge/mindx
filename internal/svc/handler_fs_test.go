package svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// fsTestDaemon 返回零值 Daemon：handler_fs.go 的各 handler 只依赖文件系统，
// 不触碰 Daemon 字段，无需重量级 newTestDaemon 夹具。
func fsTestDaemon() *Daemon { return &Daemon{} }

func fsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustParams(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return raw
}

func TestHandleFSList(t *testing.T) {
	d := fsTestDaemon()
	dir := fsFixture(t)

	result, err := d.handleFSList(context.Background(), mustParams(t, rpc.FSListParams{Path: dir}))
	if err != nil {
		t.Fatalf("handleFSList error = %v", err)
	}
	entries := result.([]FSEntry)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	// 排序：目录在前，文件在后
	if !entries[0].IsDir || entries[0].Name != "sub" {
		t.Errorf("entries[0] = %s isDir=%v, want sub (dir first)", entries[0].Name, entries[0].IsDir)
	}
	if entries[1].IsDir || entries[1].Name != "a.txt" || entries[1].Size != 5 {
		t.Errorf("entries[1] = %s isDir=%v size=%d", entries[1].Name, entries[1].IsDir, entries[1].Size)
	}
	if entries[1].Path != filepath.Join(dir, "a.txt") {
		t.Errorf("entry path = %q, want %q", entries[1].Path, filepath.Join(dir, "a.txt"))
	}
}

func TestHandleFSList_AutoCreateAndErrors(t *testing.T) {
	d := fsTestDaemon()

	// 不存在的目录自动创建
	missing := filepath.Join(t.TempDir(), "made", "up")
	if _, err := d.handleFSList(context.Background(), mustParams(t, rpc.FSListParams{Path: missing})); err != nil {
		t.Fatalf("auto-create list error = %v", err)
	}
	if fi, err := os.Stat(missing); err != nil || !fi.IsDir() {
		t.Errorf("expected auto-created dir at %s, err=%v", missing, err)
	}

	// 传文件路径 → not a directory
	file := filepath.Join(t.TempDir(), "f.txt")
	_ = os.WriteFile(file, []byte("x"), 0644)
	if _, err := d.handleFSList(context.Background(), mustParams(t, rpc.FSListParams{Path: file})); err == nil {
		t.Error("file path: expected error, got nil")
	}
}

func TestHandleFSList_DefaultHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	d := fsTestDaemon()

	// 空 path → defaultFSHome()（被隔离的 HOME）
	result, err := d.handleFSList(context.Background(), mustParams(t, rpc.FSListParams{}))
	if err != nil {
		t.Fatalf("handleFSList default home error = %v", err)
	}
	if len(result.([]FSEntry)) != 0 {
		t.Error("isolated home should be empty")
	}

	// handleFSHome 同样返回隔离后的 HOME
	homeResult, err := d.handleFSHome(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleFSHome error = %v", err)
	}
	if got := homeResult.(map[string]string)["path"]; got != home {
		t.Errorf("home = %q, want %q", got, home)
	}
	if got := defaultFSHome(); got != home {
		t.Errorf("defaultFSHome() = %q, want %q", got, home)
	}
}

func TestHandleFSRead(t *testing.T) {
	d := fsTestDaemon()
	dir := fsFixture(t)

	result, err := d.handleFSRead(context.Background(), mustParams(t, rpc.FSReadParams{Path: filepath.Join(dir, "a.txt")}))
	if err != nil {
		t.Fatalf("handleFSRead error = %v", err)
	}
	if got := result.(rpc.FSReadResult).Content; got != "hello" {
		t.Errorf("content = %q, want hello", got)
	}

	// 目录与不存在路径报错
	if _, err := d.handleFSRead(context.Background(), mustParams(t, rpc.FSReadParams{Path: dir})); err == nil {
		t.Error("directory path: expected error")
	}
	if _, err := d.handleFSRead(context.Background(), mustParams(t, rpc.FSReadParams{Path: filepath.Join(dir, "nope")})); err == nil {
		t.Error("missing path: expected error")
	}
}

func TestHandleFSReadBase64(t *testing.T) {
	d := fsTestDaemon()
	dir := t.TempDir()

	png := filepath.Join(dir, "img.png")
	raw := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	_ = os.WriteFile(png, raw, 0644)

	result, err := d.handleFSReadBase64(context.Background(), mustParams(t, rpc.FSReadBase64Params{Path: png}))
	if err != nil {
		t.Fatalf("handleFSReadBase64 error = %v", err)
	}
	res := result.(rpc.FSReadBase64Result)
	if res.Mime != "image/png" {
		t.Errorf("mime = %q, want image/png", res.Mime)
	}
	decoded, decErr := base64.StdEncoding.DecodeString(res.Content)
	if decErr != nil || string(decoded) != string(raw) {
		t.Errorf("base64 round-trip mismatch: %v", decErr)
	}

	// 未知扩展名 → octet-stream
	noext := filepath.Join(dir, "blob")
	_ = os.WriteFile(noext, raw, 0644)
	result, err = d.handleFSReadBase64(context.Background(), mustParams(t, rpc.FSReadBase64Params{Path: noext}))
	if err != nil {
		t.Fatalf("handleFSReadBase64 noext error = %v", err)
	}
	if got := result.(rpc.FSReadBase64Result).Mime; got != "application/octet-stream" {
		t.Errorf("mime = %q, want application/octet-stream", got)
	}

	if _, err := d.handleFSReadBase64(context.Background(), mustParams(t, rpc.FSReadBase64Params{Path: dir})); err == nil {
		t.Error("directory path: expected error")
	}
}

func TestHandleFSWrite(t *testing.T) {
	d := fsTestDaemon()
	dir := t.TempDir()

	// 父目录不存在也应自动创建
	dst := filepath.Join(dir, "nested", "out.txt")
	if _, err := d.handleFSWrite(context.Background(), mustParams(t, rpc.FSWriteParams{Path: dst, Content: "data"})); err != nil {
		t.Fatalf("handleFSWrite error = %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "data" {
		t.Errorf("written content = %q, want data", got)
	}
}

func TestHandleFSWriteBase64(t *testing.T) {
	d := fsTestDaemon()
	dir := t.TempDir()

	if _, err := d.handleFSWriteBase64(context.Background(), mustParams(t, rpc.FSWriteBase64Params{})); err == nil {
		t.Error("empty params: expected error")
	}
	if _, err := d.handleFSWriteBase64(context.Background(), mustParams(t, rpc.FSWriteBase64Params{
		Path: filepath.Join(dir, "x.bin"), Content: "!!!not-base64!!!",
	})); err == nil {
		t.Error("invalid base64: expected error")
	}
	if _, err := d.handleFSWriteBase64(context.Background(), mustParams(t, rpc.FSWriteBase64Params{
		Path: filepath.Join(dir, "x.bin"), Content: base64.StdEncoding.EncodeToString(nil),
	})); err == nil {
		t.Error("empty decoded content: expected error")
	}

	dst := filepath.Join(dir, "x.bin")
	payload := base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0xFF})
	result, err := d.handleFSWriteBase64(context.Background(), mustParams(t, rpc.FSWriteBase64Params{Path: dst, Content: payload}))
	if err != nil {
		t.Fatalf("handleFSWriteBase64 error = %v", err)
	}
	if result.(rpc.FSWriteBase64Result).Status != "ok" {
		t.Errorf("status = %v, want ok", result)
	}
	got, _ := os.ReadFile(dst)
	if len(got) != 3 {
		t.Errorf("written bytes = %d, want 3", len(got))
	}
}

func TestHandleFSMkdir(t *testing.T) {
	d := fsTestDaemon()
	dir := filepath.Join(t.TempDir(), "a", "b", "c")

	if _, err := d.handleFSMkdir(context.Background(), mustParams(t, rpc.FSMkdirParams{Path: dir, All: true})); err != nil {
		t.Fatalf("handleFSMkdir error = %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("expected created dir, err=%v", err)
	}
}

func TestHandleFSRm(t *testing.T) {
	d := fsTestDaemon()
	dir := fsFixture(t)

	// 删文件
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: filepath.Join(dir, "a.txt")})); err != nil {
		t.Fatalf("rm file error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}

	// 非空目录不带 force/recurse → 报错
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: filepath.Join(dir, "sub")})); err == nil {
		t.Error("non-empty dir without force: expected error")
	}
	// force → 连带删除
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: filepath.Join(dir, "sub"), Force: true})); err != nil {
		t.Fatalf("rm force error = %v", err)
	}

	// recurse 删树
	treeBase := t.TempDir()
	tree := filepath.Join(treeBase, "tree", "deep")
	_ = os.MkdirAll(tree, 0755)
	_ = os.WriteFile(filepath.Join(tree, "leaf.txt"), []byte("x"), 0644)
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: filepath.Join(treeBase, "tree"), Recurse: true})); err != nil {
		t.Fatalf("rm recurse error = %v", err)
	}

	// 空目录直接删
	empty := filepath.Join(t.TempDir(), "empty")
	_ = os.Mkdir(empty, 0755)
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: empty})); err != nil {
		t.Fatalf("rm empty dir error = %v", err)
	}

	// 不存在路径
	if _, err := d.handleFSRm(context.Background(), mustParams(t, rpc.FSRmParams{Path: filepath.Join(dir, "nope")})); err == nil {
		t.Error("missing path: expected error")
	}
}

func TestHandleFSMv(t *testing.T) {
	d := fsTestDaemon()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "moved", "dst.txt")
	_ = os.WriteFile(src, []byte("mv"), 0644)

	if _, err := d.handleFSMv(context.Background(), mustParams(t, rpc.FSMvParams{Src: src, Dst: dst})); err != nil {
		t.Fatalf("handleFSMv error = %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("src should be gone after mv")
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "mv" {
		t.Errorf("dst content = %q, want mv", got)
	}

	// 源不存在报错
	if _, err := d.handleFSMv(context.Background(), mustParams(t, rpc.FSMvParams{Src: src, Dst: dst})); err == nil {
		t.Error("missing src: expected error")
	}
}

func TestHandleFSReveal_NotFound(t *testing.T) {
	// 只覆盖校验错误分支：成功路径会真弹 Finder/文件管理器，不能在单测触发
	d := fsTestDaemon()
	if _, err := d.handleFSReveal(context.Background(), mustParams(t, rpc.FSRevealParams{Path: filepath.Join(t.TempDir(), "nope")})); err == nil {
		t.Error("missing path: expected error")
	}
}

func TestHandleFSStat(t *testing.T) {
	d := fsTestDaemon()
	dir := fsFixture(t)

	result, err := d.handleFSStat(context.Background(), mustParams(t, rpc.FSStatParams{Path: filepath.Join(dir, "a.txt")}))
	if err != nil {
		t.Fatalf("handleFSStat error = %v", err)
	}
	st := result.(rpc.FSStatResult)
	if st.Name != "a.txt" || st.IsDir || st.Size != 5 {
		t.Errorf("stat = %+v", st)
	}

	// 目录
	result, err = d.handleFSStat(context.Background(), mustParams(t, rpc.FSStatParams{Path: filepath.Join(dir, "sub")}))
	if err != nil {
		t.Fatalf("handleFSStat dir error = %v", err)
	}
	if !result.(rpc.FSStatResult).IsDir {
		t.Error("sub should report is_dir=true")
	}

	if _, err := d.handleFSStat(context.Background(), mustParams(t, rpc.FSStatParams{Path: filepath.Join(dir, "nope")})); err == nil {
		t.Error("missing path: expected error")
	}
}
