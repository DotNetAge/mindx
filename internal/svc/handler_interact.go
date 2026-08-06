package svc

import (
	"context"
	"encoding/json"

	"github.com/DotNetAge/gort/pkg/gateway"
)

func (d *Daemon) handleMessageCancel(ctx context.Context, _ json.RawMessage) (any, error) {
	// 按 clientID 精确取消，避免一个客户端停止误杀其他客户端（多窗口/多设备）的执行
	clientID := gateway.ClientIDFromContext(ctx)
	if clientID != "" {
		d.logger.Info("message.cancel called", "client_id", clientID)
		d.cancelClientExecution(clientID)
		return map[string]string{"status": "ok"}, nil
	}
	// 异常路径（context 未注入 clientID）：兜底取消全部，保证停止能力不失效
	d.logger.Info("message.cancel called without client_id, cancelling all running executions")
	d.clientCancels.Range(func(key, value any) bool {
		if set, ok := value.(*clientCancelSet); ok {
			set.CancelAll()
		}
		d.clientCancels.Delete(key)
		return true
	})
	return map[string]string{"status": "ok"}, nil
}
