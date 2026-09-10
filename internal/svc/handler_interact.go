package svc

import (
	"context"
	"encoding/json"

	"github.com/DotNetAge/gort/pkg/gateway"
)

// handleMessageCancel 处理停止指令。
//
// 取消语义（按会话隔离）：
//   - 携带 session_id：仅取消该会话的执行（运行中与排队中），并级联强停
//     其派生的全部运行中 SubAgent（子执行循环运行在独立 Background ctx 上，
//     不随主 exec ctx 取消，需经 Runtime.CancelSubAgents 显式强停）。
//     同一客户端其它 Tab / 其它会话的执行不受影响。
//   - 未携带 session_id：批量取消该客户端全部执行并强停各自派生的
//     SubAgent（旧语义兜底，供未升级的调用方与异常恢复路径使用）。
func (d *Daemon) handleMessageCancel(ctx context.Context, params json.RawMessage) (any, error) {
	// 解析可选的 session_id 参数：{"session_id": "..."}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}

	// 按 clientID 精确取消，避免一个客户端停止误杀其他客户端（多窗口/多设备）的执行
	clientID := gateway.ClientIDFromContext(ctx)
	if clientID != "" {
		if req.SessionID != "" {
			var cancelled int
			if v, ok := d.clientCancels.Load(clientID); ok {
				if set, ok := v.(*clientCancelSet); ok {
					cancelled = set.CancelSession(req.SessionID)
				}
			}
			subStopped := d.cancelSubAgents(req.SessionID)
			d.logger.Info("message.cancel called for session",
				"client_id", clientID, "session_id", req.SessionID,
				"cancelled", cancelled, "sub_agents_stopped", subStopped)
			return map[string]string{"status": "ok"}, nil
		}
		d.logger.Info("message.cancel called", "client_id", clientID)
		d.cancelClientExecution(clientID)
		return map[string]string{"status": "ok"}, nil
	}
	// 异常路径（context 未注入 clientID）：兜底取消全部，保证停止能力不失效
	d.logger.Info("message.cancel called without client_id, cancelling all running executions")
	d.clientCancels.Range(func(key, value any) bool {
		if set, ok := value.(*clientCancelSet); ok {
			for _, sid := range set.CancelAll() {
				if n := d.cancelSubAgents(sid); n > 0 {
					d.logger.Info("级联强停子代理", "session_id", sid, "count", n)
				}
			}
		}
		d.clientCancels.Delete(key)
		return true
	})
	return map[string]string{"status": "ok"}, nil
}
