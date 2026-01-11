package remote

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/dadlerj/tin/internal/model"
)

// Protocol version
const ProtocolVersion = 1

// MessageType identifies the type of protocol message
type MessageType string

const (
	MsgHello      MessageType = "hello"
	MsgRefs       MessageType = "refs"
	MsgWant       MessageType = "want"
	MsgPack       MessageType = "pack"
	MsgUpdateRefs MessageType = "update-refs"
	MsgGetConfig  MessageType = "get-config"
	MsgConfig     MessageType = "config"
	MsgSetConfig  MessageType = "set-config"
	MsgOK         MessageType = "ok"
	MsgError      MessageType = "error"
)

// Message is the envelope for all protocol messages
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// AuthInfo contains authentication credentials
type AuthInfo struct {
	Type  string `json:"type"`  // "token"
	Token string `json:"token"` // the auth token (e.g., "th_xxx")
}

// HelloMessage initiates the connection
type HelloMessage struct {
	Version   int       `json:"version"`
	Operation string    `json:"operation"` // "push" or "pull"
	RepoPath  string    `json:"repo_path"` // path to repository on server
	Auth      *AuthInfo `json:"auth,omitempty"`
}

// RefsMessage advertises refs and object IDs
type RefsMessage struct {
	HEAD           string              `json:"head"`                      // current HEAD branch
	Branches       map[string]string   `json:"branches"`                  // branch name -> commit ID
	CommitIDs      []string            `json:"commit_ids,omitempty"`      // all known commit IDs
	ThreadIDs      []string            `json:"thread_ids,omitempty"`      // all known thread IDs
	ThreadVersions map[string][]string `json:"thread_versions,omitempty"` // threadID -> [contentHashes]
}

// ThreadVersionRef identifies a specific version of a thread
type ThreadVersionRef struct {
	ThreadID    string `json:"thread_id"`
	ContentHash string `json:"content_hash"`
}

// WantMessage requests specific objects
type WantMessage struct {
	CommitIDs      []string           `json:"commit_ids,omitempty"`
	ThreadIDs      []string           `json:"thread_ids,omitempty"`
	ThreadVersions []ThreadVersionRef `json:"thread_versions,omitempty"` // request specific versions
}

// PackMessage contains objects to transfer
type PackMessage struct {
	Commits []model.TinCommit `json:"commits,omitempty"`
	Threads []model.Thread    `json:"threads,omitempty"`
}

// UpdateRefsMessage requests ref updates (for push)
type UpdateRefsMessage struct {
	Updates map[string]string `json:"updates"` // branch name -> commit ID
	Force   bool              `json:"force"`
}

// GetConfigMessage requests config from remote
type GetConfigMessage struct {
	Keys []string `json:"keys,omitempty"` // specific keys to get, empty means all
}

// ConfigMessage contains config data
type ConfigMessage struct {
	CodeHostURL string `json:"code_host_url,omitempty"`
}

// SetConfigMessage requests config update on remote
type SetConfigMessage struct {
	CodeHostURL string `json:"code_host_url,omitempty"`
}

// OKMessage indicates success
type OKMessage struct {
	Message string `json:"message,omitempty"`
}

// ErrorMessage indicates an error
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes
const (
	ErrCodeNotFound        = "not_found"
	ErrCodeInvalidRequest  = "invalid_request"
	ErrCodeNotFastForward  = "not_fast_forward"
	ErrCodeInternal        = "internal"
	ErrCodeProtocolVersion = "protocol_version"
)

// ProtocolConn wraps a connection for protocol message exchange
type ProtocolConn struct {
	reader *bufio.Reader
	writer io.Writer
}

// NewProtocolConn creates a new protocol connection wrapper
func NewProtocolConn(rw io.ReadWriter) *ProtocolConn {
	return &ProtocolConn{
		reader: bufio.NewReader(rw),
		writer: rw,
	}
}

// Send sends a message with the given type and payload
func (c *ProtocolConn) Send(msgType MessageType, payload interface{}) error {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	msg := Message{
		Type:    msgType,
		Payload: payloadBytes,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message followed by newline
	if _, err := c.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Receive reads and returns the next message
func (c *ProtocolConn) Receive() (*Message, error) {
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// SendError sends an error message
func (c *ProtocolConn) SendError(code, message string) error {
	return c.Send(MsgError, ErrorMessage{Code: code, Message: message})
}

// SendOK sends an OK message
func (c *ProtocolConn) SendOK(message string) error {
	return c.Send(MsgOK, OKMessage{Message: message})
}

// DecodePayload decodes the message payload into the given value
func (m *Message) DecodePayload(v interface{}) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}
