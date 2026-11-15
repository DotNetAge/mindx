package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/commands"
	"github.com/stretchr/testify/require"
)

// captureRegisteredCommands 用记录器跑一遍生产注册路径，返回 name → handler。
func captureRegisteredCommands(t *testing.T, d *Daemon) map[string]func(*gateway.CommandContext) (any, error) {
	t.Helper()
	got := make(map[string]func(*gateway.CommandContext) (any, error))
	registerBuiltinCommandsWith(func(meta gateway.CommandMeta, h func(*gateway.CommandContext) (any, error)) {
		got[meta.Name] = h
	}, nil, d)
	return got
}

// invokeCommand 调用命令 handler，把 panic 转成错误。
// 未包装的命令会真的执行到 handler（nil 依赖下多数直接 panic），
// 转成错误后失败信息能明确指出「命令被执行了」，而不是一串栈。
func invokeCommand(h func(*gateway.CommandContext) (any, error), ctx *gateway.CommandContext) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("命令 handler 被直接执行（缺少设备接入闸）: %v", r)
		}
	}()
	return h(ctx)
}

// TestBuiltinCommandsAreGated 每一条内建斜杠命令都必须经过设备接入闸。
//
// 回归点（P0）：命令经 gateway.RegisterCommand 直接写 method 表，不经过
// RegisterMethod 的 guard 包装。私网任意设备曾可免凭据直接发
// `job.add`，把任意指令落库成定时任务，到点由带完整工具链的 Agent 执行。
//
// 判定用「闸不可用的 daemon」：闸未装配时全部命令都必须被拒。漏包装的命令
// 会真的执行到 handler（nil 依赖下多数直接 panic 或返回业务错误），断言文案
// 即失败。
func TestBuiltinCommandsAreGated(t *testing.T) {
	handlers := captureRegisteredCommands(t, &Daemon{})

	metas := commands.New().Metas()
	require.NotEmpty(t, metas, "内建命令不应为空")
	require.Len(t, handlers, len(metas), "注册的命令数量应与元数据一致（不得有命令绕过注册路径）")

	for _, meta := range metas {
		h, ok := handlers[meta.Name]
		require.True(t, ok, "命令 %q 未注册", meta.Name)

		// 未验证客户端（任意 clientID）
		res, err := invokeCommand(h, &gateway.CommandContext{
			ClientID: "unverified-client",
			Args:     "@agent new rm -rf / expr=* * * * *",
		})
		require.Error(t, err, "命令 %q 未经过设备接入闸", meta.Name)
		require.Contains(t, err.Error(), "安全校验还没完成", "命令 %q 的拒绝原因不是设备接入闸", meta.Name)
		require.Nil(t, res, "命令 %q 在未验证连接上产生了结果", meta.Name)
	}
}

// TestRPCCommandsOpenWhitelistUnchanged 免握手白名单必须保持最小。
//
// 白名单里的方法完全不经设备接入验证，等于对任何能建立连接的设备开放。
// 新增条目必须是一次有意识的决定，因此用测试钉住集合：一旦有人往里加东西，
// 这里先红。
func TestRPCCommandsOpenWhitelistUnchanged(t *testing.T) {
	want := map[string]bool{
		"device.handshake":  true,
		"server.version":    true,
		"channel.bootstrap": true,
		"channel.verify":    true,
	}
	require.Equal(t, want, openMethods, "免握手白名单发生变化，请确认新增方法确实无需接入验证")
}

// TestRPCHandlersAreGated 所有非白名单 RPC 方法都必须先过设备接入闸。
//
// 用「闸不可用的 daemon」作为判定条件：闸未装配时 deviceGateAuthorized
// 恒为 false，任何方法都不得进入业务逻辑（P0-2 的 fail-open 回归点）。
func TestRPCHandlersAreGated(t *testing.T) {
	reg := NewRPCHandlerRegistry(&Daemon{})
	handlers := reg.handlers()
	require.NotEmpty(t, handlers, "RPC 方法表不应为空")

	// 业务 handler 一旦被调用就会在 nil 依赖上出错，用哨兵确保「没被调用」
	for method, h := range handlers {
		if openMethods[method] {
			continue
		}
		guarded := reg.guard(method, h)
		res, err := guarded(context.Background(), json.RawMessage(`{}`))
		require.Error(t, err, "方法 %q 未经过设备接入闸", method)
		require.Contains(t, err.Error(), "安全校验还没完成", "方法 %q 的拒绝原因不是设备接入闸", method)
		require.Nil(t, res, "方法 %q 在未验证连接上产生了结果", method)
	}
}
