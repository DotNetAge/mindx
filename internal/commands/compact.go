package commands

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// CompactDeps holds external dependencies for the compact command.
type CompactDeps struct {
	CompactSession func(ctx context.Context, params rpc.SessionCompactParams) (map[string]any, error)
}

var compactDeps CompactDeps

// SetCompactDeps sets the dependencies for the compact command.
func SetCompactDeps(deps CompactDeps) {
	compactDeps = deps
}

func registerCompactCommands(r *Registry) {
	r.Register(Meta{
		Name:        "compact",
		Description: "手动触发当前会话的强制压缩。将当前上下文窗口的对话压缩为摘要并释放 Token 空间。",
		Category:    "session",
		Scope:       gateway.ScopeRemote,
		Example:     "/compact — 将当前上下文压缩为摘要",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleCompact(ctx)
	})
}

func handleCompact(ctx *gateway.CommandContext) (any, error) {
	if compactDeps.CompactSession == nil {
		return nil, fmt.Errorf("compact command not configured")
	}

	if ctx.SessionID == "" {
		return nil, fmt.Errorf("当前没有活跃会话，请先开始一个对话")
	}

	result, err := compactDeps.CompactSession(context.Background(), rpc.SessionCompactParams{
		SessionID: ctx.SessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("压缩失败：%w", err)
	}

	windowTokens, _ := result["window_tokens"].(float64)
	maxWindowSize, _ := result["max_window_size"].(float64)
	usageRatio, _ := result["usage_ratio"].(float64)

	summary := fmt.Sprintf("会话压缩完成！当前上下文：%d / %d tokens（%.1f%%）",
		int64(windowTokens), int64(maxWindowSize), usageRatio*100)

	ctx.RespondWithType(gateway.RespMarkdown, "压缩结果", summary)
	return result, nil
}
