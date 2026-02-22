package channels

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

func init() {
	Register("imessage", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewIMessageChannel(&config.IMessageConfig{
			Enabled:    getBoolFromConfig(cfg, "enabled", false),
			IMsgPath:   getStringFromConfigWithDefault(cfg, "imsg_path", "/usr/local/bin/imsg"),
			Region:     getStringFromConfigWithDefault(cfg, "region", "CN"),
			Debounce:   getStringFromConfigWithDefault(cfg, "debounce", "250ms"),
			WatchSince: int64(getIntFromConfig(cfg, "watch_since", 0)),
		}), nil
	})
}

type IMessageChannel struct {
	mu          sync.RWMutex
	isRunning   bool
	config      *config.IMessageConfig
	onMessage   func(ctx context.Context, msg *entity.IncomingMessage)
	cancelWatch context.CancelFunc
	startTime   time.Time
	totalMsg    int
	lastMsgTime time.Time
}

type IMsgChat struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Identifier    string `json:"identifier"`
	Service       string `json:"service"`
	LastMessageAt string `json:"last_message_at"`
}

type IMsgMessage struct {
	ID                  int           `json:"id"`
	ChatID              int           `json:"chat_id"`
	GUID                string        `json:"guid"`
	ReplyToGUID         string        `json:"reply_to_guid,omitempty"`
	DestinationCallerID string        `json:"destination_caller_id,omitempty"`
	Sender              string        `json:"sender"`
	IsFromMe            bool          `json:"is_from_me"`
	Text                string        `json:"text"`
	CreatedAt           string        `json:"created_at"`
	Attachments         []interface{} `json:"attachments,omitempty"`
	Reactions           []interface{} `json:"reactions,omitempty"`
}

func NewIMessageChannel(cfg *config.IMessageConfig) *IMessageChannel {
	return &IMessageChannel{
		config: cfg,
	}
}

func (c *IMessageChannel) Type() entity.ChannelType {
	return entity.ChannelTypeIMessage
}

func (c *IMessageChannel) Name() string {
	return "iMessage"
}

func (c *IMessageChannel) Description() string {
	return "iMessage Channel (using steipete/imsg)"
}

func (c *IMessageChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

func (c *IMessageChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return nil
	}

	if _, err := exec.LookPath(c.config.IMsgPath); err != nil {
		return fmt.Errorf("imsg not found at %s: %w", c.config.IMsgPath, err)
	}

	c.isRunning = true
	c.startTime = time.Now()

	watchCtx, cancel := context.WithCancel(ctx)
	c.cancelWatch = cancel

	go c.startWatching(watchCtx)

	return nil
}

func (c *IMessageChannel) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return nil
	}

	if c.cancelWatch != nil {
		c.cancelWatch()
	}

	c.isRunning = false
	return nil
}

func (c *IMessageChannel) SetOnMessage(handler func(ctx context.Context, msg *entity.IncomingMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = handler
}

func (c *IMessageChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("imessage").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *IMessageChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("iMessage channel is not running")
	}

	args := []string{
		"send",
		"--to", msg.SessionID,
		"--text", msg.Content,
		"--region", c.config.Region,
	}

	cmd := exec.CommandContext(ctx, c.config.IMsgPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send iMessage: %w", err)
	}

	c.mu.Lock()
	c.totalMsg++
	c.lastMsgTime = time.Now()
	c.mu.Unlock()

	return nil
}

func (c *IMessageChannel) GetStatus() *entity.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := &entity.ChannelStatus{
		Type:          c.Type(),
		Name:          c.Name(),
		Description:   c.Description(),
		Running:       c.isRunning,
		TotalMessages: int64(c.totalMsg),
	}

	if !c.startTime.IsZero() {
		status.StartTime = &c.startTime
	}

	if !c.lastMsgTime.IsZero() {
		status.LastMessageTime = &c.lastMsgTime
	}

	return status
}

func (c *IMessageChannel) startWatching(ctx context.Context) {
	args := []string{"watch", "--json"}
	if c.config.WatchSince > 0 {
		args = append(args, "--since-rowid", strconv.FormatInt(c.config.WatchSince, 10))
	}
	args = append(args, "--debounce", c.config.Debounce)

	cmd := exec.CommandContext(ctx, c.config.IMsgPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	if err := cmd.Start(); err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			if line == "" {
				continue
			}

			var msg IMsgMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			if !msg.IsFromMe && msg.Text != "" {
				c.handleIncomingMessage(ctx, &msg)
			}
		}
	}

	scanner.Err()
	cmd.Wait()
}

func (c *IMessageChannel) handleIncomingMessage(ctx context.Context, imsg *IMsgMessage) {
	c.mu.RLock()
	handler := c.onMessage
	c.mu.RUnlock()

	if handler == nil {
		return
	}

	sender := &entity.MessageSender{
		ID:   imsg.Sender,
		Name: imsg.Sender,
		Type: "user",
	}

	createdAt, err := time.Parse(time.RFC3339, imsg.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}

	msg := &entity.IncomingMessage{
		ChannelID:   "imessage",
		ChannelName: "iMessage",
		MessageID:   strconv.Itoa(imsg.ID),
		Sender:      sender,
		Content:     imsg.Text,
		ContentType: "text",
		Timestamp:   createdAt,
		Metadata: map[string]interface{}{
			"chat_id": imsg.ChatID,
			"guid":    imsg.GUID,
			"sender":  imsg.Sender,
		},
	}

	msg.SessionID = imsg.Sender

	c.mu.Lock()
	c.totalMsg++
	c.lastMsgTime = time.Now()
	c.mu.Unlock()

	handler(ctx, msg)
}
