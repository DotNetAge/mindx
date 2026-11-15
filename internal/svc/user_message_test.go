package svc

import (
	"testing"
)

// TestParseUserMessagePayload 覆盖 user.message 载荷解析的三类真实输入。
//
// 回归点（P0）：图片收集曾写在 json.Unmarshal 之前，payload.Images 恒为零值，
// 于是图片-only 消息被当成空消息丢弃、带文字的图片消息以空图片集进模型。
func TestParseUserMessagePayload(t *testing.T) {
	t.Run("纯图片消息不得被丢弃", func(t *testing.T) {
		payload, images, err := parseUserMessagePayload([]byte(
			`{"images":[{"path":"/tmp/shot.png","media_type":"image/png"}]}`))
		if err != nil {
			t.Fatalf("图片-only 消息应被接受，得到错误: %v", err)
		}
		if payload.Text != "" {
			t.Fatalf("文本应为空，得到 %q", payload.Text)
		}
		if len(images) != 1 || images[0].Path != "/tmp/shot.png" || images[0].MediaType != "image/png" {
			t.Fatalf("图片引用未正确收集: %+v", images)
		}
	})

	t.Run("图文消息图片随文本一起返回", func(t *testing.T) {
		payload, images, err := parseUserMessagePayload([]byte(
			`{"text":"看这个报错","session_id":"s1","agent_name":"dev","images":[{"path":"/tmp/a.png"},{"path":"/tmp/b.jpg","media_type":"image/jpeg"}]}`))
		if err != nil {
			t.Fatalf("图文消息应被接受，得到错误: %v", err)
		}
		if payload.Text != "看这个报错" || payload.SessionID != "s1" || payload.AgentName != "dev" {
			t.Fatalf("文本字段解析异常: %+v", payload)
		}
		if len(images) != 2 {
			t.Fatalf("应收集 2 张图片，得到 %d", len(images))
		}
	})

	t.Run("空路径图片被跳过", func(t *testing.T) {
		_, images, err := parseUserMessagePayload([]byte(
			`{"text":"hi","images":[{"path":"","media_type":"image/png"}]}`))
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if len(images) != 0 {
			t.Fatalf("空路径图片应被跳过，得到 %+v", images)
		}
	})

	t.Run("无文字无图片丢弃", func(t *testing.T) {
		if _, _, err := parseUserMessagePayload([]byte(`{"session_id":"s1"}`)); err == nil {
			t.Fatal("既无文字也无图片的消息应返回错误")
		}
		if _, _, err := parseUserMessagePayload(nil); err == nil {
			t.Fatal("空载荷应返回错误")
		}
	})

	t.Run("畸形 JSON 报错", func(t *testing.T) {
		if _, _, err := parseUserMessagePayload([]byte(`{"text":`)); err == nil {
			t.Fatal("畸形 JSON 应返回错误")
		}
	})
}
