package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ThreadStatus represents the current state of a thread
type ThreadStatus string

const (
	ThreadStatusActive    ThreadStatus = "active"
	ThreadStatusCompleted ThreadStatus = "completed"
	ThreadStatusStaged    ThreadStatus = "staged"
	ThreadStatusCommitted ThreadStatus = "committed"
)

// Thread represents a conversation session with an AI agent
type Thread struct {
	ID                    string       `json:"id"`
	ParentThreadID        string       `json:"parent_thread_id,omitempty"`
	ParentMessageID       string       `json:"parent_message_id,omitempty"` // fork point if branched mid-thread
	Agent                 string       `json:"agent"`                       // "claude-code", "cursor", "amp", etc.
	AgentSessionID        string       `json:"agent_session_id,omitempty"`  // for resume capability
	StartedAt             time.Time    `json:"started_at"`
	CompletedAt           *time.Time   `json:"completed_at,omitempty"`
	Status                ThreadStatus `json:"status"`
	Messages              []Message    `json:"messages"`
	GitCommitHash        string `json:"git_commit_hash,omitempty"`        // git commit created when thread ended
	CommittedContentHash string `json:"committed_content_hash,omitempty"` // hash of thread content at last commit
}

// NewThread creates a new thread
func NewThread(agent string, agentSessionID string, parentThreadID string, parentMessageID string) *Thread {
	return &Thread{
		Agent:           agent,
		AgentSessionID:  agentSessionID,
		ParentThreadID:  parentThreadID,
		ParentMessageID: parentMessageID,
		StartedAt:       time.Now().UTC(),
		Status:          ThreadStatusActive,
		Messages:        []Message{},
	}
}

// AddMessage appends a message to the thread and updates the thread ID if this is the first message
func (t *Thread) AddMessage(msg *Message) {
	// Set parent message ID based on previous message in thread
	if len(t.Messages) > 0 {
		msg.ParentMessageID = t.Messages[len(t.Messages)-1].ID
		msg.ID = msg.ComputeHash()
	}

	t.Messages = append(t.Messages, *msg)

	// Thread ID is the hash of the first message
	if len(t.Messages) == 1 {
		t.ID = msg.ID
	}
}

// Complete marks the thread as completed
func (t *Thread) Complete() {
	now := time.Now().UTC()
	t.CompletedAt = &now
	t.Status = ThreadStatusCompleted
}

// LastMessage returns the most recent message in the thread
func (t *Thread) LastMessage() *Message {
	if len(t.Messages) == 0 {
		return nil
	}
	return &t.Messages[len(t.Messages)-1]
}

// LastGitHash returns the git hash after the most recent message that has one
func (t *Thread) LastGitHash() string {
	for i := len(t.Messages) - 1; i >= 0; i-- {
		if t.Messages[i].GitHashAfter != "" {
			return t.Messages[i].GitHashAfter
		}
	}
	return ""
}

// FirstHumanMessage returns the first human message for preview purposes
func (t *Thread) FirstHumanMessage() *Message {
	for i := range t.Messages {
		if t.Messages[i].Role == RoleHuman {
			return &t.Messages[i]
		}
	}
	return nil
}

// MessageCount returns the number of messages in the thread
func (t *Thread) MessageCount() int {
	return len(t.Messages)
}

// HumanMessageCount returns the number of human messages
func (t *Thread) HumanMessageCount() int {
	count := 0
	for _, m := range t.Messages {
		if m.Role == RoleHuman {
			count++
		}
	}
	return count
}

// ComputeContentHash generates a SHA256 hash of all message content.
// This is used to detect if thread content has changed since the last commit.
func (t *Thread) ComputeContentHash() string {
	h := sha256.New()
	for _, msg := range t.Messages {
		h.Write([]byte(string(msg.Role)))
		h.Write([]byte(msg.Content))
		if len(msg.ToolCalls) > 0 {
			toolCallsJSON, _ := json.Marshal(msg.ToolCalls)
			h.Write(toolCallsJSON)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
