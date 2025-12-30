package hooks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

// HookInput represents the common fields from Claude Code hooks
type HookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Prompt         string `json:"prompt,omitempty"`         // For UserPromptSubmit
	ToolName       string `json:"tool_name,omitempty"`      // For PostToolUse
	ToolInput      any    `json:"tool_input,omitempty"`     // For PostToolUse
	ToolResponse   string `json:"tool_response,omitempty"`  // For PostToolUse
}

// TranscriptMessage represents a message in the Claude Code transcript
type TranscriptMessage struct {
	Type    string                  `json:"type"`
	Message *TranscriptMessageInner `json:"message,omitempty"`
}

// TranscriptMessageInner represents the nested message content
type TranscriptMessageInner struct {
	ID      string `json:"id,omitempty"`        // Message ID for grouping incremental updates
	Role    string `json:"role,omitempty"`
	Content any    `json:"content,omitempty"` // Can be string or array of content blocks
}

// SessionState tracks the current session for hooks
type SessionState struct {
	SessionID string `json:"session_id"`
	ThreadID  string `json:"thread_id"`
}

const stateFileName = ".tin-session"

// HandleSessionStart handles the SessionStart hook event
func HandleSessionStart(input *HookInput) error {
	repo, err := storage.Open(input.Cwd)
	if err != nil {
		// Not a tin repo, skip silently
		return nil
	}

	// Check if we already have a thread for this session
	state, _ := loadSessionState(repo.RootPath)
	if state != nil && state.SessionID == input.SessionID {
		// Already tracking this session
		return nil
	}

	// Prune any empty threads before creating a new one
	repo.PruneEmptyThreads()

	// Create a new thread
	thread := model.NewThread("claude-code", input.SessionID, "", "")

	// Generate a temporary ID until first message
	thread.ID = fmt.Sprintf("cc-%s", input.SessionID[:min(12, len(input.SessionID))])

	if err := repo.SaveThread(thread); err != nil {
		return err
	}

	// Save session state using repo root path (not input.Cwd which may be a subdirectory)
	if err := saveSessionState(repo.RootPath, &SessionState{
		SessionID: input.SessionID,
		ThreadID:  thread.ID,
	}); err != nil {
		return err
	}

	return nil
}

// HandleUserPromptSubmit handles the UserPromptSubmit hook event
func HandleUserPromptSubmit(input *HookInput) error {
	if input.Prompt == "" {
		return nil
	}

	repo, err := storage.Open(input.Cwd)
	if err != nil {
		return nil // Not a tin repo
	}

	state, err := loadSessionState(repo.RootPath)
	if err != nil || state == nil {
		// No active session, create one
		if err := HandleSessionStart(input); err != nil {
			return err
		}
		state, _ = loadSessionState(repo.RootPath)
		if state == nil {
			return fmt.Errorf("failed to initialize session")
		}
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return err
	}

	// Track old ID before adding message (first message changes the thread ID)
	oldThreadID := thread.ID

	// Create human message
	msg := model.NewMessage(model.RoleHuman, input.Prompt, "", nil)
	thread.AddMessage(msg)

	// If thread ID changed (first message was added), update session state and clean up
	if thread.ID != oldThreadID {
		// Update session state with new thread ID
		if err := saveSessionState(repo.RootPath, &SessionState{
			SessionID: state.SessionID,
			ThreadID:  thread.ID,
		}); err != nil {
			return err
		}
		// Remove old temporary thread file
		repo.DeleteThread(oldThreadID)
		// Unstage old thread ID if it was staged
		repo.UnstageThread(oldThreadID)
	}

	// Auto-stage the thread
	if err := repo.StageThread(thread.ID, len(thread.Messages)); err != nil {
		return err
	}

	return repo.SaveThread(thread)
}

// HandleStop handles the Stop hook event (assistant finished responding)
func HandleStop(input *HookInput) error {
	repo, err := storage.Open(input.Cwd)
	if err != nil {
		return nil // Not a tin repo
	}

	state, err := loadSessionState(repo.RootPath)
	if err != nil || state == nil {
		return nil // No active session
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return err
	}

	// Read transcript to get latest assistant response
	assistantContent, toolCalls, err := getLatestAssistantResponse(input.TranscriptPath)
	if err != nil {
		return err
	}

	if assistantContent == "" && len(toolCalls) == 0 {
		return nil // No response to record
	}

	// Get current git hash
	gitHash, _ := repo.GetCurrentGitHash()

	// Create assistant message
	msg := model.NewMessage(model.RoleAssistant, assistantContent, "", toolCalls)
	msg.GitHashAfter = gitHash
	thread.AddMessage(msg)

	// Auto-stage the thread
	if err := repo.StageThread(thread.ID, len(thread.Messages)); err != nil {
		return err
	}

	return repo.SaveThread(thread)
}

// HandleSessionEnd handles the SessionEnd hook event
func HandleSessionEnd(input *HookInput) error {
	repo, err := storage.Open(input.Cwd)
	if err != nil {
		return nil // Not a tin repo
	}

	state, err := loadSessionState(repo.RootPath)
	if err != nil || state == nil {
		return nil // No active session
	}

	thread, err := repo.LoadThread(state.ThreadID)
	if err != nil {
		return err
	}

	// Get all changed files from git status (respects .gitignore, excludes .tin/)
	files, err := repo.GitGetChangedFiles()
	if err == nil && len(files) > 0 {
		// Stage the files
		if err := repo.GitAdd(files); err == nil {
			// Check if there are actually staged changes
			hasChanges, err := repo.GitHasStagedChanges()
			if err == nil && hasChanges {
				// Create git commit with thread info
				commitMsg := formatGitCommitMessage(thread)
				repo.GitCommit(commitMsg)
			}
		}
	}

	// Store the current git hash (either new commit or existing HEAD)
	gitHash, _ := repo.GetCurrentGitHash()
	thread.GitCommitHash = gitHash

	// Only mark as complete if thread content has changed since last commit
	// If status is committed and content hash matches, keep committed status
	if thread.Status != model.ThreadStatusCommitted || thread.ComputeContentHash() != thread.CommittedContentHash {
		thread.Complete()
	}

	// Clear session state
	clearSessionState(repo.RootPath)

	return repo.SaveThread(thread)
}

// formatGitCommitMessage creates a git commit message for a thread
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

	// Build commit message with subject and optional body
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[tin %s] %s", shortID, firstLine))

	if restOfMessage != "" {
		builder.WriteString("\n\n")
		builder.WriteString(restOfMessage)
	}

	return builder.String()
}

// getLatestAssistantResponse reads the transcript and extracts the latest assistant response
func getLatestAssistantResponse(transcriptPath string) (string, []model.ToolCall, error) {
	if transcriptPath == "" {
		return "", nil, nil
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	var messages []TranscriptMessage
	scanner := bufio.NewScanner(file)

	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var msg TranscriptMessage
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

	// Collect ALL content blocks from ALL entries with this message ID
	// (transcript streams incremental updates, each entry has only new blocks)
	var textParts []string
	var toolCalls []model.ToolCall
	seenToolIDs := make(map[string]bool) // Avoid duplicate tool calls

	for _, msg := range messages {
		if msg.Message == nil || msg.Message.ID != lastAssistantMsgID {
			continue
		}

		content := msg.Message.Content
		if content == nil {
			continue
		}

		// Content is an array of content blocks
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

func loadSessionState(cwd string) (*SessionState, error) {
	path := filepath.Join(cwd, ".tin", stateFileName)
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

func saveSessionState(cwd string, state *SessionState) error {
	path := filepath.Join(cwd, ".tin", stateFileName)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func clearSessionState(cwd string) {
	path := filepath.Join(cwd, ".tin", stateFileName)
	os.Remove(path)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
