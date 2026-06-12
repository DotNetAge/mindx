package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
)

func main() {
	addr := "ws://localhost:1314/ws"
	c := gateway.NewClient(addr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "已连接到 %s\n", addr)

	// Register handler for ALL known event types (goharness aligned)
	types := []gateway.ResponseType{
		gateway.RespThinkingDelta,
		gateway.RespThinkingDone,
		gateway.RespMarkdown,
		gateway.RespText,
		gateway.RespToolUseDelta,
		gateway.RespToolExecStart,
		gateway.RespToolExecEnd,
		gateway.RespExecutionSummary,
		gateway.RespFinalAnswer,
		gateway.RespCycleEnd,
		gateway.RespForm,
		gateway.RespPermissionRequest,
		gateway.RespError,
		gateway.RespMaxTurnsReached,
	}
	for _, rt := range types {
		rt := rt
		c.OnResponse(rt, func(env *gateway.ResponseEnvelope, msg *gateway.Message) {
			fmt.Fprintf(os.Stderr, "[EVENT] type=%s session=%q title=%q data=%v\n",
				env.Type, env.SessionID, env.Title, env.Data)
		})
	}

	// Test RPC request/response
	if result, err := c.Call(ctx, "session.list", nil); err != nil {
		fmt.Fprintf(os.Stderr, "session.list RPC 失败: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "session.list RPC 成功: %s\n", string(result))
	}

	// Send a test message
	payload := map[string]string{"text": "hello"}
	fmt.Fprintf(os.Stderr, "发送消息: %v\n", payload)
	if err := c.Notify("user.message", payload); err != nil {
		fmt.Fprintf(os.Stderr, "发送失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "消息已发送，等待事件...\n")

	// Wait for events
	time.Sleep(30 * time.Second)
	fmt.Fprintf(os.Stderr, "超时退出\n")
}
