package core

import (
	"mindx/internal/entity"
	"context"
)

// Channel 通信通道接口
// 所有通信方式 (Web, Terminal, Hook 等) 都需要实现此接口
type Channel interface {
	// Name 返回 Channel 名称 (如 "web", "terminal", "feishu" 等)
	Name() string

	// Type 返回 Channel 类型
	Type() entity.ChannelType

	// Description 返回 Channel 描述
	Description() string

	// Start 启动 Channel
	// ctx 用于控制 Channel 生命周期,当 ctx 被取消时应停止运行
	Start(ctx context.Context) error

	// Stop 停止 Channel,释放资源
	Stop() error

	// IsRunning 返回 Channel 是否正在运行
	IsRunning() bool

	// OnMessage 设置消息接收回调
	// 当 Channel 接收到消息时会调用此回调
	SetOnMessage(callback func(ctx context.Context, msg *entity.IncomingMessage))

	// SendMessage 发送消息到 Channel
	// 用于向用户发送响应
	SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error

	// GetStatus 获取 Channel 当前状态
	GetStatus() *entity.ChannelStatus
}




