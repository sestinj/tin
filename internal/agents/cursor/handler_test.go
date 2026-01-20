package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sestinj/tin/internal/agents"
)

func TestHandler_Info(t *testing.T) {
	handler := NewHandler(nil)
	info := handler.Info()

	if info.Name != "cursor" {
		t.Errorf("expected name 'cursor', got %s", info.Name)
	}
	if info.DisplayName != "Cursor" {
		t.Errorf("expected display name 'Cursor', got %s", info.DisplayName)
	}
	if info.Paradigm != agents.ParadigmHook {
		t.Errorf("expected paradigm Hook, got %v", info.Paradigm)
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

func TestSessionState_JSONRoundtrip(t *testing.T) {
	original := SessionState{
		SessionID: "conv-abc123",
		ThreadID:  "thread-xyz789",
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

func TestGetHooksPath_Global(t *testing.T) {
	path := getHooksPath("/project", true)
	if path == "" {
		t.Error("expected non-empty path")
	}
	// Global path should contain home directory, not project
	if filepath.Base(path) != "hooks.json" {
		t.Errorf("expected hooks.json, got %s", filepath.Base(path))
	}
}

func TestGetHooksPath_Project(t *testing.T) {
	path := getHooksPath("/project", false)
	if path == "" {
		t.Error("expected non-empty path")
	}
	expected := "/project/.cursor/hooks.json"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestHandler_IsInstalled_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewHandler(nil)

	installed, err := handler.IsInstalled(tmpDir, false)
	if err != nil {
		t.Fatalf("IsInstalled failed: %v", err)
	}
	if installed {
		t.Error("expected not installed when hooks file doesn't exist")
	}
}

func TestHandler_IsInstalled_NoHooks(t *testing.T) {
	tmpDir := t.TempDir()
	cursorDir := filepath.Join(tmpDir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("failed to create .cursor dir: %v", err)
	}

	// Write hooks file without tin hooks
	hooks := `{"hooks": [{"event": "stop", "command": "other-tool"}]}`
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(hooks), 0644); err != nil {
		t.Fatalf("failed to write hooks: %v", err)
	}

	handler := NewHandler(nil)
	installed, err := handler.IsInstalled(tmpDir, false)
	if err != nil {
		t.Fatalf("IsInstalled failed: %v", err)
	}
	if installed {
		t.Error("expected not installed when hooks file has no tin hooks")
	}
}

func TestHandler_IsInstalled_WithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	cursorDir := filepath.Join(tmpDir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("failed to create .cursor dir: %v", err)
	}

	// Write hooks file with tin hooks
	hooks := `{"hooks": [{"event": "stop", "command": "tin hook cursor-stop"}]}`
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(hooks), 0644); err != nil {
		t.Fatalf("failed to write hooks: %v", err)
	}

	handler := NewHandler(nil)
	installed, err := handler.IsInstalled(tmpDir, false)
	if err != nil {
		t.Fatalf("IsInstalled failed: %v", err)
	}
	if !installed {
		t.Error("expected installed when hooks file has tin hooks")
	}
}

func TestSaveAndLoadSessionState(t *testing.T) {
	tmpDir := t.TempDir()
	tinDir := filepath.Join(tmpDir, ".tin")
	if err := os.MkdirAll(tinDir, 0755); err != nil {
		t.Fatalf("failed to create .tin dir: %v", err)
	}

	sessionID := "test-session-id"
	threadID := "test-thread-id"

	// Save state
	state := &SessionState{
		SessionID: sessionID,
		ThreadID:  threadID,
	}
	if err := saveSessionState(tmpDir, sessionID, state); err != nil {
		t.Fatalf("saveSessionState failed: %v", err)
	}

	// Load state
	loaded, err := loadSessionState(tmpDir, sessionID)
	if err != nil {
		t.Fatalf("loadSessionState failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}

	if loaded.SessionID != sessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", loaded.SessionID, sessionID)
	}
	if loaded.ThreadID != threadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", loaded.ThreadID, threadID)
	}
}

func TestLoadSessionState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tinDir := filepath.Join(tmpDir, ".tin")
	if err := os.MkdirAll(tinDir, 0755); err != nil {
		t.Fatalf("failed to create .tin dir: %v", err)
	}

	// loadSessionState returns error when session file not found
	state, err := loadSessionState(tmpDir, "nonexistent-session")
	if err == nil {
		t.Error("expected error for nonexistent session file")
	}
	if state != nil {
		t.Error("expected nil state for nonexistent session")
	}
}

func TestGetSessionStatePath(t *testing.T) {
	tests := []struct {
		rootPath       string
		conversationID string
		wantContains   string
	}{
		{"/project", "abc123456789xyz", ".tin-cursor-session-abc123456789"},
		{"/project", "short", ".tin-cursor-session-short"},
	}

	for _, tt := range tests {
		got := getSessionStatePath(tt.rootPath, tt.conversationID)
		if got == "" {
			t.Errorf("expected non-empty path for %q, %q", tt.rootPath, tt.conversationID)
		}
	}
}
