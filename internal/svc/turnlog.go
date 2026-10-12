package svc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// 本文件承载「轮次日志落盘」（日报事实流采集层）：
//
// 每轮 Ask 结束后，把当轮的用户问题原文与助手最终回复原文原样写入
// <Session.ProjectDir>/.agents/logs/<YYYY-MM-DD-HHMMSS>-<agent_name>.md。
//
// 设计定稿（2026-10-12 与用户确认）：
//   - 零提炼零算力：只存原文，不做任何结论摘要（提炼留给日报生成时一次完成）；
//   - 这是日报（daily-report 技能）的专用事实流：由 mindx report collect
//     跨工作目录收割（聚合落盘校验成功后才删除源文件），消费后即删除；
//   - 不进复盘原料：notes 才是复盘素材，轮次日志只服务日报；
//   - 落点在 mindx 应用层（daemon），不改 goharness 机制。
const turnLogDirName = ".agents/logs"

// turnLogEntry 是单条轮次日志的落盘内容（原文拼接，零提炼）。
func turnLogEntry(startedAt time.Time, question, answer string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# 轮次 %s\n\n", startedAt.Format("15:04:05"))
	sb.WriteString("## 用户问题\n\n")
	sb.WriteString(strings.TrimSpace(question))
	sb.WriteString("\n\n## 最终回复\n\n")
	sb.WriteString(strings.TrimSpace(answer))
	sb.WriteString("\n")
	return sb.String()
}

// appendTurnLog 把一轮对话的原文写入项目目录的轮次日志。
// 在 AskBuilder.Run() 返回后调用；任何一步失败只记日志不中断对话流程
// （落盘是旁路采集，绝不能影响主对话）。
//
// 参数：session 用于定位项目目录与提取最终回复；agentName 用于文件命名；
// question 是本轮用户问题原文（调用方闭包持有的 content，比从消息历史
// 反查可靠——AskUser 追答等场景下历史里有多条 user 消息）；
// answerLenBefore 是 Run() 之前 s.All() 的长度，用于圈定本轮新增消息区间，
// 防止本轮无最终回复时误取上一轮的答案。
func (d *Daemon) appendTurnLog(s *goharnesssession.Session, agentName, question string, answerLenBefore int, startedAt time.Time) {
	if s == nil || strings.TrimSpace(question) == "" {
		return
	}
	projectDir := s.ProjectDir()
	if projectDir == "" {
		// 没有归属目录就不落盘（不猜目录，避免写错位置）。
		return
	}
	answer := turnLogFinalAnswer(s, answerLenBefore)
	if strings.TrimSpace(answer) == "" {
		// 本轮没有最终回复（执行失败 / 被用户中止）：不落盘。
		return
	}

	logDir := filepath.Join(projectDir, turnLogDirName)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		d.logger.Warn("turnlog: create log dir failed", "error", err, "dir", logDir)
		return
	}

	// 文件名：<YYYY-MM-DD-HHMMSS>-<agent_name>.md；同一秒同 Agent 已存在时
	// 追加序号 -2、-3（与日报命名规则一致）。O_EXCL 独占创建保证并发安全。
	base := filepath.Join(logDir, fmt.Sprintf("%s-%s.md",
		startedAt.Format("2006-01-02-150405"), agentName))
	target := base
	for i := 2; ; i++ {
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, writeErr := f.WriteString(turnLogEntry(startedAt, question, answer))
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				d.logger.Warn("turnlog: write failed", "error", writeErr, "file", target)
				return
			}
			return
		}
		if !os.IsExist(err) {
			d.logger.Warn("turnlog: create file failed", "error", err, "file", target)
			return
		}
		target = fmt.Sprintf("%s-%d.md", strings.TrimSuffix(base, ".md"), i)
	}
}

// turnLogFinalAnswer 从会话消息中提取本轮的最终回复。
// 只在 Run() 之前记录长度之后的「新增区间」里找最后一条 FinishReason == "stop"
// 的 assistant 消息（协议内建的答案边界信号，AskUser 追答产生多条 assistant
// 消息时取最后一条即结论）；新增区间内没有则返回空串。
func turnLogFinalAnswer(s *goharnesssession.Session, lenBefore int) string {
	msgs := s.All()
	if lenBefore < 0 {
		lenBefore = 0
	}
	if lenBefore >= len(msgs) {
		return ""
	}
	for i := len(msgs) - 1; i >= lenBefore; i-- {
		m := msgs[i]
		if m.Role == "assistant" && m.FinishReason == "stop" && strings.TrimSpace(m.Content) != "" {
			return m.Content
		}
	}
	return ""
}
