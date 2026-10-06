package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/logging"
)

// FileMemorySink 把压缩产物落盘为 .agents/memory/ 目录下的记忆文件。
//
// 设计定稿（2026-10-06 双系统记忆理论）：压缩器输出 JSON 数组，数组每一项是
// 一条独立的记忆条目（事件/决策/经验/踩坑），逐条写为一个 markdown 文件——
// 单条粒度才能支撑复盘的聚类合并、按条遗忘归档与独立检索。
// 记忆 = 人可读可编辑的普通文件；Agent 经系统提示规则用 Read/Grep 自助加载；
// meta 索引卡片由复盘阶段从这些文件提取（软实现，本层不做 meta 管理）。
type FileMemorySink struct {
	logger logging.Logger
	// workspaceDir 回退根目录：会话无项目目录时使用（~/.mindx/.agents/memory/）
	workspaceDir string
}

// NewFileMemorySink 创建文件记忆交付器。
// workspaceDir 为 daemon 工作目录（~/.mindx），仅用于无项目目录会话的回退。
func NewFileMemorySink(logger logging.Logger, workspaceDir string) *FileMemorySink {
	return &FileMemorySink{logger: logger, workspaceDir: workspaceDir}
}

// MemoryDirFor 返回指定项目目录的记忆文件目录。
// projectDir 为空时回退到 ~/.mindx/.agents/memory/。
func (s *FileMemorySink) MemoryDirFor(projectDir string) string {
	if projectDir == "" {
		return filepath.Join(s.workspaceDir, ".agents", "memory")
	}
	return filepath.Join(projectDir, ".agents", "memory")
}

// memoryEntry 是压缩条目的内部结构（与压缩提示词的 JSON 字段一一对应）。
type memoryEntry struct {
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Refs      []string `json:"refs"`
	Timestamp string   `json:"timestamp"`
}

// slugSanitize 清理文件名中的非法字符与路径分隔符。
var slugSanitize = regexp.MustCompile(`[\\/:*?"<>|\s]+`)

// fenceRe 剥离 LLM 可能包裹 JSON 的 markdown 代码围栏。
var fenceRe = regexp.MustCompile("(?s)^```[a-zA-Z]*\\s*(.*?)\\s*```$")

// parseEntries 宽容解析压缩产物：剥离代码围栏后解析 JSON 数组。
// 解析失败或数组为空返回 nil——调用方降级为整篇原文单文件落盘（保记忆不丢）。
func parseEntries(summary string) []memoryEntry {
	raw := strings.TrimSpace(summary)
	if m := fenceRe.FindStringSubmatch(raw); m != nil {
		raw = strings.TrimSpace(m[1])
	}
	if raw == "" || raw[0] != '[' {
		return nil
	}
	var entries []memoryEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil
	}
	// 过滤掉无实质内容的条目（title 与 content 均为空）
	out := make([]memoryEntry, 0, len(entries))
	for _, e := range entries {
		if strings.TrimSpace(e.Title) == "" && strings.TrimSpace(e.Content) == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Write 把一次压缩交付写入 memoryDir：
//  1. 产物为合法 JSON 数组 → 逐条写为独立记忆文件（主路径）
//  2. 解析失败（LLM 输出走样）→ 整篇原文写为单个兜底文件，记忆不丢
//
// 任一文件写入失败返回错误：调用方（Session.SummarySink）必须将其视为压缩
// 失败，cursor 不移动，防止丢历史不留记忆。
func (s *FileMemorySink) Write(_ context.Context, memoryDir, agentName, sessionID, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("filememory: 压缩产物为空，拒绝写入")
	}
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return fmt.Errorf("filememory: 创建记忆目录 %s 失败: %w", memoryDir, err)
	}

	batch := time.Now()
	sess := sessionID
	if len(sess) > 8 {
		sess = sess[:8]
	}

	entries := parseEntries(summary)
	if len(entries) == 0 {
		// 兜底：产物不是合法 JSON 数组（LLM 输出走样），整篇原文写单文件。
		// 记忆不丢，格式偏差留给复盘阶段清洗。
		name := fmt.Sprintf("%s-%s-summary.md", batch.Format("20060102-150405"), sess)
		path := filepath.Join(memoryDir, name)
		content := fallbackHeader(batch, agentName, sessionID) + strings.TrimRight(summary, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("filememory: 写入兜底记忆文件 %s 失败: %w", path, err)
		}
		s.logger.Warn("filememory: 压缩产物非 JSON 数组，已整篇落盘为兜底文件",
			"path", path, "bytes", len(content))
		return nil
	}

	written := 0
	for _, e := range entries {
		name := s.fileName(batch, sess, e.Title)
		path := filepath.Join(memoryDir, name)
		if err := os.WriteFile(path, []byte(renderEntry(batch, agentName, sessionID, e)), 0o644); err != nil {
			return fmt.Errorf("filememory: 写入记忆文件 %s 失败: %w", path, err)
		}
		written++
	}
	s.logger.Info("filememory: 记忆条目已落盘",
		"dir", memoryDir, "entries", written, "bytes", len(summary))
	return nil
}

// fileName 生成记忆文件名：批次时间戳 + session 前 8 位保证时序与批次归组，
// 标题 slug 供人类与 Agent 浏览识别。同秒同标题冲突时追加 -N 序号。
func (s *FileMemorySink) fileName(batch time.Time, sess, title string) string {
	slug := strings.Trim(slugSanitize.ReplaceAllString(title, "-"), "-")
	// 按 rune 截断：标题多为中文，按字节切片会产生无效 UTF-8 的乱码文件名
	runes := []rune(slug)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	slug = string(runes)
	if slug == "" {
		slug = "memory"
	}
	base := fmt.Sprintf("%s-%s-%s", batch.Format("20060102-150405"), sess, slug)
	return base + ".md"
}

// renderEntry 把一条记忆渲染为人可读可编辑的 markdown 文件内容。
// 首行标题 + 一行元信息（date/agent/session/tags/refs）+ 概括 + 正文。
// 元信息行供人浏览识别，meta 索引卡片由复盘阶段从文件提取。
func renderEntry(batch time.Time, agentName, sessionID string, e memoryEntry) string {
	eventAt := batch
	if e.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			eventAt = t
		}
	}
	meta := []string{eventAt.Format("2006-01-02 15:04"), "agent: " + agentName, "session: " + sessionID}
	if len(e.Tags) > 0 {
		meta = append(meta, "tags: "+strings.Join(e.Tags, ", "))
	}
	if len(e.Refs) > 0 {
		meta = append(meta, "refs: "+strings.Join(e.Refs, ", "))
	}

	title := strings.TrimSpace(e.Title)
	if title == "" {
		title = "会话记忆"
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("> ")
	b.WriteString(strings.Join(meta, " | "))
	b.WriteString("\n\n")
	if sm := strings.TrimSpace(e.Summary); sm != "" {
		b.WriteString(sm + "\n\n")
	}
	if ct := strings.TrimSpace(e.Content); ct != "" {
		b.WriteString(strings.TrimRight(ct, "\n") + "\n")
	}
	return b.String()
}

// fallbackHeader 兜底文件的元信息头（产物非 JSON 时保留完整原文）。
func fallbackHeader(batch time.Time, agentName, sessionID string) string {
	return fmt.Sprintf("> %s | agent: %s | session: %s | fallback\n\n",
		batch.Format("2006-01-02 15:04"), agentName, sessionID)
}
