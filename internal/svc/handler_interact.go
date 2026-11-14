package svc

import (
	"context"
	"encoding/json"
	"strings"

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
			// 按会话取消：遍历全部客户端集合，而不是只查当前连接。
			//
			// 执行登记在「发起消息的那条连接」名下，而停止指令可能来自另一条
			// 连接——断连重连后拿到的是新 clientID，或用户在另一个窗口（chatpop）
			// 点停止。只查当前 clientID 必然 cancelled=0 而执行继续。
			// 会话全局唯一且由 sessionQueue 串行执行，按会话取消不会波及
			// 其它会话的执行。
			//
			// scheduled / harvest 两个虚拟键本身就是集合，Range 天然覆盖，
			// 无需再单独特查。
			var cancelled int
			d.clientCancels.Range(func(key, value any) bool {
				set, ok := value.(*clientCancelSet)
				if !ok {
					return true
				}
				cancelled += set.CancelSession(req.SessionID)
				// 回收断连迁移遗留的空壳集合：其中的执行已全部结束，
				// 留着会随重连次数累积。
				if ks, ok := key.(string); ok &&
					strings.HasPrefix(ks, detachedCancelPrefix) && set.Empty() {
					d.clientCancels.Delete(ks)
				}
				return true
			})
			// 级联强停该会话派生的整棵子代理树（含成员再派工的孙代理），
			// 使用户在负责人会话点一次停止即可停下整个团队任务。
			subStopped := d.cancelSubAgents(req.SessionID)
			// 执行树被实际取消/强停：挂起等待随执行消亡，解除挂起登记（气球熄灭）。
			// 仅在实际发生取消时清除——对未在执行、仅挂起等待的会话，停止指令
			// 无可取消对象，挂起登记保持（Agent 仍在等待用户回应）。
			if cancelled > 0 || subStopped > 0 {
				d.clearSessionBlock(req.SessionID)
			}
			d.logger.Info("message.cancel called for session",
				"client_id", clientID, "session_id", req.SessionID,
				"cancelled", cancelled, "sub_agents_stopped", subStopped)
			// 返回实际停止数量：前端据此决定是否保留本地乐观标记。
			// cancelled=0 且 subStopped=0 说明没有任何执行被停，
			// 前端不能把 busy 置否（否则执行继续推事件会把 busy 重新点亮，
			// 表现为「打断一下又继续跑」）。
			return map[string]any{
				"status":             "ok",
				"cancelled":          cancelled,
				"sub_agents_stopped": subStopped,
			}, nil
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
				// 执行树被强停：解除挂起登记（与按会话取消同语义）
				d.clearSessionBlock(sid)
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
