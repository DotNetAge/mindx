package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/goharness/memory"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// MemoryDeps holds external dependencies for memory commands.
type MemoryDeps struct {
	SearchMemory func(ctx context.Context, params rpc.MemoryQueryParams) ([]memory.MemoryChunk, error)
}

var memoryDeps MemoryDeps

// SetMemoryDeps sets the dependencies for memory commands.
func SetMemoryDeps(deps MemoryDeps) {
	memoryDeps = deps
}

func registerMemoryCommands(r *Registry) {
	r.Register(Meta{
		Name:        "memory",
		Description: "记忆管理：/memory search <query> 搜索历史记忆",
		Category:    "agent",
		Scope:       gateway.ScopeRemote,
		Example:     "/memory search 用户偏好设置",
		Params:      "search <query> — 语义搜索历史记忆",
	}, handleMemoryCommand)
}

func handleMemoryCommand(ctx *gateway.CommandContext) (any, error) {
	args := strings.Fields(ctx.Args)
	if len(args) < 2 || args[0] != "search" {
		return nil, fmt.Errorf("用法: /memory search <query>\n示例: /memory search 用户偏好设置")
	}

	query := strings.Join(args[1:], " ")
	if memoryDeps.SearchMemory == nil {
		return nil, fmt.Errorf("memory command not configured")
	}

	results, err := memoryDeps.SearchMemory(context.Background(), rpc.MemoryQueryParams{
		Query: query,
		Limit: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未找到相关记忆")
	}

	// Build table for TUI
	rows := make([][]string, 0, len(results))
	for _, c := range results {
		summary := c.Summary
		if summary == "" {
			summary = "(无标题)"
		}
		if len(summary) > 60 {
			summary = summary[:60] + "..."
		}
		rows = append(rows, []string{
			c.ID[:8] + "...",
			summary,
			c.Timestamp.Format("01-02 15:04"),
		})
	}

	ctx.RespondWithType(gateway.RespTable, "记忆搜索结果", map[string]interface{}{
		"headers": []string{"ID", "摘要", "时间"},
		"rows":    rows,
	})

	// Return raw data for programmatic callers
	return results, nil
}
