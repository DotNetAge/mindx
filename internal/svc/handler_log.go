package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/pkg/logging"
)

// logReadParams — 逆向分页读取日志
//
//	offset: 从文件末尾向前的偏移行数（0 = 从最后开始）
//	limit:  每次返回的行数（默认 10）
//	stream: "main" (默认) 或 "error" — 选择读取哪个日志流
type logReadParams struct {
	Offset int    `json:"offset,omitempty"` // 从末尾向前的偏移（行数）
	Limit  int    `json:"limit,omitempty"`  // 每页行数，默认 10
	Stream string `json:"stream,omitempty"` // "main" | "error"，默认 "main"
}

// 允许的日志流名
var logStreamFilenames = map[string]string{
	"main":  "mindx.log",
	"error": "error.log",
}

func (d *Daemon) handleLogRead(_ context.Context, params json.RawMessage) (any, error) {
	var p logReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Limit <= 0 {
		p.Limit = 10
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Stream == "" {
		p.Stream = "main"
	}

	filename, ok := logStreamFilenames[p.Stream]
	if !ok {
		return nil, fmt.Errorf("unknown log stream %q (allowed: main, error)", p.Stream)
	}

	logPath := filepath.Join(logging.ResolveLogDir(), filename)
	data, err := os.ReadFile(logPath)
	if err != nil {
		// error.log 可能尚未生成 — 返回空列表而不是错误
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"lines":    []string{},
				"total":    0,
				"returned": 0,
				"offset":   p.Offset,
				"has_more": false,
				"path":     logPath,
				"stream":   p.Stream,
			}, nil
		}
		return nil, fmt.Errorf("read log %s: %w", logPath, err)
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	// 逆向分页：从末尾往前取 [offset, offset+limit)
	end := total - p.Offset
	start := end - p.Limit
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}

	return map[string]interface{}{
		"lines":    lines[start:end],
		"total":    total,
		"returned": end - start,
		"offset":   p.Offset,
		"has_more": start > 0, // 是否还有更早的日志可加载
		"path":     logPath,
		"stream":   p.Stream,
	}, nil
}

type logClearParams struct {
	Confirmed bool `json:"confirmed"` // must be true to confirm destructive action
}

func (d *Daemon) handleLogClear(_ context.Context, params json.RawMessage) (any, error) {
	var p logClearParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if !p.Confirmed {
		return nil, fmt.Errorf("clear requires confirmed=true")
	}

	logDir := logging.ResolveLogDir()
	cleared := make([]string, 0, len(logStreamFilenames))
	for _, filename := range logStreamFilenames {
		path := filepath.Join(logDir, filename)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("clear log %s: %w", path, err)
		}
		cleared = append(cleared, path)
	}

	return map[string]any{
		"status":  "ok",
		"cleared": cleared,
	}, nil
}

// handleLogCount — 轻量级统计接口：只计行数，不返回内容
// 返回每个日志流的字节数和行数（行数 = 换行符数 + 1（如果文件非空且不以 \n 结尾））
// 用于 UI 在标签上显示数量徽章
func (d *Daemon) handleLogCount(_ context.Context, _ json.RawMessage) (any, error) {
	logDir := logging.ResolveLogDir()
	counts := make(map[string]map[string]int64, len(logStreamFilenames))

	for stream, filename := range logStreamFilenames {
		path := filepath.Join(logDir, filename)
		counts[stream] = countLogFile(path)
	}

	return map[string]any{
		"counts": counts,
	}, nil
}

// countLogFile 统计日志文件的字节数和行数（仅计换行符，性能 O(N) 但只读不解析）
func countLogFile(path string) map[string]int64 {
	result := map[string]int64{
		"bytes":  0,
		"lines":  0,
		"exists": 0,
	}
	info, err := os.Stat(path)
	if err != nil {
		return result
	}
	result["exists"] = 1
	result["bytes"] = info.Size()

	if info.Size() == 0 {
		return result
	}

	// 大文件按 64KB 块流式计数，避免一次性读到内存
	f, err := os.Open(path)
	if err != nil {
		return result
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 64*1024)
	var lines int64
	var endsWithNewline bool
	for {
		n, rerr := f.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				lines++
			}
		}
		if n > 0 {
			endsWithNewline = buf[n-1] == '\n'
		}
		if rerr != nil {
			break
		}
	}
	// 如果文件不以换行结尾，最后一行也算一行
	if info.Size() > 0 && !endsWithNewline {
		lines++
	}
	result["lines"] = lines
	return result
}
