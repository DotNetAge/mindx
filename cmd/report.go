package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── report parent ─────────────────────────────────────────────

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Work report log operations (turn logs harvesting)",
	Long: `Harvest turn logs (.agents/logs/) for daily reports.

Turn logs are written by the daemon after each conversation turn:
raw user question + final answer, stored per project directory.
The collect command gathers one day's logs from several work
directories into a single file, then removes the source files.

Examples:
  mindx report collect --date 2026-10-12
  mindx report collect --date 2026-10-12 --agent coder
  mindx report collect --date 2026-10-12 --dir ~/workspaces/other-project`,
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportCollectCmd)

	reportCollectCmd.Flags().String("date", "", "Date to harvest, format YYYY-MM-DD (required)")
	reportCollectCmd.Flags().StringSlice("dir", nil, "Additional work directories to harvest (repeatable); current directory is always included")
	reportCollectCmd.Flags().String("agent", "", "Only harvest logs of this agent name")
	reportCollectCmd.Flags().String("out", "", "Output file (default: <cwd>/.agents/logs/collected-<date>.md)")
	_ = reportCollectCmd.MarkFlagRequired("date")
}

// ── report collect ────────────────────────────────────────────

// turnLogFileNameRe 匹配轮次日志文件名：<date>-<HHMMSS>-<agent>[-N].md。
// date 与时间由 daemon 生成（格式固定），agent 名可含连字符，
// 尾部 -N 是同秒同名冲突时的序号。
var turnLogFileNameRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(\d{6})-(.+)\.md$`)

// turnLogAgentName 从匹配的文件名中解析 agent 名：正则第 3 组去掉尾部
// 数字序号（-2、-3）。解析失败返回空串（该文件在 --agent 过滤时跳过）。
func turnLogAgentName(reMatch []string) string {
	agent := reMatch[3]
	if idx := strings.LastIndex(agent, "-"); idx > 0 {
		tail := agent[idx+1:]
		if isAllDigits(tail) {
			agent = agent[:idx]
		}
	}
	return agent
}

// isAllDigits 判断字符串是否全为数字。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var reportCollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Harvest one day's turn logs into a single file, then remove sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		date, _ := cmd.Flags().GetString("date")
		agent, _ := cmd.Flags().GetString("agent")
		out, _ := cmd.Flags().GetString("out")
		extraDirs, _ := cmd.Flags().GetStringSlice("dir")

		// 日期格式校验：既是文件名前缀也是收割范围，格式错了宁可拒绝。
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return fmt.Errorf("--date 必须是 YYYY-MM-DD 格式（收到 %q）", date)
		}

		// 目标目录由调用方显式给出（当前目录 + --dir 附加目录），
		// 不依赖任何注册表。去重并做绝对路径归一。
		seen := map[string]bool{}
		var dirs []string
		addDir := func(p string) {
			abs, err := filepath.Abs(p)
			if err != nil || seen[abs] {
				return
			}
			seen[abs] = true
			dirs = append(dirs, abs)
		}
		cwd, _ := os.Getwd()
		addDir(cwd)
		for _, d := range extraDirs {
			addDir(d)
		}

		// 第一阶段：读取全部匹配文件（不改动任何源文件）。
		type turnLog struct {
			path    string // 绝对路径
			content string
		}
		var logs []turnLog
		for _, dir := range dirs {
			logDir := filepath.Join(dir, ".agents", "logs")
			entries, err := os.ReadDir(logDir)
			if err != nil {
				continue // 该目录没有轮次日志，直接跳过
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				m := turnLogFileNameRe.FindStringSubmatch(e.Name())
				if m == nil || m[1] != date {
					continue
				}
				if agent != "" {
					if logAgent := turnLogAgentName(m); logAgent != agent {
						continue
					}
				}
				path := filepath.Join(logDir, e.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("读取 %s 失败: %w", path, err)
				}
				logs = append(logs, turnLog{path: path, content: string(data)})
			}
		}

		// 按文件名排序：同一天内按时间顺序呈现。
		sort.Slice(logs, func(i, j int) bool {
			return filepath.Base(logs[i].path) < filepath.Base(logs[j].path)
		})

		if len(logs) == 0 {
			fmt.Println("没有找到匹配的轮次日志，未生成收割文件，源目录不受影响。")
			return nil
		}

		// 聚合内容：来源路径以注释形式随文保存，正文保持原文零提炼。
		var sb strings.Builder
		fmt.Fprintf(&sb, "# 轮次日志收割 @%s\n\n", date)
		fmt.Fprintf(&sb, "- 收割时间：%s\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&sb, "- 来源目录：%s\n", strings.Join(dirs, "、"))
		if agent != "" {
			fmt.Fprintf(&sb, "- Agent 过滤：%s\n", agent)
		}
		fmt.Fprintf(&sb, "- 文件数：%d\n", len(logs))
		for _, lg := range logs {
			fmt.Fprintf(&sb, "\n---\n\n<!-- 来源: %s -->\n\n", lg.path)
			sb.WriteString(strings.TrimRight(lg.content, "\n"))
			sb.WriteString("\n")
		}

		// 第二阶段（先聚合落盘校验成功、后删源）：输出文件写成功之前
		// 不碰任何源文件；写失败时源文件原样保留，可安全重试。
		if out == "" {
			out = filepath.Join(cwd, ".agents", "logs", "collected-"+date+".md")
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
		if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("写入聚合文件失败（源文件未删除，可重试）: %w", err)
		}

		// 源文件删除：聚合文件已安全落盘，逐个删除；删除失败的列入报告
		//（下次 collect 会重复收割该文件，聚合内容以文件数标注可发现）。
		var failed []string
		for _, lg := range logs {
			if err := os.Remove(lg.path); err != nil {
				failed = append(failed, lg.path)
			}
		}

		fmt.Printf("已收割 %d 条轮次日志 → %s\n", len(logs), out)
		if len(failed) > 0 {
			fmt.Printf("警告：%d 个源文件删除失败（内容已在聚合文件中，删除失败的会在下次收割时重复出现）:\n", len(failed))
			for _, p := range failed {
				fmt.Println("  " + p)
			}
		}
		return nil
	},
}
