package svc

import (
	"encoding/json"
	"errors"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// userMessagePayload 是 user.message 通知的数据载荷。
type userMessagePayload struct {
	Text      string `json:"text"`
	SessionID string `json:"session_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Images    []struct {
		Path      string `json:"path"`
		MediaType string `json:"media_type"`
	} `json:"images,omitempty"`
}

// errEmptyUserMessage 表示消息既没有文字也没有图片，属于无内容消息。
var errEmptyUserMessage = errors.New("消息既没有文字也没有图片")

// parseUserMessagePayload 解析 user.message 载荷，返回文本相关字段与图片引用列表。
//
// 附加图片由前端把粘贴/上传的图片落盘到会话临时目录后，以 {path, media_type}
// 引用随消息送来；此处只收路径引用，读文件转 base64 延迟到组装模型消息时。
//
// 两条纪律写在函数签名里：
//   - 反序列化先于任何字段读取。payload 在反序列化前是零值，先收集 images
//     会恒得空切片——纯图片消息被判「空消息」丢弃，带文字的图片消息也会以
//     空图片集进入模型请求。
//   - 图片-only 消息合法（Content 为空但有视觉内容）。只有文字与图片同时
//     为空才返回错误，由调用方丢弃。
func parseUserMessagePayload(data []byte) (userMessagePayload, []goharnesssession.ImageBlock, error) {
	var payload userMessagePayload
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return userMessagePayload{}, nil, err
		}
	}

	images := make([]goharnesssession.ImageBlock, 0, len(payload.Images))
	for _, img := range payload.Images {
		if img.Path == "" {
			continue
		}
		images = append(images, goharnesssession.ImageBlock{
			Path:      img.Path,
			MediaType: img.MediaType,
		})
	}
	if payload.Text == "" && len(images) == 0 {
		return payload, images, errEmptyUserMessage
	}
	return payload, images, nil
}
