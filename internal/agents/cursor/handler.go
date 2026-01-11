// Package cursor provides the Cursor IDE agent integration for Tin.
// It implements the HookHandler interface for real-time thread tracking.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dadlerj/tin/internal/agents"
	"github.com/dadlerj/tin/internal/model"
	"github.com/dadlerj/tin/internal/storage"
)

const (
	agentName        = "cursor"
	agentDisplayName = "Cursor"
	stateFileName    = ".tin-cursor-session"
	defaultTimeout   = 30
)

// Handler implements agents.HookHandler for Cursor
type Handler struct {
	config *agents.Config
}

// NewHandler creates a new Cursor handler with the given config.
// If config is nil, default configuration is used.
func NewHandler(config *agents.Config) *Handler {
	if config == nil {
		config = agents.DefaultConfig()
	}
	return &Handler{config: config}
}

// Info returns metadata about the Cursor agent
func (h *Handler) Info() agents.AgentInfo {
	return agents.AgentInfo{
		Name:        agentName,
		DisplayName: agentDisplayName,
		Paradigm:    agents.ParadigmHook,
		Version:     "1.0.0",
	}
}

// Install configures hooks for Cursor
func (h *Handler) Install(projectDir string, global bool) error {
	return InstallHooks(projectDir, global, h.config.HookTimeout)
}

// Uninstall removes hooks for Cursor
func (h *Handler) Uninstall(projectDir string, global bool) error {
	return UninstallHooks(projectDir, global)
}

// IsInstalled checks if hooks are currently installed
func (h *Handler) IsInstalled(projectDir string, global bool) (bool, error) {
	hooksPath := getHooksPath(projectDir, global)
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), "tin hook"), nil
}

// HandleEvent processes a hook event from Cursor
func (h *Handler) HandleEvent(event *agents.HookEvent) (string, error) {
	repo, err := storage.Open(event.Cwd)
	if err != nil {
		// Not a tin repo - return empty string, no error (silent skip)
		return "", nil
	}

	switch event.Type {
	case agents.HookEventUserPrompt:
		return h.handleUserPrompt(repo, event)
	case agents.HookEventAssistantStop:
		return h.handleStop(repo, event)
	case agents.HookEventFileEdit:
		return h.handleFileEdit(repo, event)
	default:
		// For unhandled events, just return without error
		return "", nil
	}
}

func (h *Handler) handleUserPrompt(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	if event.Prompt == "" {
		return "", nil
	}

	// Try to load existing session state
	state, _ := loadSessionState(repo.RootPath, event.SessionID)

	var thread *model.Thread
	var oldThreadID string

	if state != nil {
		// Load existing thread
		var err error
		thread, err = repo.LoadThread(state.ThreadID)
		if err != nil {
			// Thread not found, create new
			thread = nil
		} else {
			oldThreadID = thread.ID
		}
	}

	if thread == nil {
		// Create new thread
		thread = model.NewThread(agentName, event.SessionID, "", "")
		thread.ID = fmt.Sprintf("cursor-%s", event.SessionID[:min(12, len(event.SessionID))])

		// Save session state
		if err := saveSessionState(repo.RootPath, event.SessionID, &SessionState{
			SessionID: event.SessionID,
			ThreadID:  thread.ID,
			StartedAt: time.Now().UTC(),
		}); err != nil {
			return "", err
		}
	}

	// Create human message
	msg := model.NewMessage(model.RoleHuman, event.Prompt, "", nil)
	thread.AddMessage(msg)

	// Handle thread ID change
	if oldThreadID != "" && thread.ID != oldThreadID {
		if err := saveSessionState(repo.RootPath, event.SessionID, &SessionState{
			SessionID: event.SessionID,
			ThreadID:  thread.ID,
			StartedAt: state.StartedAt,
		}); err != nil {
			return thread.ID, err
		}
		repo.DeleteThread(oldThreadID)
		repo.UnstageThread(oldThreadID)
	}

	// Auto-stage if configured
	if h.config.AutoStage == nil || *h.config.AutoStage {
		contentHash := thread.ComputeContentHash()
		if err := repo.StageThread(thread.ID, len(thread.Messages), contentHash); err != nil {
			return thread.ID, err
		}
	}

	return thread.ID, repo.SaveThread(thread)
}

func (h *Handler) handleStop(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	state, err := loadSessionState(repo.RootPath, event.SessionID)
	if err != nil || state == nil {
		return "", nil // No active session
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return state.ThreadID, err
	}

	// Get assistant response from event
	if event.Response == "" && len(event.ToolCalls) == 0 {
		return state.ThreadID, nil // No response to record
	}

	// Get current git hash
	gitHash, _ := repo.GetCurrentGitHash()

	// Create assistant message
	msg := model.NewMessage(model.RoleAssistant, event.Response, "", event.ToolCalls)
	msg.GitHashAfter = gitHash
	thread.AddMessage(msg)

	// Mark as complete on stop
	thread.Complete()

	// Auto-commit git changes if configured
	if h.config.AutoCommitGit == nil || *h.config.AutoCommitGit {
		files, err := repo.GitGetChangedFiles()
		if err == nil && len(files) > 0 {
			if err := repo.GitAdd(files); err == nil {
				hasChanges, err := repo.GitHasStagedChanges()
				if err == nil && hasChanges {
					commitMsg := formatGitCommitMessage(thread)
					repo.GitCommit(commitMsg)
				}
			}
		}
	}

	// Store git hash
	thread.GitCommitHash, _ = repo.GetCurrentGitHash()

	// Auto-stage if configured
	if h.config.AutoStage == nil || *h.config.AutoStage {
		contentHash := thread.ComputeContentHash()
		if err := repo.StageThread(thread.ID, len(thread.Messages), contentHash); err != nil {
			return thread.ID, err
		}
	}

	// Clear session state
	clearSessionState(repo.RootPath, event.SessionID)

	return thread.ID, repo.SaveThread(thread)
}

func (h *Handler) handleFileEdit(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	// File edits are informational - we just note the git hash changed
	state, _ := loadSessionState(repo.RootPath, event.SessionID)
	if state == nil {
		return "", nil
	}

	// The file edit is captured via git hash on the next message
	return state.ThreadID, nil
}

// SessionState tracks the current session for hooks
type SessionState struct {
	SessionID string    `json:"session_id"`
	ThreadID  string    `json:"thread_id"`
	StartedAt time.Time `json:"started_at"`
}

func getSessionStatePath(rootPath, sessionID string) string {
	shortID := sessionID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return filepath.Join(rootPath, ".tin", fmt.Sprintf("%s-%s", stateFileName, shortID))
}

func loadSessionState(rootPath, sessionID string) (*SessionState, error) {
	path := getSessionStatePath(rootPath, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveSessionState(rootPath, sessionID string, state *SessionState) error {
	path := getSessionStatePath(rootPath, sessionID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func clearSessionState(rootPath, sessionID string) {
	path := getSessionStatePath(rootPath, sessionID)
	os.Remove(path)
}

func formatGitCommitMessage(thread *model.Thread) string {
	shortID := thread.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	message := "thread completed"
	if first := thread.FirstHumanMessage(); first != nil {
		message = strings.TrimSpace(first.Content)
	}

	firstLine := message
	restOfMessage := ""
	if idx := strings.Index(message, "\n"); idx != -1 {
		firstLine = strings.TrimSpace(message[:idx])
		restOfMessage = strings.TrimSpace(message[idx+1:])
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[tin %s] %s", shortID, firstLine))

	if restOfMessage != "" {
		builder.WriteString("\n\n")
		builder.WriteString(restOfMessage)
	}

	return builder.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Register this handler with the global registry
func init() {
	agents.Register(NewHandler(nil))
}
