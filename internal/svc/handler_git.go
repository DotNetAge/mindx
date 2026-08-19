package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// handleGitClone 克隆 Git 仓库到目标目录（dir 为父目录，实际克隆到 dir/<仓库名>）。
// 返回完整克隆路径，供前端直接切换为工作区。
func (d *Daemon) handleGitClone(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.GitCloneParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.URL == "" {
		return nil, fmt.Errorf("仓库地址不能为空")
	}
	if p.Dir == "" {
		return nil, fmt.Errorf("目标目录不能为空")
	}

	parentDir, err := filepath.Abs(filepath.Clean(p.Dir))
	if err != nil {
		return nil, fmt.Errorf("invalid dir: %w", err)
	}
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("无法创建目标目录: %w", err)
	}

	// 从 URL 推断仓库目录名（支持 .git 后缀与 :/ 分隔）
	trimmed := strings.TrimSuffix(p.URL, "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	repoName := trimmed[strings.LastIndex(trimmed, "/")+1:]
	if repoName == "" {
		return nil, fmt.Errorf("无法从仓库地址推断目录名")
	}

	cloneDir := filepath.Join(parentDir, repoName)
	// 目标已存在且非空时拒绝，避免覆盖
	if info, statErr := os.Stat(cloneDir); statErr == nil && info.IsDir() {
		entries, readErr := os.ReadDir(cloneDir)
		if readErr == nil && len(entries) > 0 {
			return nil, fmt.Errorf("目标目录已存在且非空: %s", cloneDir)
		}
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--progress", p.URL, cloneDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if cloneCtx.Err() != nil {
			return nil, fmt.Errorf("克隆超时")
		}
		msg := strings.TrimSpace(string(output))
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		return nil, fmt.Errorf("克隆失败: %s", msg)
	}

	return map[string]any{"dir": cloneDir}, nil
}
