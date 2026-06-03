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
//   offset: 从文件末尾向前的偏移行数（0 = 从最后开始）
//   limit:  每次返回的行数（默认 10）
type logReadParams struct {
	Offset int `json:"offset,omitempty"` // 从末尾向前的偏移（行数）
	Limit  int `json:"limit,omitempty"`  // 每页行数，默认 10
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

	logPath := filepath.Join(logging.ResolveLogDir(), "mindx.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
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
	paths := []string{
		filepath.Join(logDir, "mindx.log"),
		filepath.Join(logDir, "error.log"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("clear log %s: %w", p, err)
		}
	}

	return map[string]string{"status": "ok"}, nil
}
