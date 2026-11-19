package svc

// 头像写入校验（agent.update / team.update 共用）。
//
// icon 最终落到 frontmatter 并被界面当 <img src> 使用，所以只放行两种形态：
//   1. data:image/...;base64,... 内嵌图（头像库、手工上传都走这条）
//   2. 图标名（lucide:xxx 之类，由 MxIcon 解析）
// 拒绝 http(s)/file 等外链——界面会真的去加载，等于把追踪信标写进档案；
// 也拒绝裸 base64/杂串，避免把 frontmatter 撑坏或让界面渲染不出东西。

import (
	"fmt"
	"strings"
)

/** 头像字符串上限：256×256 JPEG 的 base64 约 40KB，留足余量后仍拒绝超大值 */
const maxIconBytes = 512 * 1024

/** 短图标名/emoji 的字节上限（既有数据里存在 "rocket" 这类裸名与 emoji，照旧放行） */
const maxIconToken = 32

func normalizeIcon(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "data:") {
		if !strings.HasPrefix(lower, "data:image/") || !strings.Contains(lower, ";base64,") {
			return "", fmt.Errorf("头像只接受 data:image/...;base64 内嵌图")
		}
		if len(v) > maxIconBytes {
			return "", fmt.Errorf("头像过大（%d 字节，上限 %d）", len(v), maxIconBytes)
		}
		return v, nil
	}
	// 唯一硬拦：带协议的外链（<img src> 会真的去加载，等于把追踪信标写进档案）。
	if strings.Contains(v, "://") {
		return "", fmt.Errorf("头像不接受外链（http/https/file）")
	}
	// 其余允许短 token：图标名（lucide:xxx / rocket）与 emoji，均为既有合法形态。
	if len(v) <= maxIconToken {
		return v, nil
	}
	return "", fmt.Errorf("头像必须为 data:image/...;base64 内嵌图、图标名或 emoji")
}
