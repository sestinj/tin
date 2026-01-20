// Package agents provides interfaces and common types for integrating
// various AI coding assistants with Tin's thread-based version control.
//
// There are three integration paradigms supported:
//
// 1. HookHandler - For agents with real-time hook support (Claude Code, Cursor)
//    These agents call external commands at lifecycle events.
//
// 2. NotifyHandler - For agents with notification-only support (Codex CLI)
//    These agents send limited event notifications that trigger syncs.
//
// 3. PullAdapter - For agents requiring pull-based sync (AMP, Copilot CLI)
//    These agents don't support hooks; threads are fetched on demand.
package agents

import (
	"encoding/json"
	"time"

	"github.com/sestinj/tin/internal/model"
)

// AgentInfo contains metadata about an agent integration
type AgentInfo struct {
	Name        string   // e.g., "claude-code", "cursor", "codex", "amp"
	DisplayName string   // e.g., "Claude Code", "Cursor", "Codex CLI", "AMP"
	Paradigm    Paradigm // hook, notify, or pull
	Version     string   // Version of the integration
}

// Paradigm represents the integration pattern used by an agent
type Paradigm string

const (
	ParadigmHook   Paradigm = "hook"   // Real-time hooks (push model)
	ParadigmNotify Paradigm = "notify" // Notification + sync (hybrid model)
	ParadigmPull   Paradigm = "pull"   // Pull/import (batch model)
)

// HookEvent represents a normalized event from hook-based agents
type HookEvent struct {
	Type        HookEventType   // Event type
	SessionID   string          // Agent's session identifier
	Cwd         string          // Working directory
	Timestamp   time.Time       // When the event occurred
	Prompt      string          // User prompt (for message events)
	Response    string          // Assistant response (for response events)
	ToolCalls   []model.ToolCall // Tool calls in response
	Transcript  string          // Path to transcript file
	RawPayload  json.RawMessage // Agent-specific raw data
}

// HookEventType enumerates the standard hook events
type HookEventType string

const (
	HookEventSessionStart  HookEventType = "session_start"
	HookEventUserPrompt    HookEventType = "user_prompt"
	HookEventAssistantStop HookEventType = "assistant_stop"
	HookEventSessionEnd    HookEventType = "session_end"
	HookEventFileEdit      HookEventType = "file_edit"     // Cursor-specific
	HookEventToolUse       HookEventType = "tool_use"      // Pre/post tool use
)

// NotifyEvent represents a notification from notification-based agents
type NotifyEvent struct {
	Type        NotifyEventType // Event type
	SessionID   string          // Agent's session/thread identifier
	Cwd         string          // Working directory
	Timestamp   time.Time       // When the event occurred
	Message     string          // Last assistant message (if available)
	RawPayload  json.RawMessage // Agent-specific raw data
}

// NotifyEventType enumerates notification event types
type NotifyEventType string

const (
	NotifyEventTurnComplete NotifyEventType = "turn_complete" // Agent finished a turn
)

// PullOptions configures how threads are pulled from an agent
type PullOptions struct {
	Count      int    // Number of threads to pull (0 = all available)
	ThreadID   string // Specific thread ID to pull
	Since      *time.Time // Only pull threads updated since this time
	IncludeGit bool   // Record current git hash on pulled threads
}

// HookHandler defines the interface for agents with full hook support.
// These agents call Tin at lifecycle events, enabling real-time tracking.
//
// Examples: Claude Code, Cursor
type HookHandler interface {
	// Info returns metadata about this agent
	Info() AgentInfo

	// Install configures hooks for this agent.
	// If global is true, installs to user-level config (~/.agent/);
	// otherwise installs to project-level (.agent/).
	Install(projectDir string, global bool) error

	// Uninstall removes hooks for this agent.
	Uninstall(projectDir string, global bool) error

	// IsInstalled checks if hooks are currently installed.
	IsInstalled(projectDir string, global bool) (bool, error)

	// HandleEvent processes a hook event and updates the thread.
	// Returns the thread ID that was created/updated, or error.
	HandleEvent(event *HookEvent) (string, error)
}

// NotifyHandler defines the interface for agents with notification support.
// These agents send limited events that trigger Tin to sync thread data.
//
// Examples: Codex CLI
type NotifyHandler interface {
	// Info returns metadata about this agent
	Info() AgentInfo

	// Setup configures notification handling for this agent.
	// This may involve outputting config instructions for the user.
	Setup(projectDir string) error

	// HandleNotification processes a notification event.
	// This typically triggers a sync of the affected thread.
	HandleNotification(event *NotifyEvent) error

	// SyncThread fetches and updates a specific thread by session ID.
	SyncThread(sessionID string, cwd string) (*model.Thread, error)
}

// PullAdapter defines the interface for agents requiring pull-based sync.
// These agents don't support hooks; threads are fetched on demand.
//
// Examples: AMP, GitHub Copilot CLI
type PullAdapter interface {
	// Info returns metadata about this agent
	Info() AgentInfo

	// List returns available thread IDs from the agent.
	List(limit int) ([]string, error)

	// Pull fetches a specific thread by ID.
	Pull(threadID string, opts PullOptions) (*model.Thread, error)

	// PullRecent fetches the N most recent threads.
	PullRecent(count int, opts PullOptions) ([]*model.Thread, error)
}

// Config holds agent-specific configuration
type Config struct {
	// HookTimeout is the timeout for hook execution in seconds.
	// Default is 30 seconds if not specified.
	HookTimeout int `json:"hook_timeout,omitempty"`

	// AutoStage automatically stages threads after updates.
	// Default is true.
	AutoStage *bool `json:"auto_stage,omitempty"`

	// AutoCommitGit automatically commits git changes on session end.
	// Default is true for hook-based agents.
	AutoCommitGit *bool `json:"auto_commit_git,omitempty"`

	// PollInterval is the interval for daemon polling in seconds.
	// Only applicable to pull-based agents with daemon mode.
	PollInterval int `json:"poll_interval,omitempty"`
}

// DefaultConfig returns the default agent configuration
func DefaultConfig() *Config {
	autoStage := true
	autoCommit := true
	return &Config{
		HookTimeout:   30,
		AutoStage:     &autoStage,
		AutoCommitGit: &autoCommit,
		PollInterval:  60,
	}
}
