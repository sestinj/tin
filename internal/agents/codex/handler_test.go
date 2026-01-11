package codex

import (
	"encoding/json"
	"testing"

	"github.com/dadlerj/tin/internal/agents"
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
