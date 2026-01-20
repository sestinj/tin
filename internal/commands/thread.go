package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sestinj/tin/internal/model"
	"github.com/sestinj/tin/internal/storage"
)

func Thread(args []string) error {
	if len(args) == 0 {
		printThreadHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "list":
		return threadList(subargs)
	case "show":
		return threadShow(subargs)
	case "start":
		return threadStart(subargs)
	case "append":
		return threadAppend(subargs)
	case "complete":
		return threadComplete(subargs)
	case "delete":
		return threadDelete(subargs)
	case "-h", "--help":
		printThreadHelp()
		return nil
	default:
		return fmt.Errorf("unknown thread subcommand: %s", subcmd)
	}
}

func threadList(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	threads, err := repo.ListThreads()
	if err != nil {
		return err
	}

	if len(threads) == 0 {
		fmt.Println("No threads found")
		return nil
	}

	fmt.Printf("%-10s %-12s %-10s %-8s %s\n", "ID", "AGENT", "STATUS", "MSGS", "PREVIEW")
	fmt.Println(strings.Repeat("-", 80))

	for _, t := range threads {
		preview := ""
		if first := t.FirstHumanMessage(); first != nil {
			preview = truncate(first.Content, 35)
		}

		// Add continuation indicator if thread has a parent
		idDisplay := t.ID[:8]
		if t.ParentThreadID != "" {
			idDisplay = "↳ " + t.ID[:8]
		}

		fmt.Printf("%-10s %-12s %-10s %-8d %s\n",
			idDisplay,
			truncate(t.Agent, 12),
			t.Status,
			len(t.Messages),
			preview,
		)
	}

	return nil
}

func threadShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("thread ID required")
	}

	threadID := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	// Find thread by prefix
	thread, err := findThreadByPrefix(repo, threadID)
	if err != nil {
		return err
	}

	fmt.Printf("Thread: %s\n", thread.ID)
	fmt.Printf("Agent: %s\n", thread.Agent)
	fmt.Printf("Status: %s\n", thread.Status)
	fmt.Printf("Started: %s\n", thread.StartedAt.Format(time.RFC3339))
	if thread.CompletedAt != nil {
		fmt.Printf("Completed: %s\n", thread.CompletedAt.Format(time.RFC3339))
	}
	if thread.ParentThreadID != "" {
		fmt.Printf("Continues: %s\n", thread.ParentThreadID[:min(8, len(thread.ParentThreadID))])
	}
	fmt.Printf("Messages: %d\n", len(thread.Messages))
	fmt.Println(strings.Repeat("-", 60))

	for i, msg := range thread.Messages {
		role := "Human"
		if msg.Role == model.RoleAssistant {
			role = "Assistant"
		}
		fmt.Printf("\n[%d] %s (%s)\n", i+1, role, msg.Timestamp.Format("15:04:05"))

		if len(msg.ToolCalls) > 0 {
			// Group tool calls by type and summarize
			toolSummary := summarizeToolCalls(msg.ToolCalls)
			if toolSummary != "" {
				fmt.Println(toolSummary)
			}
		}

		if msg.Content != "" {
			fmt.Println(msg.Content)
		}

		if msg.GitHashAfter != "" {
			fmt.Printf("\n  Git state: %s\n", msg.GitHashAfter[:8])
		}
	}

	return nil
}

func threadStart(args []string) error {
	agent := "unknown"
	sessionID := ""

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		case "--session-id":
			if i+1 < len(args) {
				sessionID = args[i+1]
				i++
			}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	// Prune any empty threads before creating a new one
	repo.PruneEmptyThreads()

	thread := model.NewThread(agent, sessionID, "", "")

	// We need at least one message to generate the thread ID
	// For now, we'll save without an ID and update when first message is added
	// Actually, let's generate a temporary ID
	thread.ID = fmt.Sprintf("pending-%d", time.Now().UnixNano())

	if err := repo.SaveThread(thread); err != nil {
		return err
	}

	fmt.Printf("Started thread: %s\n", thread.ID)
	return nil
}

func threadAppend(args []string) error {
	var threadID, role, content, gitHash string
	var toolCallsJSON string

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--thread":
			if i+1 < len(args) {
				threadID = args[i+1]
				i++
			}
		case "--role":
			if i+1 < len(args) {
				role = args[i+1]
				i++
			}
		case "--content":
			if i+1 < len(args) {
				content = args[i+1]
				i++
			}
		case "--git-hash":
			if i+1 < len(args) {
				gitHash = args[i+1]
				i++
			}
		case "--tool-calls":
			if i+1 < len(args) {
				toolCallsJSON = args[i+1]
				i++
			}
		}
	}

	if threadID == "" {
		return fmt.Errorf("--thread is required")
	}
	if role == "" {
		return fmt.Errorf("--role is required")
	}
	if content == "" {
		return fmt.Errorf("--content is required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	thread, err := findThreadByPrefix(repo, threadID)
	if err != nil {
		return err
	}

	// Parse tool calls if provided
	var toolCalls []model.ToolCall
	if toolCallsJSON != "" {
		if err := json.Unmarshal([]byte(toolCallsJSON), &toolCalls); err != nil {
			return fmt.Errorf("invalid tool calls JSON: %w", err)
		}
	}

	// Create message
	msgRole := model.RoleHuman
	if role == "assistant" {
		msgRole = model.RoleAssistant
	}

	msg := model.NewMessage(msgRole, content, "", toolCalls)
	msg.GitHashAfter = gitHash

	// Add to thread
	thread.AddMessage(msg)

	// Save thread (this may update thread ID if it was the first message)
	if err := repo.SaveThread(thread); err != nil {
		return err
	}

	fmt.Printf("Appended message to thread %s (message %d)\n", thread.ID[:8], len(thread.Messages))
	return nil
}

func threadComplete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("thread ID required")
	}

	threadID := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	thread, err := findThreadByPrefix(repo, threadID)
	if err != nil {
		return err
	}

	thread.Complete()

	if err := repo.SaveThread(thread); err != nil {
		return err
	}

	fmt.Printf("Thread %s marked as completed\n", thread.ID[:8])
	return nil
}

func threadDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("thread ID required")
	}

	// Parse flags
	force := false
	var threadID string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--force":
			force = true
		default:
			if threadID == "" {
				threadID = args[i]
			}
		}
	}

	if threadID == "" {
		return fmt.Errorf("thread ID required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	thread, err := findThreadByPrefix(repo, threadID)
	if err != nil {
		return err
	}

	// Check if thread is active
	if thread.Status == model.ThreadStatusActive && !force {
		return fmt.Errorf("cannot delete active thread %s (use --force to override)", thread.ID[:8])
	}

	// Check if thread is committed
	isCommitted, err := repo.ThreadIsCommitted(thread.ID)
	if err != nil {
		return err
	}
	if isCommitted && !force {
		return fmt.Errorf("cannot delete committed thread %s (use --force to override)", thread.ID[:8])
	}

	// Remove from staging if staged
	if err := repo.UnstageThread(thread.ID); err != nil {
		return err
	}

	// Delete the thread
	if err := repo.DeleteThread(thread.ID); err != nil {
		return err
	}

	fmt.Printf("Deleted thread %s\n", thread.ID[:8])
	return nil
}

func findThreadByPrefix(repo *storage.Repository, prefix string) (*model.Thread, error) {
	// First try exact match
	thread, err := repo.LoadThread(prefix)
	if err == nil {
		return thread, nil
	}

	// Try prefix match
	threads, err := repo.ListThreads()
	if err != nil {
		return nil, err
	}

	var matches []*model.Thread
	for _, t := range threads {
		if strings.HasPrefix(t.ID, prefix) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("thread not found: %s", prefix)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous thread prefix: %s (matches %d threads)", prefix, len(matches))
	}

	return matches[0], nil
}

// summarizeToolCalls creates a readable summary of tool calls
// Extracts file paths, patterns, and other useful info from arguments
func summarizeToolCalls(toolCalls []model.ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}

	// Group by tool type
	reads := []string{}
	edits := []string{}
	creates := []string{}
	greps := []string{}
	globs := []string{}
	bashes := []string{}
	webSearches := []string{}
	webReads := []string{}
	finders := []string{}
	others := []string{}

	for _, tc := range toolCalls {
		var args map[string]interface{}
		if len(tc.Arguments) > 0 {
			json.Unmarshal(tc.Arguments, &args)
		}

		switch tc.Name {
		case "Read":
			if path, ok := args["path"].(string); ok {
				reads = append(reads, shortenPath(path))
			} else {
				reads = append(reads, "?")
			}
		case "edit_file":
			if path, ok := args["path"].(string); ok {
				edits = append(edits, shortenPath(path))
			} else {
				edits = append(edits, "?")
			}
		case "create_file":
			if path, ok := args["path"].(string); ok {
				creates = append(creates, shortenPath(path))
			} else {
				creates = append(creates, "?")
			}
		case "Grep":
			pattern := ""
			if p, ok := args["pattern"].(string); ok {
				pattern = p
			}
			path := ""
			if p, ok := args["path"].(string); ok {
				path = shortenPath(p)
			}
			if pattern != "" {
				if path != "" {
					greps = append(greps, fmt.Sprintf("\"%s\" in %s", truncate(pattern, 20), path))
				} else {
					greps = append(greps, fmt.Sprintf("\"%s\"", truncate(pattern, 30)))
				}
			} else {
				greps = append(greps, "?")
			}
		case "glob":
			if pattern, ok := args["filePattern"].(string); ok {
				globs = append(globs, pattern)
			} else {
				globs = append(globs, "?")
			}
		case "Bash":
			if cmd, ok := args["cmd"].(string); ok {
				bashes = append(bashes, truncate(cmd, 40))
			} else {
				bashes = append(bashes, "?")
			}
		case "web_search":
			if obj, ok := args["objective"].(string); ok {
				webSearches = append(webSearches, truncate(obj, 40))
			} else {
				webSearches = append(webSearches, "?")
			}
		case "read_web_page":
			if url, ok := args["url"].(string); ok {
				webReads = append(webReads, truncate(url, 50))
			} else {
				webReads = append(webReads, "?")
			}
		case "finder":
			if query, ok := args["query"].(string); ok {
				finders = append(finders, truncate(query, 40))
			} else {
				finders = append(finders, "?")
			}
		default:
			others = append(others, tc.Name)
		}
	}

	var lines []string

	if len(reads) > 0 {
		lines = append(lines, fmt.Sprintf("  📖 Read: %s", strings.Join(reads, ", ")))
	}
	if len(edits) > 0 {
		lines = append(lines, fmt.Sprintf("  ✏️  Edit: %s", strings.Join(edits, ", ")))
	}
	if len(creates) > 0 {
		lines = append(lines, fmt.Sprintf("  📝 Create: %s", strings.Join(creates, ", ")))
	}
	if len(greps) > 0 {
		lines = append(lines, fmt.Sprintf("  🔍 Grep: %s", strings.Join(greps, "; ")))
	}
	if len(globs) > 0 {
		lines = append(lines, fmt.Sprintf("  📁 Glob: %s", strings.Join(globs, ", ")))
	}
	if len(bashes) > 0 {
		lines = append(lines, fmt.Sprintf("  💻 Bash: %s", strings.Join(bashes, "; ")))
	}
	if len(finders) > 0 {
		lines = append(lines, fmt.Sprintf("  🔎 Finder: %s", strings.Join(finders, "; ")))
	}
	if len(webSearches) > 0 {
		lines = append(lines, fmt.Sprintf("  🌐 Web search: %s", strings.Join(webSearches, "; ")))
	}
	if len(webReads) > 0 {
		lines = append(lines, fmt.Sprintf("  🌐 Web read: %s", strings.Join(webReads, "; ")))
	}
	if len(others) > 0 {
		// Deduplicate and count others
		counts := make(map[string]int)
		for _, o := range others {
			counts[o]++
		}
		var otherSummary []string
		for name, count := range counts {
			if count > 1 {
				otherSummary = append(otherSummary, fmt.Sprintf("%s×%d", name, count))
			} else {
				otherSummary = append(otherSummary, name)
			}
		}
		lines = append(lines, fmt.Sprintf("  🔧 Other: %s", strings.Join(otherSummary, ", ")))
	}

	return strings.Join(lines, "\n")
}

// shortenPath removes common prefixes and returns just the relevant part of a path
func shortenPath(path string) string {
	// Get base parts - try to show last 2-3 components
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= 3 {
		return path
	}
	// Return last 2 parts
	return strings.Join(parts[len(parts)-2:], "/")
}

func printThreadHelp() {
	fmt.Println(`Manage threads

Usage: tin thread <command> [arguments]

Commands:
  list                 List all threads
  show <id>            Show details of a thread
  start                Start a new thread (used by hooks)
  append               Append a message to a thread (used by hooks)
  complete <id>        Mark a thread as completed
  delete <id>          Delete a thread and its changes

Start options:
  --agent <name>       Agent name (e.g., claude-code, cursor)
  --session-id <id>    Agent session ID for resume capability

Append options:
  --thread <id>        Thread ID to append to (required)
  --role <role>        Message role: human or assistant (required)
  --content <text>     Message content (required)
  --git-hash <hash>    Git commit hash after this message
  --tool-calls <json>  Tool calls as JSON array

Delete options:
  -f, --force          Force deletion of active or committed threads`)
}
