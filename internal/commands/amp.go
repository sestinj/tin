package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sestinj/tin/internal/model"
	"github.com/sestinj/tin/internal/storage"
)

func Amp(args []string) error {
	if len(args) == 0 {
		printAmpHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "pull":
		return ampPull(subargs)
	case "-h", "--help":
		printAmpHelp()
		return nil
	default:
		return fmt.Errorf("unknown amp subcommand: %s", subcmd)
	}
}

func ampPull(args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printAmpPullHelp()
			return nil
		}
	}

	// Determine what to pull based on arguments
	if len(args) == 0 {
		// No arguments: pull the latest thread
		return pullLatestThreads(1)
	}

	arg := args[0]

	// Check if it's a thread URL or ID
	if strings.HasPrefix(arg, "https://ampcode.com/threads/") || strings.HasPrefix(arg, "T-") {
		return pullThreadByID(arg)
	}

	// Check if it's a number
	if count, err := strconv.Atoi(arg); err == nil && count > 0 {
		return pullLatestThreads(count)
	}

	return fmt.Errorf("invalid argument: %s (expected thread URL, thread ID, or number)", arg)
}

func pullLatestThreads(count int) error {
	threadIDs, err := listAmpThreads(count)
	if err != nil {
		return err
	}

	if len(threadIDs) == 0 {
		fmt.Println("No threads found")
		return nil
	}

	for _, id := range threadIDs {
		if err := pullThreadByID(id); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to pull thread %s: %v\n", id, err)
		}
	}

	return nil
}

func pullThreadByID(idOrURL string) error {
	// Extract thread ID from URL if needed
	threadID := idOrURL
	if strings.HasPrefix(idOrURL, "https://ampcode.com/threads/") {
		threadID = strings.TrimPrefix(idOrURL, "https://ampcode.com/threads/")
	}

	// Fetch the thread markdown from amp CLI
	markdown, err := fetchAmpThreadMarkdown(threadID)
	if err != nil {
		return fmt.Errorf("failed to fetch thread %s: %w", threadID, err)
	}

	// Parse the markdown into a tin Thread
	thread, err := parseAmpMarkdown(markdown, threadID)
	if err != nil {
		return fmt.Errorf("failed to parse thread %s: %w", threadID, err)
	}

	// Open the tin repository
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return fmt.Errorf("not a tin repository (run 'tin init' first)")
	}

	// Auto-stage git changes (like Claude hooks do at session end)
	// Do this early so it happens even if thread is up-to-date
	files, gitErr := repo.GitGetChangedFiles()
	if gitErr == nil && len(files) > 0 {
		if err := repo.GitAdd(files); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stage git changes: %v\n", err)
		} else {
			fmt.Printf("Staged %d file(s)\n", len(files))
		}
	}

	// Check if thread already exists by Amp session ID (for deduplication)
	existingThreads, _ := repo.FindThreadsBySessionID(threadID)
	if len(existingThreads) > 0 {
		existing := existingThreads[0]
		newContentHash := thread.ComputeContentHash()
		existingContentHash := existing.ComputeContentHash()

		if newContentHash == existingContentHash {
			fmt.Printf("Thread %s is up to date (%d messages)\n", threadID, len(existing.Messages))
			return nil
		}

		// Thread has changed - update it, preserving the tin thread ID
		fmt.Printf("Thread %s has changed, updating...\n", threadID)
		thread.ID = existing.ID
	}

	// Save the thread
	if err := repo.SaveThread(thread); err != nil {
		return fmt.Errorf("failed to save thread: %w", err)
	}

	// Auto-stage the thread
	contentHash := thread.ComputeContentHash()
	if err := repo.StageThread(thread.ID, len(thread.Messages), contentHash); err != nil {
		return fmt.Errorf("failed to stage thread: %w", err)
	}

	fmt.Printf("Pulled thread: %s (%d messages)\n", thread.ID, len(thread.Messages))
	return nil
}

func listAmpThreads(count int) ([]string, error) {
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
			if len(threadIDs) >= count {
				break
			}
		}
	}

	return threadIDs, nil
}

func fetchAmpThreadMarkdown(threadID string) (string, error) {
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

func parseAmpMarkdown(markdown string, ampThreadID string) (*model.Thread, error) {
	thread := &model.Thread{
		Agent:    "amp",
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

	// Parse messages - collect all lines first for easier processing
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

		// Detect role headers
		if line == "## User" {
			flushMessage()
			currentRole = model.RoleHuman
			i++
			continue
		}
		if line == "## Assistant" {
			flushMessage()
			currentRole = model.RoleAssistant
			i++
			continue
		}

		// Skip tool result sections (these are user messages with tool results)
		// but continue processing after them to capture assistant text
		if strings.HasPrefix(line, "**Tool Result:**") {
			// Skip until we hit another section marker
			i++
			for i < len(lines) {
				nextLine := lines[i]
				if strings.HasPrefix(nextLine, "## ") || strings.HasPrefix(nextLine, "**Tool Use:**") || strings.HasPrefix(nextLine, "**Tool Result:**") {
					break
				}
				i++
			}
			continue
		}

		// Detect tool use
		if strings.HasPrefix(line, "**Tool Use:** `") {
			toolName := strings.TrimSuffix(strings.TrimPrefix(line, "**Tool Use:** `"), "`")
			var toolArgs strings.Builder
			i++

			// Skip empty lines between tool use header and code block
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}

			// Skip opening code fence (```json or ``` or ```anything)
			if i < len(lines) && strings.HasPrefix(lines[i], "```") {
				i++
			}

			// Collect JSON arguments until closing fence
			for i < len(lines) && lines[i] != "```" {
				toolArgs.WriteString(lines[i])
				toolArgs.WriteString("\n")
				i++
			}

			// Skip closing ```
			if i < len(lines) && lines[i] == "```" {
				i++
			}

			// Validate and store the arguments
			argsStr := strings.TrimSpace(toolArgs.String())
			var argsBytes []byte
			if argsStr != "" && argsStr[0] == '{' {
				// Try to validate as JSON, store as-is if valid
				var test interface{}
				if err := json.Unmarshal([]byte(argsStr), &test); err == nil {
					argsBytes = []byte(argsStr)
				} else {
					// Invalid JSON - store empty object with tool name hint
					argsBytes = []byte(`{"_raw": "parse error"}`)
				}
			} else {
				// Not JSON - store as empty
				argsBytes = []byte("{}")
			}

			currentToolCalls = append(currentToolCalls, model.ToolCall{
				Name:      toolName,
				Arguments: argsBytes,
			})
			continue
		}

		// Regular content - only add if we're in an assistant role
		// (user messages are just their prompts, no tool results)
		if currentRole != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
		i++
	}

	// Flush final message
	flushMessage()

	// Set thread ID based on first message (consistent with tin's model)
	if len(thread.Messages) > 0 {
		thread.ID = thread.Messages[0].ID
	} else {
		// Fallback to amp thread ID if no messages
		thread.ID = ampThreadID
	}

	// Mark as completed
	now := time.Now().UTC()
	thread.CompletedAt = &now

	return thread, nil
}

func printAmpHelp() {
	fmt.Println(`Manage Amp agent integration

Usage: tin amp <command> [arguments]

Commands:
  pull        Pull threads from Amp

Use "tin amp <command> --help" for more information about a command.`)
}

func printAmpPullHelp() {
	fmt.Println(`Pull threads from Amp into the tin repository

Usage: tin amp pull [argument]

Arguments:
  <thread-url>  Pull a specific thread by URL (e.g., https://ampcode.com/threads/T-...)
  <thread-id>   Pull a specific thread by ID (e.g., T-019b7d09-b84c-700d-81a8-dc9536e90b62)
  <number>      Pull the N most recent threads (e.g., tin amp pull 5)
  (none)        Pull the most recent thread

Examples:
  tin amp pull                                                    # Pull latest thread
  tin amp pull 3                                                  # Pull 3 latest threads
  tin amp pull T-019b7d09-b84c-700d-81a8-dc9536e90b62            # Pull specific thread
  tin amp pull https://ampcode.com/threads/T-019b7d09-...        # Pull by URL

Pulled threads are automatically staged and can be committed with 'tin commit'.`)
}
