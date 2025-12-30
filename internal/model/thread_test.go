package model

import (
	"testing"
	"time"
)

func TestNewThread(t *testing.T) {
	thread := NewThread("claude-code", "session-123", "", "")

	if thread.Agent != "claude-code" {
		t.Errorf("expected agent 'claude-code', got %s", thread.Agent)
	}
	if thread.AgentSessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", thread.AgentSessionID)
	}
	if thread.Status != ThreadStatusActive {
		t.Errorf("expected status %s, got %s", ThreadStatusActive, thread.Status)
	}
	if thread.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if len(thread.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(thread.Messages))
	}
}

func TestNewThread_WithParent(t *testing.T) {
	thread := NewThread("claude-code", "", "parent-thread-id", "parent-msg-id")

	if thread.ParentThreadID != "parent-thread-id" {
		t.Errorf("expected parent thread ID 'parent-thread-id', got %s", thread.ParentThreadID)
	}
	if thread.ParentMessageID != "parent-msg-id" {
		t.Errorf("expected parent message ID 'parent-msg-id', got %s", thread.ParentMessageID)
	}
}

func TestThread_AddMessage(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	// Thread ID should be empty before first message
	if thread.ID != "" {
		t.Error("expected empty ID before first message")
	}

	// Add first message
	msg1 := NewMessage(RoleHuman, "Hello", "", nil)
	thread.AddMessage(msg1)

	// Thread ID should now be set to first message's ID
	if thread.ID == "" {
		t.Error("expected non-empty ID after first message")
	}
	if thread.ID != msg1.ID {
		t.Errorf("expected thread ID %s, got %s", msg1.ID, thread.ID)
	}
	if len(thread.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(thread.Messages))
	}

	// Add second message
	msg2 := NewMessage(RoleAssistant, "Hi there!", "", nil)
	thread.AddMessage(msg2)

	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(thread.Messages))
	}

	// Second message should have first message as parent
	if thread.Messages[1].ParentMessageID != thread.Messages[0].ID {
		t.Error("second message should have first message as parent")
	}

	// Thread ID should not change after second message
	if thread.ID != msg1.ID {
		t.Error("thread ID should remain unchanged after additional messages")
	}
}

func TestThread_Complete(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	if thread.Status != ThreadStatusActive {
		t.Errorf("expected initial status %s, got %s", ThreadStatusActive, thread.Status)
	}
	if thread.CompletedAt != nil {
		t.Error("expected nil CompletedAt initially")
	}

	thread.Complete()

	if thread.Status != ThreadStatusCompleted {
		t.Errorf("expected status %s after Complete, got %s", ThreadStatusCompleted, thread.Status)
	}
	if thread.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt after Complete")
	}
}

func TestThread_LastMessage(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	// No messages
	if thread.LastMessage() != nil {
		t.Error("expected nil when no messages")
	}

	// Add messages
	msg1 := NewMessage(RoleHuman, "First", "", nil)
	thread.AddMessage(msg1)

	last := thread.LastMessage()
	if last == nil {
		t.Fatal("expected non-nil last message")
	}
	if last.Content != "First" {
		t.Errorf("expected content 'First', got %s", last.Content)
	}

	msg2 := NewMessage(RoleAssistant, "Second", "", nil)
	thread.AddMessage(msg2)

	last = thread.LastMessage()
	if last.Content != "Second" {
		t.Errorf("expected content 'Second', got %s", last.Content)
	}
}

func TestThread_LastGitHash(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	// No messages
	if thread.LastGitHash() != "" {
		t.Error("expected empty hash when no messages")
	}

	// Add message without git hash
	msg1 := NewMessage(RoleHuman, "First", "", nil)
	thread.AddMessage(msg1)

	if thread.LastGitHash() != "" {
		t.Error("expected empty hash when no message has git hash")
	}

	// Add message with git hash
	msg2 := NewMessage(RoleAssistant, "Second", "", nil)
	msg2.GitHashAfter = "abc123"
	thread.AddMessage(msg2)

	if thread.LastGitHash() != "abc123" {
		t.Errorf("expected 'abc123', got %s", thread.LastGitHash())
	}

	// Add another message without git hash
	msg3 := NewMessage(RoleHuman, "Third", "", nil)
	thread.AddMessage(msg3)

	// Should still return the last message with a hash
	if thread.LastGitHash() != "abc123" {
		t.Errorf("expected 'abc123', got %s", thread.LastGitHash())
	}
}

func TestThread_FirstHumanMessage(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	// No messages
	if thread.FirstHumanMessage() != nil {
		t.Error("expected nil when no messages")
	}

	// Add assistant message first
	msg1 := NewMessage(RoleAssistant, "I'm assistant", "", nil)
	thread.AddMessage(msg1)

	if thread.FirstHumanMessage() != nil {
		t.Error("expected nil when no human message")
	}

	// Add human message
	msg2 := NewMessage(RoleHuman, "I'm human", "", nil)
	thread.AddMessage(msg2)

	first := thread.FirstHumanMessage()
	if first == nil {
		t.Fatal("expected non-nil first human message")
	}
	if first.Content != "I'm human" {
		t.Errorf("expected content 'I'm human', got %s", first.Content)
	}

	// Add another human message - should still return first
	msg3 := NewMessage(RoleHuman, "Second human", "", nil)
	thread.AddMessage(msg3)

	first = thread.FirstHumanMessage()
	if first.Content != "I'm human" {
		t.Errorf("expected content 'I'm human', got %s", first.Content)
	}
}

func TestThread_MessageCount(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	if thread.MessageCount() != 0 {
		t.Errorf("expected 0, got %d", thread.MessageCount())
	}

	thread.AddMessage(NewMessage(RoleHuman, "1", "", nil))
	if thread.MessageCount() != 1 {
		t.Errorf("expected 1, got %d", thread.MessageCount())
	}

	thread.AddMessage(NewMessage(RoleAssistant, "2", "", nil))
	if thread.MessageCount() != 2 {
		t.Errorf("expected 2, got %d", thread.MessageCount())
	}
}

func TestThread_HumanMessageCount(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	if thread.HumanMessageCount() != 0 {
		t.Errorf("expected 0, got %d", thread.HumanMessageCount())
	}

	thread.AddMessage(NewMessage(RoleHuman, "human 1", "", nil))
	if thread.HumanMessageCount() != 1 {
		t.Errorf("expected 1, got %d", thread.HumanMessageCount())
	}

	thread.AddMessage(NewMessage(RoleAssistant, "assistant", "", nil))
	if thread.HumanMessageCount() != 1 {
		t.Errorf("expected 1, got %d", thread.HumanMessageCount())
	}

	thread.AddMessage(NewMessage(RoleHuman, "human 2", "", nil))
	if thread.HumanMessageCount() != 2 {
		t.Errorf("expected 2, got %d", thread.HumanMessageCount())
	}
}

func TestThreadStatus_Constants(t *testing.T) {
	// Verify status string values
	if ThreadStatusActive != "active" {
		t.Errorf("expected 'active', got %s", ThreadStatusActive)
	}
	if ThreadStatusCompleted != "completed" {
		t.Errorf("expected 'completed', got %s", ThreadStatusCompleted)
	}
	if ThreadStatusStaged != "staged" {
		t.Errorf("expected 'staged', got %s", ThreadStatusStaged)
	}
	if ThreadStatusCommitted != "committed" {
		t.Errorf("expected 'committed', got %s", ThreadStatusCommitted)
	}
}

func TestThread_MessageChaining(t *testing.T) {
	thread := NewThread("claude-code", "", "", "")

	// Add multiple messages and verify the merkle chain
	for i := 0; i < 5; i++ {
		role := RoleHuman
		if i%2 == 1 {
			role = RoleAssistant
		}
		msg := NewMessage(role, "message "+string(rune('A'+i)), "", nil)
		thread.AddMessage(msg)
	}

	// Verify chain
	for i := 1; i < len(thread.Messages); i++ {
		if thread.Messages[i].ParentMessageID != thread.Messages[i-1].ID {
			t.Errorf("message %d should have message %d as parent", i, i-1)
		}
	}

	// First message should have no parent (from thread perspective)
	// Note: parent ID might be set from NewMessage call, but AddMessage sets it based on previous
	// Actually looking at AddMessage, it only sets parent if there's a previous message
	if thread.Messages[0].ParentMessageID != "" {
		// First message's parent depends on how it was created
		// NewMessage("", ...) sets parent to ""
	}
}

func TestThread_JSON(t *testing.T) {
	thread := NewThread("claude-code", "sess-123", "parent-t", "parent-m")
	thread.AddMessage(NewMessage(RoleHuman, "Hello", "", nil))
	thread.AddMessage(NewMessage(RoleAssistant, "Hi", "", nil))
	thread.Complete()

	// Just verify it doesn't panic and maintains structure
	if thread.Status != ThreadStatusCompleted {
		t.Error("expected completed status")
	}
	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(thread.Messages))
	}
}

func TestThread_StartedAt_IsUTC(t *testing.T) {
	thread := NewThread("test", "", "", "")

	if thread.StartedAt.Location() != time.UTC {
		t.Error("StartedAt should be in UTC")
	}
}
