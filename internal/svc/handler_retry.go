package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// retryCancelClientID 是失败重试执行在 clientCancels 中的虚拟客户端键。
// RPC 入口不携带发起客户端 ID，取消登记到此集合；message.cancel 按会话
// 取消时按 sessionID 遍历集合，同样能覆盖本轮重试。
const retryCancelClientID = "__retry__"

// errRetryInFlight 会话已有执行在跑或排队，重试需等其结束。
var errRetryInFlight = errors.New("这一轮还在执行，先等它结束或停止后再重试。")

// errRetryNoRound 会话里没有可重试的轮次（无用户消息）。
var errRetryNoRound = errors.New("这个会话里没有可重试的轮次。")

// errRetrySuspended 会话处于挂起等待态（待授权 / 待回答提问），
// 应走对应的授权或回答通道恢复，不能按失败重试处理。
var errRetrySuspended = errors.New("会话正在等待授权或回答，不能按失败重试。")

// handleSessionRetryLast 失败重试：不截断、不删除既有上下文，基于当前
// 会话窗口原样重发上一次请求（AskBuilder.ReuseHistory 续跑模式）。
//
// 与旧的 session.truncate 重来路径的差别：保留本轮已完成的工具调用与
// 工具结果，不向会话追加新的 user 消息；失败那次 LLM 调用的半截产出本就
// 不会落库（goagent 出错分支不追加半截消息），上下文天然干净。
//
// 执行经会话串行队列异步展开，事件走广播链路；RPC 立即返回排队确认。
func (d *Daemon) handleSessionRetryLast(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionRetryLastParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if d.sessionQueueFor(p.SessionID).Busy() {
		return nil, errRetryInFlight
	}

	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("打开会话失败：%w", err)
	}

	// 挂起态由授权 / 回答通道恢复，续跑会跳过魔法词消费，必须在此拦截。
	if sess.PendingPermission() != nil || sess.PeekPendingAskUser() != nil {
		return nil, errRetrySuspended
	}

	// 定位最后一条 user 消息：作为计量埋点与轮次日志的提示文案；
	// 会话里没有 user 消息时无可重试对象。
	msgs := sess.All()
	hint := ""
	hasUser := false
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			hasUser = true
			hint = msgs[i].Content
			break
		}
	}
	if !hasUser {
		return nil, errRetryNoRound
	}

	agentName := d.app.CurrentAgentName()
	if meta, metaErr := d.getSessionMeta(p.SessionID); metaErr == nil && meta != nil && meta.AgentName != "" {
		agentName = meta.AgentName
	}

	d.logger.Info("retry last turn: resume on existing context",
		"session_id", p.SessionID, "agent", agentName)

	d.runBackgroundTurn(p.SessionID, agentName, "", hint, retryCancelClientID, func(error) {}, true)

	return map[string]any{
		"session_id": p.SessionID,
		"resumed":    true,
	}, nil
}
