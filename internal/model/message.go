package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Role represents the sender of a message
type Role string

const (
	RoleHuman     Role = "human"
	RoleAssistant Role = "assistant"
)

// ToolCall represents a tool invocation by the assistant
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Result    string          `json:"result,omitempty"`
}

// Message represents a single message in a conversation thread
type Message struct {
	ID              string     `json:"id"`
	Role            Role       `json:"role"`
	Content         string     `json:"content"`
	Timestamp       time.Time  `json:"timestamp"`
	ToolCalls       []ToolCall `json:"tool_calls,omitempty"`
	GitHashAfter    string     `json:"git_hash_after,omitempty"`
	ParentMessageID string     `json:"parent_message_id,omitempty"`
}

// ComputeHash generates a SHA256 hash for the message based on its content and parent
func (m *Message) ComputeHash() string {
	h := sha256.New()

	// Include parent hash in the chain
	h.Write([]byte(m.ParentMessageID))
	h.Write([]byte(string(m.Role)))
	h.Write([]byte(m.Content))
	h.Write([]byte(m.Timestamp.Format(time.RFC3339Nano)))

	// Include tool calls if present
	if len(m.ToolCalls) > 0 {
		toolCallsJSON, _ := json.Marshal(m.ToolCalls)
		h.Write(toolCallsJSON)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// NewMessage creates a new message with computed hash
func NewMessage(role Role, content string, parentID string, toolCalls []ToolCall) *Message {
	m := &Message{
		Role:            role,
		Content:         content,
		Timestamp:       time.Now().UTC(),
		ToolCalls:       toolCalls,
		ParentMessageID: parentID,
	}
	m.ID = m.ComputeHash()
	return m
}

// Preview returns a truncated version of the content for display
func (m *Message) Preview(maxLen int) string {
	if len(m.Content) <= maxLen {
		return m.Content
	}
	return m.Content[:maxLen-3] + "..."
}
