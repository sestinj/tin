package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(RoleHuman, "Hello, world!", "", nil)

	if msg.Role != RoleHuman {
		t.Errorf("expected role %s, got %s", RoleHuman, msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %s", msg.Content)
	}
	if msg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestMessage_ComputeHash(t *testing.T) {
	// Create message with known values
	msg := &Message{
		Role:            RoleHuman,
		Content:         "test content",
		Timestamp:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		ParentMessageID: "",
	}

	hash1 := msg.ComputeHash()
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}

	// Same content should produce same hash
	hash2 := msg.ComputeHash()
	if hash1 != hash2 {
		t.Error("identical messages should produce identical hashes")
	}
}

func TestMessage_ComputeHash_DifferentContent(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	msg1 := &Message{
		Role:      RoleHuman,
		Content:   "content A",
		Timestamp: timestamp,
	}

	msg2 := &Message{
		Role:      RoleHuman,
		Content:   "content B",
		Timestamp: timestamp,
	}

	hash1 := msg1.ComputeHash()
	hash2 := msg2.ComputeHash()

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
}

func TestMessage_ComputeHash_DifferentRoles(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	msg1 := &Message{
		Role:      RoleHuman,
		Content:   "same content",
		Timestamp: timestamp,
	}

	msg2 := &Message{
		Role:      RoleAssistant,
		Content:   "same content",
		Timestamp: timestamp,
	}

	hash1 := msg1.ComputeHash()
	hash2 := msg2.ComputeHash()

	if hash1 == hash2 {
		t.Error("different roles should produce different hashes")
	}
}

func TestMessage_ComputeHash_WithToolCalls(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	msg1 := &Message{
		Role:      RoleAssistant,
		Content:   "content",
		Timestamp: timestamp,
	}

	msg2 := &Message{
		Role:      RoleAssistant,
		Content:   "content",
		Timestamp: timestamp,
		ToolCalls: []ToolCall{
			{ID: "1", Name: "read_file", Arguments: json.RawMessage(`{"path": "/test"}`)},
		},
	}

	hash1 := msg1.ComputeHash()
	hash2 := msg2.ComputeHash()

	if hash1 == hash2 {
		t.Error("message with tool calls should have different hash")
	}
}

func TestMessage_ComputeHash_WithParent(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	msg1 := &Message{
		Role:      RoleHuman,
		Content:   "content",
		Timestamp: timestamp,
	}

	msg2 := &Message{
		Role:            RoleHuman,
		Content:         "content",
		Timestamp:       timestamp,
		ParentMessageID: "parent123",
	}

	hash1 := msg1.ComputeHash()
	hash2 := msg2.ComputeHash()

	if hash1 == hash2 {
		t.Error("message with parent should have different hash")
	}
}

func TestMessage_Preview(t *testing.T) {
	tests := []struct {
		content  string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"this is a longer message", 10, "this is..."},
		{"", 10, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		msg := &Message{Content: tt.content}
		result := msg.Preview(tt.maxLen)
		if result != tt.expected {
			t.Errorf("Preview(%q, %d) = %q, want %q", tt.content, tt.maxLen, result, tt.expected)
		}
	}
}

func TestToolCall_JSON(t *testing.T) {
	tc := ToolCall{
		ID:        "call_123",
		Name:      "bash",
		Arguments: json.RawMessage(`{"command": "ls -la"}`),
		Result:    "file1.txt\nfile2.txt",
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var tc2 ToolCall
	if err := json.Unmarshal(data, &tc2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if tc2.ID != tc.ID {
		t.Errorf("ID mismatch: got %s, want %s", tc2.ID, tc.ID)
	}
	if tc2.Name != tc.Name {
		t.Errorf("Name mismatch: got %s, want %s", tc2.Name, tc.Name)
	}
	if tc2.Result != tc.Result {
		t.Errorf("Result mismatch: got %s, want %s", tc2.Result, tc.Result)
	}
}
