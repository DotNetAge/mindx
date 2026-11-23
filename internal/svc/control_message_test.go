package svc

import (
	"testing"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// 授权魔法词不是轮次起点：用量归组、回退定位、重试提示都依赖这条判断。
func TestRoundAnchorSkipsPermissionControl(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{"PermissionAllow", false},
		{"PermissionAllowSession", false},
		{"PermissionDeny", false},
		{"PermissionAllow: call-1", false},
		{"  PermissionDeny: abc  ", false},
		{"帮我改文件", true},
		{"PermissionAllowance", true},
		{"Permission", true},
	}
	for _, c := range cases {
		m := goharnesssession.Message{Role: "user", Content: c.content}
		if got := isRoundAnchorUserMessage(m); got != c.want {
			t.Errorf("content %q: isRoundAnchorUserMessage = %v, want %v", c.content, got, c.want)
		}
	}
	// 非 user 消息永远不是轮次锚点。
	if isRoundAnchorUserMessage(goharnesssession.Message{Role: "tool", Content: "随便"}) {
		t.Error("tool message must not be a round anchor")
	}
}
