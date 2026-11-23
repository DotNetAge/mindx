package svc

import (
	"encoding/json"
	"strings"

	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
)

// AskUser 的参数 schema 只声明 options 为 array，没有声明 items 类型。
// 部分模型会按“选项对象”来理解，传 [{label, description}]，而 goharness
// 的参数解析只接受字符串数组，结果 options 被整体丢弃，前端只剩自由输入。
//
// 这里不改 goharness 的 schema，而是在事件广播前用本轮已落盘的原始工具调用
// 参数补一次：字符串项直接用，对象项取 label/text/value/name 作为选项文案。
// multiSelect 也以原始参数为准，避免只认 camelCase 的旧解析漏掉 snake_case。
func enrichAskUserPendingFromToolCall(
	sess *goharnesssession.Session,
	data *goharnessevents.AskUserPendingData,
) {
	if sess == nil || data == nil || data.ToolCallID == "" {
		return
	}
	args, ok := findAskUserToolArgs(sess.All(), data.ToolCallID)
	if !ok {
		return
	}
	applyAskUserToolArgs(data, args)
}

func findAskUserToolArgs(messages []goharnesssession.Message, toolCallID string) (map[string]any, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		for _, call := range msg.ToolCalls {
			if call.ID != toolCallID || !strings.EqualFold(call.Name, "AskUser") {
				continue
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				return nil, false
			}
			return args, true
		}
	}
	return nil, false
}

func applyAskUserToolArgs(data *goharnessevents.AskUserPendingData, args map[string]any) {
	if len(data.Questions) == 0 {
		return
	}
	q := &data.Questions[0]

	if len(q.Options) == 0 {
		q.Options = normalizeAskUserOptions(args["options"])
	}

	if v, ok := args["multiSelect"].(bool); ok {
		q.MultiSelect = v
	}
	if v, ok := args["multi_select"].(bool); ok {
		q.MultiSelect = v
	}
}

func normalizeAskUserOptions(raw any) []string {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case string:
			if text := strings.TrimSpace(v); text != "" {
				out = append(out, text)
			}
		case map[string]any:
			for _, key := range []string{"label", "text", "value", "name"} {
				if text, ok := v[key].(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, strings.TrimSpace(text))
					break
				}
			}
		}
	}
	return out
}

// askUserEventSession 返回承载本次工具调用的会话：主会话直接用传入会话，
// 子会话提问则从活跃会话表取子会话，拿不到时退回主会话（至少不丢事件）。
func (d *Daemon) askUserEventSession(
	sess *goharnesssession.Session,
	sessionID string,
) *goharnesssession.Session {
	if sessionID == "" || sess == nil || sessionID == sess.ID() {
		return sess
	}
	if val, ok := d.activeSessions.Load(sessionID); ok {
		if sub, ok := val.(*goharnesssession.Session); ok {
			return sub
		}
	}
	return sess
}
