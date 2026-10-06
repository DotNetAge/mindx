package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goharnesslogging "github.com/DotNetAge/goharness/logging"
)

func TestParseEntries_PlainArray(t *testing.T) {
	in := `[{"title":"修复压缩死循环","summary":"冷却机制落地","content":"- 决策：persist 失败不移 cursor\n- 路径：/Users/x/compaction.go","tags":["compaction"],"refs":[],"timestamp":"2026-10-06T10:00:00Z"}]`
	out := parseEntries(in)
	if len(out) != 1 {
		t.Fatalf("应解析出 1 条, got %d", len(out))
	}
	if out[0].Title != "修复压缩死循环" || len(out[0].Tags) != 1 {
		t.Errorf("字段解析不符: %+v", out[0])
	}
}

func TestParseEntries_FenceWrapped(t *testing.T) {
	in := "```json\n[{\"title\":\"t\",\"content\":\"c\"}]\n```"
	out := parseEntries(in)
	if len(out) != 1 || out[0].Title != "t" {
		t.Errorf("围栏包裹的 JSON 应被剥离后解析: %+v", out)
	}
}

func TestParseEntries_InvalidFallsBackToNil(t *testing.T) {
	for _, in := range []string{
		"这不是 JSON，是 LLM 走样的纯文本总结",
		"{\"title\": \"单个对象而非数组\"}",
		"",
	} {
		if out := parseEntries(in); out != nil {
			t.Errorf("非法输入应返回 nil 触发兜底, input=%q got %+v", in, out)
		}
	}
}

func TestParseEntries_EmptyEntriesFiltered(t *testing.T) {
	in := `[{"title":"","content":""},{"title":"有效条目","content":"c"}]`
	out := parseEntries(in)
	if len(out) != 1 || out[0].Title != "有效条目" {
		t.Errorf("空条目应被过滤: %+v", out)
	}
}

func TestWrite_JsonEntries_OneFilePerEntry(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemorySink(goharnesslogging.NewNopLogger(), dir)
	summary := `[{"title":"决策一","summary":"选了A","content":"- 理由：性能","tags":["a"],"refs":[],"timestamp":""},{"title":"踩坑二","summary":"B方案失败","content":"- 现象：超时\n- 解法：加重试","tags":["b"],"refs":["ref_001"],"timestamp":""}]`
	if err := s.Write(context.Background(), dir, "agent-x", "sess12345", summary); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var mdFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	if len(mdFiles) != 2 {
		t.Fatalf("两条记忆应落盘两个文件, got %v", mdFiles)
	}
	// 文件名四元组：记录时间-agent名-分类-标题（分类 = tags 首个关键词）
	for _, f := range mdFiles {
		if !strings.Contains(f, "-agent-x-") {
			t.Errorf("文件名应包含 agent 名: %s", f)
		}
	}
	if !strings.Contains(mdFiles[0], "-a-") && !strings.Contains(mdFiles[1], "-a-") {
		t.Errorf("文件名应包含分类（tags[0]）: %v", mdFiles)
	}
	for _, f := range mdFiles {
		data, _ := os.ReadFile(filepath.Join(dir, f))
		text := string(data)
		if !strings.HasPrefix(text, "# ") {
			t.Errorf("%s 应以 H1 标题开头", f)
		}
		if !strings.Contains(text, "agent: agent-x") || !strings.Contains(text, "session: sess12345") {
			t.Errorf("%s 元信息行缺失 agent/session: %s", f, text)
		}
	}
	// refs 出现在对应文件元信息行
	found := false
	for _, f := range mdFiles {
		data, _ := os.ReadFile(filepath.Join(dir, f))
		if strings.Contains(string(data), "refs: ref_001") {
			found = true
		}
	}
	if !found {
		t.Error("含 refs 的条目元信息行应包含 refs 列表")
	}
}

func TestWrite_InvalidJson_FallbackSingleFile(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemorySink(goharnesslogging.NewNopLogger(), dir)
	summary := "LLM 走样输出：这不是 JSON，就是一段总结文字。"
	if err := s.Write(context.Background(), dir, "agent-x", "sess12345", summary); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("兜底应写单个文件, got %d", len(entries))
	}
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if !strings.Contains(string(data), "LLM 走样输出") || !strings.Contains(string(data), "fallback") {
		t.Errorf("兜底文件应保留完整原文并带 fallback 标记: %s", data)
	}
}

func TestWrite_EmptySummaryRejected(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemorySink(goharnesslogging.NewNopLogger(), dir)
	if err := s.Write(context.Background(), dir, "a", "s", "  "); err == nil {
		t.Fatal("空产物应返回错误（压缩失败，cursor 不移动）")
	}
}

func TestMemoryDirFor_EmptyProjectFallsBackToWorkspace(t *testing.T) {
	s := NewFileMemorySink(goharnesslogging.NewNopLogger(), "/home/x/.mindx")
	if got := s.MemoryDirFor(""); got != "/home/x/.mindx/.agents/memory" {
		t.Errorf("MemoryDirFor(\"\") = %q", got)
	}
	if got := s.MemoryDirFor("/proj"); got != "/proj/.agents/memory" {
		t.Errorf("MemoryDirFor(/proj) = %q", got)
	}
}
