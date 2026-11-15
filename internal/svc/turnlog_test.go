package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// appendAssistantStop 追加一条 assistant + stop 的最终回复消息。
func appendAssistantStop(t *testing.T, sess *goharnesssession.Session, content, finishReason string) {
	t.Helper()
	if err := sess.Append(context.Background(), goharnesssession.Message{
		Role: "assistant", Content: content, FinishReason: finishReason, Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("append assistant message: %v", err)
	}
}

func TestTurnLogEntry(t *testing.T) {
	started := time.Date(2026, 11, 15, 10, 30, 5, 0, time.Local)
	got := turnLogEntry(started, "  问题原文  ", " 回答原文 ")
	if !strings.Contains(got, "# 轮次 10:30:05") {
		t.Errorf("missing turn header: %q", got)
	}
	if !strings.Contains(got, "## 用户问题\n\n问题原文") || !strings.Contains(got, "## 最终回复\n\n回答原文") {
		t.Errorf("entry should contain trimmed question/answer: %q", got)
	}
}

func TestAppendTurnLog_HappyPath(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")
	lenBefore := len(sess.All())
	appendAssistantStop(t, sess, "最终答案", "stop")

	started := time.Date(2026, 11, 15, 9, 0, 0, 0, time.Local)
	d.appendTurnLog(sess, "tester", "用户提问", lenBefore, started)

	// 文件应落在项目目录 .agents/logs 下
	projectDir := sess.ProjectDir()
	if projectDir == "" {
		t.Fatal("session project dir is empty")
	}
	dir := filepath.Join(projectDir, ".agents/logs")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("log dir entries = %d err = %v, want 1", len(entries), err)
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "2026-11-15-090000-tester") || !strings.HasSuffix(name, ".md") {
		t.Errorf("log file name = %q", name)
	}
	content, _ := os.ReadFile(filepath.Join(dir, name))
	if !strings.Contains(string(content), "用户提问") || !strings.Contains(string(content), "最终答案") {
		t.Errorf("log content missing question/answer: %q", content)
	}
}

func TestAppendTurnLog_SkipBranches(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")

	// nil session / 空问题 → 不落盘
	d.appendTurnLog(nil, "tester", "q", 0, time.Now())
	d.appendTurnLog(sess, "tester", "   ", 0, time.Now())

	// 新增区间无 stop 回复 → 不落盘（answerLenBefore 圈定区间为空）
	d.appendTurnLog(sess, "tester", "q", len(sess.All())+5, time.Now())

	// 有项目目录的会话在上述分支下均不应产生日志
	if dir := sess.ProjectDir(); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, ".agents/logs")); !os.IsNotExist(err) {
			t.Errorf("unexpected turn log dir under %s", dir)
		}
	}
}

func TestAppendTurnLog_SameSecondSuffix(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")
	appendAssistantStop(t, sess, "answer", "stop")

	started := time.Date(2026, 11, 15, 9, 0, 0, 0, time.Local)
	d.appendTurnLog(sess, "tester", "q", 0, started)
	d.appendTurnLog(sess, "tester", "q", 0, started)

	dir := filepath.Join(sess.ProjectDir(), ".agents/logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2（同秒第二次落盘追加 -2 序号）", len(entries))
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-2.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("second file name should carry -2 后缀, got %v", entries)
	}
}

func TestTurnLogFinalAnswer(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")

	// lenBefore >= len(msgs) → 空串
	if got := turnLogFinalAnswer(sess, 99); got != "" {
		t.Errorf("out-of-range = %q, want empty", got)
	}
	// 负数下标收敛为 0；区间内无 stop assistant → 空串
	if got := turnLogFinalAnswer(sess, -1); got != "" {
		t.Errorf("no stop answer = %q, want empty", got)
	}

	// 追加两条 stop assistant → 取最后一条（AskUser 追答场景）
	appendAssistantStop(t, sess, "第一版答案", "stop")
	appendAssistantStop(t, sess, "最终答案", "stop")
	lenBefore := len(sess.All()) - 2
	if got := turnLogFinalAnswer(sess, lenBefore); got != "最终答案" {
		t.Errorf("final answer = %q, want 最终答案", got)
	}

	// 区间外的 stop 回复不取
	if got := turnLogFinalAnswer(sess, len(sess.All())); got != "" {
		t.Errorf("answer outside range = %q, want empty", got)
	}
}
