package channel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockChannel(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	assert.Equal(t, "wechat", ch.Name())
	assert.Equal(t, ChannelTypeWeChat, ch.Type())
	assert.False(t, ch.IsRunning())
	assert.Equal(t, StatusStopped, ch.GetStatus())
}

func TestMockChannel_Start(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	err := ch.Start(context.Background(), handler)
	require.NoError(t, err)
	assert.True(t, ch.IsRunning())
	assert.Equal(t, StatusRunning, ch.GetStatus())
}

func TestMockChannel_Start_AlreadyRunning(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	err := ch.Start(context.Background(), handler)
	require.NoError(t, err)

	err = ch.Start(context.Background(), handler)
	assert.Equal(t, ErrChannelAlreadyRunning, err)
}

func TestMockChannel_Stop(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	err := ch.Start(context.Background(), handler)
	require.NoError(t, err)

	err = ch.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, ch.IsRunning())
	assert.Equal(t, StatusStopped, ch.GetStatus())
}

func TestMockChannel_Stop_NotRunning(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	err := ch.Stop(context.Background())
	assert.Equal(t, ErrChannelNotRunning, err)
}

func TestMockChannel_SendMessage(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	msg := NewMessage("msg_001", "wechat", DirectionOutbound, UserInfo{}, "test", MessageTypeText)

	err := ch.SendMessage(context.Background(), msg)
	require.NoError(t, err)

	messages := ch.GetMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, msg, messages[0])
}

func TestMockChannel_SendMessage_Error(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)
	expectedErr := errors.New("send error")
	ch.SetSendError(expectedErr)

	msg := NewMessage("msg_001", "wechat", DirectionOutbound, UserInfo{}, "test", MessageTypeText)

	err := ch.SendMessage(context.Background(), msg)
	assert.Equal(t, expectedErr, err)
}

func TestMockChannel_SimulateMessage(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	receivedMsg := (*Message)(nil)
	handler := func(ctx context.Context, msg *Message) error {
		receivedMsg = msg
		return nil
	}

	err := ch.Start(context.Background(), handler)
	require.NoError(t, err)

	msg := NewMessage("msg_001", "wechat", DirectionInbound, UserInfo{}, "test", MessageTypeText)

	err = ch.SimulateMessage(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, msg, receivedMsg)
}

func TestMockChannel_SimulateMessage_NotRunning(t *testing.T) {
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	msg := NewMessage("msg_001", "wechat", DirectionInbound, UserInfo{}, "test", MessageTypeText)

	err := ch.SimulateMessage(context.Background(), msg)
	assert.Equal(t, ErrChannelNotRunning, err)
}

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	assert.NotNil(t, reg)
	assert.Equal(t, 0, reg.Count())
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	err := reg.Register(ch)
	require.NoError(t, err)
	assert.Equal(t, 1, reg.Count())

	registered, ok := reg.Get("wechat")
	assert.True(t, ok)
	assert.Equal(t, ch, registered)
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	reg := NewRegistry()
	ch1 := NewMockChannel("wechat", ChannelTypeWeChat)
	ch2 := NewMockChannel("wechat", ChannelTypeWeChat)

	err := reg.Register(ch1)
	require.NoError(t, err)

	err = reg.Register(ch2)
	assert.Equal(t, ErrChannelAlreadyExists, err)
}

func TestRegistry_Unregister(t *testing.T) {
	reg := NewRegistry()
	ch := NewMockChannel("wechat", ChannelTypeWeChat)

	err := reg.Register(ch)
	require.NoError(t, err)

	reg.Unregister("wechat")
	assert.Equal(t, 0, reg.Count())

	_, ok := reg.Get("wechat")
	assert.False(t, ok)
}

func TestRegistry_GetAll(t *testing.T) {
	reg := NewRegistry()

	ch1 := NewMockChannel("wechat", ChannelTypeWeChat)
	ch2 := NewMockChannel("dingtalk", ChannelTypeDingTalk)

	err := reg.Register(ch1)
	require.NoError(t, err)
	err = reg.Register(ch2)
	require.NoError(t, err)

	channels := reg.GetAll()
	assert.Len(t, channels, 2)
}

func TestRegistry_Count(t *testing.T) {
	reg := NewRegistry()

	assert.Equal(t, 0, reg.Count())

	ch1 := NewMockChannel("wechat", ChannelTypeWeChat)
	err := reg.Register(ch1)
	require.NoError(t, err)
	assert.Equal(t, 1, reg.Count())

	ch2 := NewMockChannel("dingtalk", ChannelTypeDingTalk)
	err = reg.Register(ch2)
	require.NoError(t, err)
	assert.Equal(t, 2, reg.Count())
}
