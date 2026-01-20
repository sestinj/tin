// Package claudecode provides the Claude Code agent integration for Tin.
// It implements the HookHandler interface for real-time thread tracking.
package claudecode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sestinj/tin/internal/agents"
	"github.com/sestinj/tin/internal/model"
	"github.com/sestinj/tin/internal/storage"
)

const (
	agentName        = "claude-code"
	agentDisplayName = "Claude Code"
	stateFileName    = ".tin-session"
	defaultTimeout   = 30
	maxBufferSize    = 1024 * 1024 // 1MB for transcript parsing
)

// Handler implements agents.HookHandler for Claude Code
type Handler struct {
	config *agents.Config
}

// NewHandler creates a new Claude Code handler with the given config.
// If config is nil, default configuration is used.
func NewHandler(config *agents.Config) *Handler {
	if config == nil {
		config = agents.DefaultConfig()
	}
	return &Handler{config: config}
}

// Info returns metadata about the Claude Code agent
func (h *Handler) Info() agents.AgentInfo {
	return agents.AgentInfo{
		Name:        agentName,
		DisplayName: agentDisplayName,
		Paradigm:    agents.ParadigmHook,
		Version:     "1.0.0",
	}
}

// Install configures hooks for Claude Code
func (h *Handler) Install(projectDir string, global bool) error {
	return InstallHooks(projectDir, global, h.config.HookTimeout)
}

// Uninstall removes hooks for Claude Code
func (h *Handler) Uninstall(projectDir string, global bool) error {
	return UninstallHooks(projectDir, global)
}

// IsInstalled checks if hooks are currently installed
func (h *Handler) IsInstalled(projectDir string, global bool) (bool, error) {
	settingsPath := getSettingsPath(projectDir, global)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), "tin hook"), nil
}

// HandleEvent processes a hook event from Claude Code
func (h *Handler) HandleEvent(event *agents.HookEvent) (string, error) {
	repo, err := storage.Open(event.Cwd)
	if err != nil {
		// Not a tin repo - return empty string, no error (silent skip)
		return "", nil
	}

	switch event.Type {
	case agents.HookEventSessionStart:
		return h.handleSessionStart(repo, event)
	case agents.HookEventUserPrompt:
		return h.handleUserPrompt(repo, event)
	case agents.HookEventAssistantStop:
		return h.handleStop(repo, event)
	case agents.HookEventSessionEnd:
		return h.handleSessionEnd(repo, event)
	default:
		return "", fmt.Errorf("unknown event type: %s", event.Type)
	}
}

func (h *Handler) handleSessionStart(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	// Check if we already have a thread for this session
	state, _ := loadSessionState(repo.RootPath, event.SessionID)
	if state != nil && state.SessionID == event.SessionID {
		// Already tracking this session
		return state.ThreadID, nil
	}

	// Prune any empty threads before creating a new one
	repo.PruneEmptyThreads()

	// Check for existing threads with this session ID (for resumed sessions)
	var parentThreadID, parentMessageID string
	existingThreads, _ := repo.FindThreadsBySessionID(event.SessionID)
	if len(existingThreads) > 0 {
		// Link to the most recent thread with this session ID
		parent := existingThreads[0]
		parentThreadID = parent.ID
		if len(parent.Messages) > 0 {
			parentMessageID = parent.Messages[len(parent.Messages)-1].ID
		}
	}

	// Create a new thread with parent reference if resuming
	thread := model.NewThread(agentName, event.SessionID, parentThreadID, parentMessageID)

	// Generate a temporary ID until first message
	thread.ID = fmt.Sprintf("cc-%s", event.SessionID[:min(12, len(event.SessionID))])

	if err := repo.SaveThread(thread); err != nil {
		return "", err
	}

	// Save session state
	if err := saveSessionState(repo.RootPath, event.SessionID, &SessionState{
		SessionID: event.SessionID,
		ThreadID:  thread.ID,
		StartedAt: time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return thread.ID, nil
}

func (h *Handler) handleUserPrompt(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	if event.Prompt == "" {
		return "", nil
	}

	state, err := loadSessionState(repo.RootPath, event.SessionID)
	if err != nil || state == nil {
		// No active session, create one
		threadID, err := h.handleSessionStart(repo, event)
		if err != nil {
			return "", err
		}
		state, _ = loadSessionState(repo.RootPath, event.SessionID)
		if state == nil {
			return threadID, fmt.Errorf("failed to initialize session")
		}
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return state.ThreadID, err
	}

	// Track old ID before adding message
	oldThreadID := thread.ID

	// Create human message
	msg := model.NewMessage(model.RoleHuman, event.Prompt, "", nil)
	thread.AddMessage(msg)

	// If thread ID changed (first message was added), update session state and clean up
	if thread.ID != oldThreadID {
		if err := saveSessionState(repo.RootPath, event.SessionID, &SessionState{
			SessionID: state.SessionID,
			ThreadID:  thread.ID,
			StartedAt: state.StartedAt,
		}); err != nil {
			return thread.ID, err
		}
		repo.DeleteThread(oldThreadID)
		repo.UnstageThread(oldThreadID)
	}

	// Auto-stage the thread if configured
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

	// Get assistant response - either from event or parse transcript
	assistantContent := event.Response
	toolCalls := event.ToolCalls

	if assistantContent == "" && len(toolCalls) == 0 && event.Transcript != "" {
		assistantContent, toolCalls, err = parseLatestAssistantResponse(event.Transcript)
		if err != nil {
			// Log error but continue - don't fail the hook
			fmt.Fprintf(os.Stderr, "Warning: failed to parse transcript: %v\n", err)
		}
	}

	if assistantContent == "" && len(toolCalls) == 0 {
		return state.ThreadID, nil // No response to record
	}

	// Get current git hash
	gitHash, _ := repo.GetCurrentGitHash()

	// Create assistant message
	msg := model.NewMessage(model.RoleAssistant, assistantContent, "", toolCalls)
	msg.GitHashAfter = gitHash
	thread.AddMessage(msg)

	// Auto-stage if configured
	if h.config.AutoStage == nil || *h.config.AutoStage {
		contentHash := thread.ComputeContentHash()
		if err := repo.StageThread(thread.ID, len(thread.Messages), contentHash); err != nil {
			return thread.ID, err
		}
	}

	return thread.ID, repo.SaveThread(thread)
}

func (h *Handler) handleSessionEnd(repo *storage.Repository, event *agents.HookEvent) (string, error) {
	state, err := loadSessionState(repo.RootPath, event.SessionID)
	if err != nil || state == nil {
		return "", nil // No active session
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return state.ThreadID, err
	}

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

	// Store the current git hash
	gitHash, _ := repo.GetCurrentGitHash()
	thread.GitCommitHash = gitHash

	// Mark as complete if content has changed
	if thread.Status != model.ThreadStatusCommitted || thread.ComputeContentHash() != thread.CommittedContentHash {
		thread.Complete()
	}

	// Clear session state
	clearSessionState(repo.RootPath, event.SessionID)

	return thread.ID, repo.SaveThread(thread)
}

// SessionState tracks the current session for hooks
type SessionState struct {
	SessionID string    `json:"session_id"`
	ThreadID  string    `json:"thread_id"`
	StartedAt time.Time `json:"started_at"`
}

func getSessionStatePath(rootPath, sessionID string) string {
	// Use session-specific state file to support multiple concurrent sessions
	shortID := sessionID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return filepath.Join(rootPath, ".tin", fmt.Sprintf("%s-%s", stateFileName, shortID))
}

// getLegacySessionStatePath returns the old single-session state file path
func getLegacySessionStatePath(rootPath string) string {
	return filepath.Join(rootPath, ".tin", stateFileName)
}

func loadSessionState(rootPath, sessionID string) (*SessionState, error) {
	// Try new session-specific path first
	path := getSessionStatePath(rootPath, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		// Fall back to legacy path for backwards compatibility
		legacyPath := getLegacySessionStatePath(rootPath)
		data, err = os.ReadFile(legacyPath)
		if err != nil {
			return nil, err
		}
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
	// Remove session-specific file
	path := getSessionStatePath(rootPath, sessionID)
	os.Remove(path)

	// Also remove legacy file if it exists and matches this session
	legacyPath := getLegacySessionStatePath(rootPath)
	if data, err := os.ReadFile(legacyPath); err == nil {
		var state SessionState
		if json.Unmarshal(data, &state) == nil && state.SessionID == sessionID {
			os.Remove(legacyPath)
		}
	}
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

	// Split into subject line and body
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

// parseLatestAssistantResponse reads the transcript and extracts the latest assistant response
func parseLatestAssistantResponse(transcriptPath string) (string, []model.ToolCall, error) {
	if transcriptPath == "" {
		return "", nil, nil
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	var messages []transcriptMessage
	scanner := bufio.NewScanner(file)

	// Use larger buffer for big messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxBufferSize)

	for scanner.Scan() {
		var msg transcriptMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		return "", nil, nil
	}

	// Find the message ID of the last assistant message
	var lastAssistantMsgID string
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Message != nil && msg.Message.Role == "assistant" && msg.Message.ID != "" {
			lastAssistantMsgID = msg.Message.ID
			break
		}
	}

	if lastAssistantMsgID == "" {
		return "", nil, nil
	}

	// Collect all content blocks from entries with this message ID
	var textParts []string
	var toolCalls []model.ToolCall
	seenToolIDs := make(map[string]bool)

	for _, msg := range messages {
		if msg.Message == nil || msg.Message.ID != lastAssistantMsgID {
			continue
		}

		content := msg.Message.Content
		if content == nil {
			continue
		}

		contentArr, ok := content.([]interface{})
		if !ok {
			continue
		}

		for _, block := range contentArr {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := blockMap["type"].(string)

			switch blockType {
			case "text":
				if text, ok := blockMap["text"].(string); ok && text != "" {
					textParts = append(textParts, text)
				}
			case "tool_use":
				toolID, _ := blockMap["id"].(string)
				if toolID != "" && !seenToolIDs[toolID] {
					seenToolIDs[toolID] = true
					toolName, _ := blockMap["name"].(string)
					toolInput := blockMap["input"]
					inputJSON, _ := json.Marshal(toolInput)
					toolCalls = append(toolCalls, model.ToolCall{
						ID:        toolID,
						Name:      toolName,
						Arguments: inputJSON,
					})
				}
			}
		}
	}

	fullContent := strings.Join(textParts, "\n")
	return fullContent, toolCalls, nil
}

type transcriptMessage struct {
	Type    string                  `json:"type"`
	Message *transcriptMessageInner `json:"message,omitempty"`
}

type transcriptMessageInner struct {
	ID      string `json:"id,omitempty"`
	Role    string `json:"role,omitempty"`
	Content any    `json:"content,omitempty"`
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
