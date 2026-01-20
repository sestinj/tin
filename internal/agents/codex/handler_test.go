package codex

import (
	"encoding/json"
	"testing"

	"github.com/sestinj/tin/internal/agents"
	"github.com/sestinj/tin/internal/model"
	"github.com/sestinj/tin/internal/storage"
)

func TestHandler_Info(t *testing.T) {
	handler := NewHandler(nil)
	info := handler.Info()

	if info.Name != "codex" {
		t.Errorf("expected name 'codex', got %s", info.Name)
	}
	if info.DisplayName != "Codex CLI" {
		t.Errorf("expected display name 'Codex CLI', got %s", info.DisplayName)
	}
	if info.Paradigm != agents.ParadigmNotify {
		t.Errorf("expected paradigm Notify, got %v", info.Paradigm)
	}
}

func TestHandler_NewWithNilConfig(t *testing.T) {
	handler := NewHandler(nil)
	if handler.config == nil {
		t.Error("expected default config when nil is passed")
	}
}

func TestHandler_NewWithConfig(t *testing.T) {
	customTimeout := 60
	config := &agents.Config{HookTimeout: customTimeout}
	handler := NewHandler(config)

	if handler.config.HookTimeout != customTimeout {
		t.Errorf("expected custom timeout %d, got %d", customTimeout, handler.config.HookTimeout)
	}
}

func TestParseNotifyArgs_Valid(t *testing.T) {
	payload := CodexNotifyPayload{
		Type:                 "agent-turn-complete",
		ThreadID:             "thread-123",
		TurnID:               "turn-456",
		Cwd:                  "/project/path",
		InputMessages:        []string{"Hello", "How are you?"},
		LastAssistantMessage: "I'm doing well, thanks!",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	event, err := ParseNotifyArgs([]string{string(data)})
	if err != nil {
		t.Fatalf("ParseNotifyArgs failed: %v", err)
	}

	if event.Type != agents.NotifyEventTurnComplete {
		t.Errorf("expected type TurnComplete, got %v", event.Type)
	}
	if event.SessionID != "thread-123" {
		t.Errorf("expected session ID 'thread-123', got %s", event.SessionID)
	}
	if event.Cwd != "/project/path" {
		t.Errorf("expected cwd '/project/path', got %s", event.Cwd)
	}
	if event.Message != "I'm doing well, thanks!" {
		t.Errorf("unexpected message: %s", event.Message)
	}
}

func TestParseNotifyArgs_NoArgs(t *testing.T) {
	_, err := ParseNotifyArgs([]string{})
	if err == nil {
		t.Error("expected error with no arguments")
	}
}

func TestParseNotifyArgs_InvalidJSON(t *testing.T) {
	_, err := ParseNotifyArgs([]string{"not valid json"})
	if err == nil {
		t.Error("expected error with invalid JSON")
	}
}

func TestCodexNotifyPayload_JSONRoundtrip(t *testing.T) {
	original := CodexNotifyPayload{
		Type:                 "agent-turn-complete",
		ThreadID:             "T-abc123",
		TurnID:               "turn-xyz",
		Cwd:                  "/home/user/project",
		InputMessages:        []string{"First message", "Second message"},
		LastAssistantMessage: "Response content",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CodexNotifyPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %s, want %s", decoded.Type, original.Type)
	}
	if decoded.ThreadID != original.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", decoded.ThreadID, original.ThreadID)
	}
	if decoded.TurnID != original.TurnID {
		t.Errorf("TurnID mismatch: got %s, want %s", decoded.TurnID, original.TurnID)
	}
	if decoded.Cwd != original.Cwd {
		t.Errorf("Cwd mismatch: got %s, want %s", decoded.Cwd, original.Cwd)
	}
	if len(decoded.InputMessages) != len(original.InputMessages) {
		t.Errorf("InputMessages length mismatch: got %d, want %d", len(decoded.InputMessages), len(original.InputMessages))
	}
	if decoded.LastAssistantMessage != original.LastAssistantMessage {
		t.Errorf("LastAssistantMessage mismatch: got %s, want %s", decoded.LastAssistantMessage, original.LastAssistantMessage)
	}
}

func TestSessionState_JSONRoundtrip(t *testing.T) {
	original := SessionState{
		SessionID: "sess-123",
		ThreadID:  "thread-456",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded SessionState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", decoded.SessionID, original.SessionID)
	}
	if decoded.ThreadID != original.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", decoded.ThreadID, original.ThreadID)
	}
}

func TestGetSessionStatePath(t *testing.T) {
	tests := []struct {
		rootPath  string
		sessionID string
		wantSuffix string
	}{
		{"/project", "abc123456789xyz", ".tin-codex-session-abc123456789"},
		{"/project", "short", ".tin-codex-session-short"},
		{"/project", "exactly12chr", ".tin-codex-session-exactly12chr"},
	}

	for _, tt := range tests {
		got := getSessionStatePath(tt.rootPath, tt.sessionID)
		if got != tt.rootPath+"/.tin/"+tt.wantSuffix {
			t.Errorf("getSessionStatePath(%q, %q) = %q, want suffix %q", tt.rootPath, tt.sessionID, got, tt.wantSuffix)
		}
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{0, 10, 0},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestHandler_HandleNotification_NoDuplicateMessages(t *testing.T) {
	// Setup: create a temp tin repo
	tmpDir := t.TempDir()
	_, err := storage.Init(tmpDir)
	if err != nil {
		t.Fatalf("failed to init tin repo: %v", err)
	}

	handler := NewHandler(nil)
	threadID := "test-thread-123"

	// First notification: user sends "testing"
	payload1 := CodexNotifyPayload{
		Type:                 "agent-turn-complete",
		ThreadID:             threadID,
		TurnID:               "turn-1",
		Cwd:                  tmpDir,
		InputMessages:        []string{"testing"},
		LastAssistantMessage: "Got your test!",
	}
	data1, _ := json.Marshal(payload1)
	event1 := &agents.NotifyEvent{
		Type:       agents.NotifyEventTurnComplete,
		SessionID:  threadID,
		Cwd:        tmpDir,
		RawPayload: data1,
	}

	if err := handler.HandleNotification(event1); err != nil {
		t.Fatalf("first notification failed: %v", err)
	}

	// Second notification: user sends "wow" (Codex includes ALL previous messages)
	payload2 := CodexNotifyPayload{
		Type:                 "agent-turn-complete",
		ThreadID:             threadID,
		TurnID:               "turn-2",
		Cwd:                  tmpDir,
		InputMessages:        []string{"testing", "wow"}, // Codex sends all messages
		LastAssistantMessage: "Nice!",
	}
	data2, _ := json.Marshal(payload2)
	event2 := &agents.NotifyEvent{
		Type:       agents.NotifyEventTurnComplete,
		SessionID:  threadID,
		Cwd:        tmpDir,
		RawPayload: data2,
	}

	if err := handler.HandleNotification(event2); err != nil {
		t.Fatalf("second notification failed: %v", err)
	}

	// Third notification: user sends "tell a story"
	payload3 := CodexNotifyPayload{
		Type:                 "agent-turn-complete",
		ThreadID:             threadID,
		TurnID:               "turn-3",
		Cwd:                  tmpDir,
		InputMessages:        []string{"testing", "wow", "tell a story"}, // All messages again
		LastAssistantMessage: "Once upon a time...",
	}
	data3, _ := json.Marshal(payload3)
	event3 := &agents.NotifyEvent{
		Type:       agents.NotifyEventTurnComplete,
		SessionID:  threadID,
		Cwd:        tmpDir,
		RawPayload: data3,
	}

	if err := handler.HandleNotification(event3); err != nil {
		t.Fatalf("third notification failed: %v", err)
	}

	// Verify: load the thread and check message count
	repo, err := storage.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	threads, err := repo.FindThreadsBySessionID(threadID)
	if err != nil {
		t.Fatalf("failed to find thread: %v", err)
	}
	if len(threads) == 0 {
		t.Fatal("thread not found")
	}

	thread := threads[0]

	// Count messages by role
	humanCount := 0
	assistantCount := 0
	for _, msg := range thread.Messages {
		if msg.Role == model.RoleHuman {
			humanCount++
		} else if msg.Role == model.RoleAssistant {
			assistantCount++
		}
	}

	// Should have exactly 3 human messages (not 6 from duplicates)
	if humanCount != 3 {
		t.Errorf("expected 3 human messages, got %d", humanCount)
	}

	// Should have exactly 3 assistant messages
	if assistantCount != 3 {
		t.Errorf("expected 3 assistant messages, got %d", assistantCount)
	}

	// Verify message order and content
	expectedMessages := []struct {
		role    model.Role
		content string
	}{
		{model.RoleHuman, "testing"},
		{model.RoleAssistant, "Got your test!"},
		{model.RoleHuman, "wow"},
		{model.RoleAssistant, "Nice!"},
		{model.RoleHuman, "tell a story"},
		{model.RoleAssistant, "Once upon a time..."},
	}

	if len(thread.Messages) != len(expectedMessages) {
		t.Fatalf("expected %d messages, got %d", len(expectedMessages), len(thread.Messages))
	}

	for i, expected := range expectedMessages {
		if thread.Messages[i].Role != expected.role {
			t.Errorf("message %d: expected role %s, got %s", i, expected.role, thread.Messages[i].Role)
		}
		if thread.Messages[i].Content != expected.content {
			t.Errorf("message %d: expected content %q, got %q", i, expected.content, thread.Messages[i].Content)
		}
	}
}
