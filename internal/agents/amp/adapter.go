// Package amp provides the AMP (Sourcegraph) agent integration for Tin.
// It implements the PullAdapter interface for batch thread import.
package amp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/dadlerj/tin/internal/agents"
	"github.com/dadlerj/tin/internal/model"
)

const (
	agentName        = "amp"
	agentDisplayName = "AMP (Sourcegraph)"
)

// Adapter implements agents.PullAdapter for AMP
type Adapter struct{}

// NewAdapter creates a new AMP adapter
func NewAdapter() *Adapter {
	return &Adapter{}
}

// Info returns metadata about the AMP agent
func (a *Adapter) Info() agents.AgentInfo {
	return agents.AgentInfo{
		Name:        agentName,
		DisplayName: agentDisplayName,
		Paradigm:    agents.ParadigmPull,
		Version:     "1.0.0",
	}
}

// List returns available thread IDs from AMP
func (a *Adapter) List(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	cmd := exec.Command("amp", "threads", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list amp threads: %w", err)
	}

	// Parse the output to extract thread IDs
	// Format: Title | Last Updated | Visibility | Messages | Thread ID
	var threadIDs []string
	lines := strings.Split(string(output), "\n")

	// Skip header lines (first two lines are header and separator)
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		// Extract the thread ID (last column, starts with T-)
		if idx := strings.LastIndex(line, "T-"); idx != -1 {
			threadID := strings.TrimSpace(line[idx:])
			threadIDs = append(threadIDs, threadID)
			if len(threadIDs) >= limit {
				break
			}
		}
	}

	return threadIDs, nil
}

// Pull fetches a specific thread by ID
func (a *Adapter) Pull(threadID string, opts agents.PullOptions) (*model.Thread, error) {
	// Extract thread ID from URL if needed
	if strings.HasPrefix(threadID, "https://ampcode.com/threads/") {
		threadID = strings.TrimPrefix(threadID, "https://ampcode.com/threads/")
	}

	// Fetch the thread markdown from amp CLI
	markdown, err := fetchThreadMarkdown(threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thread %s: %w", threadID, err)
	}

	// Parse the markdown into a tin Thread
	thread, err := parseMarkdown(markdown, threadID, opts.IncludeGit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse thread %s: %w", threadID, err)
	}

	return thread, nil
}

// PullRecent fetches the N most recent threads
func (a *Adapter) PullRecent(count int, opts agents.PullOptions) ([]*model.Thread, error) {
	threadIDs, err := a.List(count)
	if err != nil {
		return nil, err
	}

	var threads []*model.Thread
	for _, id := range threadIDs {
		thread, err := a.Pull(id, opts)
		if err != nil {
			// Log error but continue with other threads
			fmt.Fprintf(os.Stderr, "Warning: failed to pull thread %s: %v\n", id, err)
			continue
		}
		threads = append(threads, thread)
	}

	return threads, nil
}

func fetchThreadMarkdown(threadID string) (string, error) {
	// Write to temp file to work around amp CLI truncating output when piped
	tmpFile, err := os.CreateTemp("", "amp-thread-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Use shell redirection to write to file
	cmd := exec.Command("sh", "-c", fmt.Sprintf("amp threads markdown %s > %s", threadID, tmpPath))
	if err := cmd.Run(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseMarkdown(markdown string, ampThreadID string, includeGit bool) (*model.Thread, error) {
	thread := &model.Thread{
		Agent:    agentName,
		Status:   model.ThreadStatusCompleted,
		Messages: []model.Message{},
	}

	// Parse YAML frontmatter
	if strings.HasPrefix(markdown, "---") {
		endIdx := strings.Index(markdown[3:], "---")
		if endIdx != -1 {
			frontmatter := markdown[3 : endIdx+3]
			markdown = markdown[endIdx+6:] // Skip past closing ---

			// Extract created timestamp
			if match := regexp.MustCompile(`created:\s*(.+)`).FindStringSubmatch(frontmatter); len(match) > 1 {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(match[1])); err == nil {
					thread.StartedAt = t
				}
			}

			// Store the original Amp thread ID in AgentSessionID
			thread.AgentSessionID = ampThreadID
		}
	}

	// Parse messages
	lines := strings.Split(markdown, "\n")
	var currentRole model.Role
	var currentContent strings.Builder
	var currentToolCalls []model.ToolCall
	i := 0

	flushMessage := func() {
		if currentRole != "" {
			content := strings.TrimSpace(currentContent.String())
			if content != "" || len(currentToolCalls) > 0 {
				msg := model.NewMessage(currentRole, content, "", currentToolCalls)
				thread.AddMessage(msg)
			}
		}
		currentContent.Reset()
		currentToolCalls = nil
	}

	for i < len(lines) {
		line := lines[i]

		// Detect role headers (more robust matching)
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "## User" || strings.HasPrefix(trimmedLine, "## User ") {
			flushMessage()
			currentRole = model.RoleHuman
			i++
			continue
		}
		if trimmedLine == "## Assistant" || strings.HasPrefix(trimmedLine, "## Assistant ") {
			flushMessage()
			currentRole = model.RoleAssistant
			i++
			continue
		}

		// Parse tool result sections - now we preserve them!
		if strings.HasPrefix(trimmedLine, "**Tool Result:**") {
			var resultContent strings.Builder
			i++

			// Check for code block
			if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				i++ // Skip opening fence
				for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					resultContent.WriteString(lines[i])
					resultContent.WriteString("\n")
					i++
				}
				if i < len(lines) {
					i++ // Skip closing fence
				}
			} else {
				// Collect until next section marker
				for i < len(lines) {
					nextLine := lines[i]
					nextTrimmed := strings.TrimSpace(nextLine)
					if strings.HasPrefix(nextTrimmed, "## ") ||
						strings.HasPrefix(nextTrimmed, "**Tool Use:**") ||
						strings.HasPrefix(nextTrimmed, "**Tool Result:**") {
						break
					}
					resultContent.WriteString(nextLine)
					resultContent.WriteString("\n")
					i++
				}
			}

			// Associate result with the last tool call if available
			result := strings.TrimSpace(resultContent.String())
			if len(currentToolCalls) > 0 && result != "" {
				// Update the last tool call with its result
				lastIdx := len(currentToolCalls) - 1
				currentToolCalls[lastIdx].Result = result
			}
			continue
		}

		// Detect tool use
		if strings.HasPrefix(trimmedLine, "**Tool Use:** `") {
			toolName := strings.TrimSuffix(strings.TrimPrefix(trimmedLine, "**Tool Use:** `"), "`")
			var toolArgs strings.Builder
			i++

			// Skip empty lines between tool use header and code block
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}

			// Skip opening code fence
			if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				i++
			}

			// Collect JSON arguments until closing fence
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				toolArgs.WriteString(lines[i])
				toolArgs.WriteString("\n")
				i++
			}

			// Skip closing fence
			if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				i++
			}

			// Validate and store the arguments
			argsStr := strings.TrimSpace(toolArgs.String())
			var argsBytes []byte
			if argsStr != "" && len(argsStr) > 0 && argsStr[0] == '{' {
				var test interface{}
				if err := json.Unmarshal([]byte(argsStr), &test); err == nil {
					argsBytes = []byte(argsStr)
				} else {
					argsBytes = []byte(`{"_error": "parse error"}`)
				}
			} else {
				argsBytes = []byte("{}")
			}

			currentToolCalls = append(currentToolCalls, model.ToolCall{
				Name:      toolName,
				Arguments: argsBytes,
			})
			continue
		}

		// Regular content
		if currentRole != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
		i++
	}

	// Flush final message
	flushMessage()

	// Set thread ID based on first message
	if len(thread.Messages) > 0 {
		thread.ID = thread.Messages[0].ID
	} else {
		thread.ID = ampThreadID
	}

	// Mark as completed
	now := time.Now().UTC()
	thread.CompletedAt = &now

	// Record git hash if requested
	if includeGit {
		if hash := getCurrentGitHash(); hash != "" {
			thread.GitCommitHash = hash
		}
	}

	return thread, nil
}

func getCurrentGitHash() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// Register this adapter with the global registry
func init() {
	agents.Register(NewAdapter())
}
