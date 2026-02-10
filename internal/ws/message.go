// Package ws provides WebSocket server and client functionality for MAXAM chat
package ws

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// TypeChat is a regular chat message
	TypeChat MessageType = "chat"
	// TypeSystem is a system notification
	TypeSystem MessageType = "system"
	// TypeHistory is a batch of historical messages
	TypeHistory MessageType = "history"
	// TypeJoin is sent when a client joins
	TypeJoin MessageType = "join"
	// TypeLeave is sent when a client leaves
	TypeLeave MessageType = "leave"
)

// Message represents a WebSocket message
type Message struct {
	Type      MessageType `json:"type"`
	From      string      `json:"from"`
	To        string      `json:"to,omitempty"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewChatMessage creates a new chat message
func NewChatMessage(from, to, content string) *Message {
	return &Message{
		Type:      TypeChat,
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemMessage creates a new system message
func NewSystemMessage(content string) *Message {
	return &Message{
		Type:      TypeSystem,
		From:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewJoinMessage creates a new join notification
func NewJoinMessage(clientID string) *Message {
	return &Message{
		Type:      TypeJoin,
		From:      clientID,
		Content:   clientID + " joined",
		Timestamp: time.Now(),
	}
}

// NewLeaveMessage creates a new leave notification
func NewLeaveMessage(clientID string) *Message {
	return &Message{
		Type:      TypeLeave,
		From:      clientID,
		Content:   clientID + " left",
		Timestamp: time.Now(),
	}
}

// ToJSON serializes the message to JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ParseMessage deserializes a message from JSON
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// HistoryResponse wraps multiple messages for history delivery
type HistoryResponse struct {
	Type     MessageType `json:"type"`
	Messages []*Message  `json:"messages"`
}

// NewHistoryResponse creates a new history response
func NewHistoryResponse(messages []*Message) *HistoryResponse {
	return &HistoryResponse{
		Type:     TypeHistory,
		Messages: messages,
	}
}

// ToJSON serializes the history response to JSON
func (h *HistoryResponse) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}
