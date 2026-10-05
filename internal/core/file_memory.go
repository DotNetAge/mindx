package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/logging"
)

// FileMemorySink 把压缩摘要原文写为 .agents/memory/ 目录下的 markdown 记忆文件。
//
// 设计定稿（2026-10-06 记忆剥离重构）：记忆 = 人可读可编辑的普通文件。
// Agent 通过提示词规则（SOUL.md / experienceSystemNote）在会话开始时自主加载
// 最新记忆文件，会话中途用 Read/Grep 工具按需检索；用户可直接查看、编辑、删除。
// 无 embedding、无向量库、无注入 Hook——goharness 只交付原文，存哪、存成什么样
// 由本层决定。
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

// slugSanitize 清理文件名中的非法字符与路径分隔符。
var slugSanitize = regexp.MustCompile(`[\\/:*?"<>|\s]+`)

// Write 把一篇压缩摘要写入 memoryDir 下的 markdown 记忆文件。
//
// 文件名：{yyyyMMdd-HHmmss}-{sessionID 前 8 位}-{标题 slug}.md，
// 时间序文件名使"最新记忆"天然等于 ls 倒序第一个，无需索引文件。
// 文件头附加一行引用元信息（时间 / agent / session），正文为 LLM 摘要原文。
// 写入失败返回错误：调用方（Session.SummarySink）必须将其视为压缩失败，
// cursor 不移动，防止丢历史不留记忆。
func (s *FileMemorySink) Write(_ context.Context, memoryDir, agentName, sessionID, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("filememory: 摘要原文为空，拒绝写入")
	}
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return fmt.Errorf("filememory: 创建记忆目录 %s 失败: %w", memoryDir, err)
	}

	name := s.fileName(sessionID, summary)
	path := filepath.Join(memoryDir, name)

	header := fmt.Sprintf("> %s | agent: %s | session: %s\n\n",
		time.Now().Format("2006-01-02 15:04"), agentName, sessionID)
	content := header + strings.TrimRight(summary, "\n") + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("filememory: 写入记忆文件 %s 失败: %w", path, err)
	}
	s.logger.Info("filememory: 记忆文件已落盘", "path", path, "bytes", len(content))
	return nil
}

// fileName 生成记忆文件名：时间戳 + session 前 8 位保证时序唯一，
// 标题 slug 取摘要首个 H1（LLM 按压缩指令必然输出），供人类与 Agent 浏览识别。
func (s *FileMemorySink) fileName(sessionID, summary string) string {
	title := summaryTitle(summary)
	slug := slugSanitize.ReplaceAllString(title, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "summary"
	}
	// 按 rune 截断：标题多为中文，按字节切片会产生无效 UTF-8 的乱码文件名
	runes := []rune(slug)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	slug = string(runes)
	sess := sessionID
	if len(sess) > 8 {
		sess = sess[:8]
	}
	return fmt.Sprintf("%s-%s-%s.md", time.Now().Format("20060102-150405"), sess, slug)
}

// summaryTitle 提取摘要的首个 H1 行文本（去掉 "#" 前缀）；无 H1 时回退"会话摘要"。
func summaryTitle(summary string) string {
	for _, line := range strings.Split(summary, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return "会话摘要"
}
