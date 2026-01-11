package amp

import (
	"testing"

	"github.com/dadlerj/tin/internal/agents"
	"github.com/dadlerj/tin/internal/model"
)

func TestAdapter_Info(t *testing.T) {
	adapter := NewAdapter()
	info := adapter.Info()

	if info.Name != "amp" {
		t.Errorf("expected name 'amp', got %s", info.Name)
	}
	if info.DisplayName != "AMP (Sourcegraph)" {
		t.Errorf("expected display name 'AMP (Sourcegraph)', got %s", info.DisplayName)
	}
	if info.Paradigm != agents.ParadigmPull {
		t.Errorf("expected paradigm Pull, got %v", info.Paradigm)
	}
}

func TestParseMarkdown_BasicConversation(t *testing.T) {
	markdown := `---
created: 2024-01-15T10:30:00Z
---

## User

Hello, can you help me?

## Assistant

Of course! How can I assist you today?

## User

What is 2 + 2?

## Assistant

2 + 2 equals 4.
`

	thread, err := parseMarkdown(markdown, "T-test-123", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if thread.Agent != "amp" {
		t.Errorf("expected agent 'amp', got %s", thread.Agent)
	}

	if thread.AgentSessionID != "T-test-123" {
		t.Errorf("expected AgentSessionID 'T-test-123', got %s", thread.AgentSessionID)
	}

	if len(thread.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(thread.Messages))
	}

	// Verify message roles
	expectedRoles := []model.Role{model.RoleHuman, model.RoleAssistant, model.RoleHuman, model.RoleAssistant}
	for i, msg := range thread.Messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("message %d: expected role %s, got %s", i, expectedRoles[i], msg.Role)
		}
	}

	// Verify first user message content
	if thread.Messages[0].Content != "Hello, can you help me?" {
		t.Errorf("unexpected first message content: %s", thread.Messages[0].Content)
	}
}

func TestParseMarkdown_WithToolUse(t *testing.T) {
	markdown := `---
created: 2024-01-15T10:30:00Z
---

## User

Read the file test.go

## Assistant

I'll read that file for you.

**Tool Use:** ` + "`Read`" + `

` + "```json" + `
{"file_path": "/path/to/test.go"}
` + "```" + `

**Tool Result:**

` + "```" + `
package main

func main() {}
` + "```" + `

Here's the content of the file.
`

	thread, err := parseMarkdown(markdown, "T-tool-test", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(thread.Messages))
	}

	// Second message should have tool calls
	assistantMsg := thread.Messages[1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}

	if assistantMsg.ToolCalls[0].Name != "Read" {
		t.Errorf("expected tool name 'Read', got %s", assistantMsg.ToolCalls[0].Name)
	}

	// Verify tool result is preserved (this was previously discarded)
	if assistantMsg.ToolCalls[0].Result == "" {
		t.Error("expected tool result to be preserved, got empty string")
	}
}

func TestParseMarkdown_MultipleToolCalls(t *testing.T) {
	markdown := `---
created: 2024-01-15T10:30:00Z
---

## User

Check both files

## Assistant

I'll check both files.

**Tool Use:** ` + "`Read`" + `

` + "```json" + `
{"file_path": "file1.go"}
` + "```" + `

**Tool Result:**

` + "```" + `
content of file1
` + "```" + `

**Tool Use:** ` + "`Read`" + `

` + "```json" + `
{"file_path": "file2.go"}
` + "```" + `

**Tool Result:**

` + "```" + `
content of file2
` + "```" + `

Both files have been read.
`

	thread, err := parseMarkdown(markdown, "T-multi-tool", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(thread.Messages))
	}

	assistantMsg := thread.Messages[1]
	if len(assistantMsg.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls, got %d", len(assistantMsg.ToolCalls))
	}

	// Verify both tool results are captured
	for i, tc := range assistantMsg.ToolCalls {
		if tc.Result == "" {
			t.Errorf("tool call %d: expected result to be captured", i)
		}
	}
}

func TestParseMarkdown_NoFrontmatter(t *testing.T) {
	markdown := `## User

Simple message

## Assistant

Simple response
`

	thread, err := parseMarkdown(markdown, "T-no-front", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(thread.Messages))
	}

	// Without frontmatter, AgentSessionID is not set (only set in frontmatter parsing)
	// Thread ID should fall back to first message ID
	if thread.ID == "" {
		t.Error("expected non-empty thread ID")
	}
}

func TestParseMarkdown_EmptyContent(t *testing.T) {
	markdown := `---
created: 2024-01-15T10:30:00Z
---
`

	thread, err := parseMarkdown(markdown, "T-empty", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if len(thread.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(thread.Messages))
	}

	// Thread ID should fall back to ampThreadID when no messages
	if thread.ID != "T-empty" {
		t.Errorf("expected thread ID 'T-empty', got %s", thread.ID)
	}
}

func TestParseMarkdown_InvalidToolJSON(t *testing.T) {
	markdown := `## User

Test

## Assistant

Testing

**Tool Use:** ` + "`Bash`" + `

` + "```json" + `
{invalid json here}
` + "```" + `
`

	thread, err := parseMarkdown(markdown, "T-invalid-json", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	assistantMsg := thread.Messages[1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call even with invalid JSON, got %d", len(assistantMsg.ToolCalls))
	}

	// Should have error indicator in arguments
	if string(assistantMsg.ToolCalls[0].Arguments) == "{}" {
		t.Error("expected error indicator in arguments for invalid JSON")
	}
}

func TestParseMarkdown_ThreadStatus(t *testing.T) {
	markdown := `## User

Test message

## Assistant

Response
`

	thread, err := parseMarkdown(markdown, "T-status", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if thread.Status != model.ThreadStatusCompleted {
		t.Errorf("expected status Completed, got %s", thread.Status)
	}

	if thread.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestParseMarkdown_TimestampParsing(t *testing.T) {
	markdown := `---
created: 2024-06-15T14:30:00Z
---

## User

Test
`

	thread, err := parseMarkdown(markdown, "T-time", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if thread.StartedAt.IsZero() {
		t.Error("expected StartedAt to be parsed from frontmatter")
	}

	if thread.StartedAt.Year() != 2024 || thread.StartedAt.Month() != 6 {
		t.Errorf("unexpected StartedAt: %v", thread.StartedAt)
	}
}

func TestParseMarkdown_RobustRoleDetection(t *testing.T) {
	// Test with extra whitespace and variations
	markdown := `---
created: 2024-01-15T10:30:00Z
---

  ## User

Message with whitespace

## Assistant

Response
`

	thread, err := parseMarkdown(markdown, "T-whitespace", false)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if len(thread.Messages) != 2 {
		t.Errorf("expected 2 messages with whitespace handling, got %d", len(thread.Messages))
	}
}
