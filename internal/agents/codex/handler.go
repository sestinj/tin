// Package codex provides the OpenAI Codex CLI agent integration for Tin.
// It implements the NotifyHandler interface for notification-based tracking.
//
// Codex CLI only supports the "agent-turn-complete" notification event,
// which provides limited real-time tracking compared to hook-based agents.
package codex

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
	agentName        = "codex"
	agentDisplayName = "Codex CLI"
	stateFileName    = ".tin-codex-session"
)

// Handler implements agents.NotifyHandler for Codex CLI
type Handler struct {
	config *agents.Config
}

// NewHandler creates a new Codex handler with the given config.
func NewHandler(config *agents.Config) *Handler {
	if config == nil {
		config = agents.DefaultConfig()
	}
	return &Handler{config: config}
}

// Info returns metadata about the Codex agent
func (h *Handler) Info() agents.AgentInfo {
	return agents.AgentInfo{
		Name:        agentName,
		DisplayName: agentDisplayName,
		Paradigm:    agents.ParadigmNotify,
		Version:     "1.0.0",
	}
}

// Setup configures notification handling for Codex.
// This prints instructions for the user to configure their config.toml.
func (h *Handler) Setup(projectDir string) error {
	// Find tin binary path
	tinPath, err := findTinBinary()
	if err != nil {
		return err
	}

	fmt.Println("To enable Codex CLI integration, add the following to your Codex config.toml:")
	fmt.Println()
	fmt.Printf("notify = [\"%s\", \"hook\", \"codex-notify\"]\n", tinPath)
	fmt.Println()
	fmt.Println("Config file locations:")
	fmt.Println("  - macOS/Linux: ~/.codex/config.toml")
	os.Stdout.WriteString("  - Windows: %USERPROFILE%\\.codex\\config.toml\n")
	fmt.Println()
	fmt.Println("After adding, Codex will notify tin when agent turns complete.")

	return nil
}

// HandleNotification processes a notification event from Codex.
// Codex sends a JSON payload as the first argument to the notify command.
func (h *Handler) HandleNotification(event *agents.NotifyEvent) error {
	// Parse Codex-specific payload
	var payload CodexNotifyPayload
	if err := json.Unmarshal(event.RawPayload, &payload); err != nil {
		return fmt.Errorf("failed to parse Codex notification: %w", err)
	}

	// Only handle agent-turn-complete events
	if payload.Type != "agent-turn-complete" {
		return nil
	}

	repo, err := storage.Open(payload.Cwd)
	if err != nil {
		// Not a tin repo
		return nil
	}

	// Try to load or create thread for this session
	thread, err := h.getOrCreateThread(repo, payload.ThreadID)
	if err != nil {
		return err
	}

	// Track old ID before adding messages (first message changes the thread ID)
	oldThreadID := thread.ID

	// Count existing human messages to avoid duplicates
	// (Codex sends ALL input messages on each notification, not just new ones)
	existingHumanCount := 0
	for _, m := range thread.Messages {
		if m.Role == model.RoleHuman {
			existingHumanCount++
		}
	}

	// Only add new messages (those beyond what we've already stored)
	for i, inputMsg := range payload.InputMessages {
		if i < existingHumanCount {
			continue // Already have this message
		}
		msg := model.NewMessage(model.RoleHuman, inputMsg, "", nil)
		thread.AddMessage(msg)
	}

	// If thread ID changed (first message was added), clean up old temp thread
	if thread.ID != oldThreadID {
		repo.DeleteThread(oldThreadID)
		repo.UnstageThread(oldThreadID)
	}

	if payload.LastAssistantMessage != "" {
		// Get current git hash
		gitHash, _ := repo.GetCurrentGitHash()
		msg := model.NewMessage(model.RoleAssistant, payload.LastAssistantMessage, "", nil)
		msg.GitHashAfter = gitHash
		thread.AddMessage(msg)
	}

	// Mark as complete since turn is done
	thread.Complete()

	// Auto-stage if configured
	if h.config.AutoStage == nil || *h.config.AutoStage {
		contentHash := thread.ComputeContentHash()
		if err := repo.StageThread(thread.ID, len(thread.Messages), contentHash); err != nil {
			return err
		}
	}

	return repo.SaveThread(thread)
}

// SyncThread fetches and updates a specific thread by session ID.
// For Codex, this is a no-op since we don't have access to session data
// outside of notifications.
func (h *Handler) SyncThread(sessionID string, cwd string) (*model.Thread, error) {
	repo, err := storage.Open(cwd)
	if err != nil {
		return nil, err
	}

	threads, err := repo.FindThreadsBySessionID(sessionID)
	if err != nil {
		return nil, err
	}

	if len(threads) == 0 {
		return nil, fmt.Errorf("thread not found: %s", sessionID)
	}

	return threads[0], nil
}

func (h *Handler) getOrCreateThread(repo *storage.Repository, threadID string) (*model.Thread, error) {
	// Check for existing thread
	threads, _ := repo.FindThreadsBySessionID(threadID)
	if len(threads) > 0 {
		return threads[0], nil
	}

	// Create new thread
	thread := model.NewThread(agentName, threadID, "", "")
	thread.ID = fmt.Sprintf("codex-%s", threadID[:min(12, len(threadID))])

	if err := repo.SaveThread(thread); err != nil {
		return nil, err
	}

	return thread, nil
}

// CodexNotifyPayload represents the JSON payload sent by Codex CLI
// on agent-turn-complete notifications.
type CodexNotifyPayload struct {
	Type                 string   `json:"type"`                   // "agent-turn-complete"
	ThreadID             string   `json:"thread-id"`              // Session identifier
	TurnID               string   `json:"turn-id"`                // Turn identifier
	Cwd                  string   `json:"cwd"`                    // Working directory
	InputMessages        []string `json:"input-messages"`         // User messages
	LastAssistantMessage string   `json:"last-assistant-message"` // Assistant response
}

// ParseNotifyArgs parses the command-line argument from Codex notify command.
// Codex passes a single JSON argument.
func ParseNotifyArgs(args []string) (*agents.NotifyEvent, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("expected JSON argument")
	}

	var payload CodexNotifyPayload
	if err := json.Unmarshal([]byte(args[0]), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse Codex notification: %w", err)
	}

	return &agents.NotifyEvent{
		Type:       agents.NotifyEventTurnComplete,
		SessionID:  payload.ThreadID,
		Cwd:        payload.Cwd,
		Timestamp:  time.Now().UTC(),
		Message:    payload.LastAssistantMessage,
		RawPayload: []byte(args[0]),
	}, nil
}

// SessionState tracks the current session
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

func findTinBinary() (string, error) {
	// Try to find in PATH
	path, err := findInPath("tin")
	if err == nil {
		return filepath.Abs(path)
	}

	// Try current executable
	exe, err := os.Executable()
	if err == nil {
		return filepath.Abs(exe)
	}

	return "", fmt.Errorf("could not find tin binary")
}

func findInPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("not found in PATH")
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
