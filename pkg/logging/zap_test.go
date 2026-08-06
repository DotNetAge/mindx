package logging

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLogFileAutoRebuildAfterDelete 验证日志文件被外部删除后能自动重建。
//
// 回归背景：原实现持有文件句柄，删除后写入落到已删除的 inode 上，
// 日志文件不会重建、日志内容全部丢失（黑洞）。
func TestLogFileAutoRebuildAfterDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir := t.TempDir()
	path := filepath.Join(dir, "mindx.log")
	cfg := &ZapConfig{Filename: path, MaxSize: 20, MaxBackups: 7, MaxAge: 30, Compress: true}
	w := createSafeFileWriter(cfg)

	if _, err := w.Write([]byte("first write\n")); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}

	// 模拟用户手动删除日志文件
	if err := os.Remove(path); err != nil {
		t.Fatalf("删除日志文件失败: %v", err)
	}

	// 删除后再次写入：应自动重建文件，而非写入已删除的 inode
	if _, err := w.Write([]byte("after delete\n")); err != nil {
		t.Fatalf("删除后写入失败: %v", err)
	}
	if err := w.Sync(); err != nil {
		t.Fatalf("Sync 失败: %v", err)
	}

	select {
	case <-ctx.Done():
		t.Fatal("测试超时")
	default:
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("重建后的日志文件不存在: %v", err)
	}
	if !bytes.Contains(data, []byte("after delete")) {
		t.Fatalf("重建后的日志文件缺少删除后的写入内容，实际内容: %q", data)
	}
	if bytes.Contains(data, []byte("first write")) {
		t.Fatalf("重建后的日志不应包含删除前的写入（已随旧 inode 丢失）")
	}
}

// TestLogRotation 验证日志按大小滚动：写入量超过 MaxSize 后生成滚动备份文件。
func TestLogRotation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir := t.TempDir()
	path := filepath.Join(dir, "mindx.log")
	cfg := &ZapConfig{Filename: path, MaxSize: 1, MaxBackups: 7, MaxAge: 30, Compress: true}
	w := createSafeFileWriter(cfg)

	chunk := bytes.Repeat([]byte("a"), 256*1024) // 256KB
	for i := 0; i < 6; i++ {                     // 共 1.5MB，必然超过 1MB 阈值
		if _, err := w.Write(chunk); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
	}
	_ = w.Sync()

	select {
	case <-ctx.Done():
		t.Fatal("测试超时")
	default:
	}

	entries, _ := os.ReadDir(dir)
	backupCount := 0
	for _, e := range entries {
		if e.Name() != "mindx.log" && e.Name() != ".DS_Store" {
			backupCount++
		}
	}
	if backupCount == 0 {
		t.Fatal("超过 MaxSize 后应生成滚动备份文件，但目录中没有备份")
	}
}
