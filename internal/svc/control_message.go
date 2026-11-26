package svc

import (
	"regexp"
	"strings"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// permissionControlPattern 匹配授权决策魔术词（可带 :payload 后缀）。
// 这类文本是协议载荷而非用户提问：任何以「user 消息」为轮次边界的后端计算
// （用量归组、回退定位、重试提示）都必须跳过，否则会凭空多出一轮。
var permissionControlPattern = regexp.MustCompile(`^Permission(?:AllowSession|Allow|Deny)(?::\s*[0-9A-Za-z_-]+)?$`)

// isPermissionControlText 判断一段文本是否为授权决策魔术词。
func isPermissionControlText(text string) bool {
	return permissionControlPattern.MatchString(strings.TrimSpace(text))
}

// isRoundAnchorUserMessage 判断一条 user 消息能否作为轮次起点。
// 授权魔术词只是当前轮的控制应答，不是新提问。
func isRoundAnchorUserMessage(m goharnesssession.Message) bool {
	// 轮锚点统一判定：排除工具产出的视觉附件（user 角色但非用户提问）与
	// 授权决策的魔术词控制消息。
	return m.IsRoundAnchor() && !isPermissionControlText(m.Content)
}
