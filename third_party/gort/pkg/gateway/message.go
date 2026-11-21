package gateway

import (
	"encoding/json"
	"time"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

// Message wraps a channel.Message for use in the gateway layer.
// It adds gateway-specific fields while preserving the original channel message data.
type Message struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	SessionID   string    `json:"session_id"`
	ChannelID   string    `json:"channel_id,omitempty"`
	Direction   Direction `json:"direction"`
	Data        []byte    `json:"data,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewMessage creates a new gateway Message with the given fields.
// Timestamp is automatically set to the current time.
func NewMessage(id, clientID, sessionID, channelID string, direction Direction, data []byte, contentType string) *Message {
	return &Message{
		ID:          id,
		ClientID:    clientID,
		SessionID:   sessionID,
		ChannelID:   channelID,
		Direction:   direction,
		Data:        data,
		ContentType: contentType,
		Timestamp:   time.Now(),
	}
}

func (m *Message) Text() string {
	return string(m.Data)
}

func (m *Message) IsSuccess() bool {
	return len(m.Data) > 0
}

func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func MessageFromJSON(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (m *Message) WithClientID(clientID string) *Message {
	m.ClientID = clientID
	return m
}

func (m *Message) WithSessionID(sessionID string) *Message {
	m.SessionID = sessionID
	return m
}

func (m *Message) WithContentType(ct string) *Message {
	m.ContentType = ct
	return m
}

func (m *Message) WithDirection(d Direction) *Message {
	m.Direction = d
	return m
}
