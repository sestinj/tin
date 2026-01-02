package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dadlerj/tin/internal/model"
	"github.com/dadlerj/tin/internal/storage"
)

func Commit(args []string) error {
	var message string
	var force bool

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printCommitHelp()
			return nil
		case "-m", "--message":
			if i+1 < len(args) {
				message = args[i+1]
				i++
			}
		case "-f", "--force":
			force = true
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

	// Check tin/git state alignment (unless force)
	if !force {
		if err := repo.CheckBranchSync(); err != nil {
			if mismatch, ok := err.(*storage.BranchMismatchError); ok {
				return fmt.Errorf("%s\n\nUse 'tin sync' to align states, or 'tin commit --force' to proceed anyway", mismatch)
			}
			return err
		}
	}

	// Check if user might want to pull Amp threads first
	if !force {
		if shouldPrompt, reason := shouldPromptForAmpPull(repo); shouldPrompt {
			fmt.Printf("%s\nRun 'tin amp pull' first? [Y/n]: ", reason)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "" || response == "y" || response == "yes" {
				fmt.Println()
				if err := Amp([]string{"pull"}); err != nil {
					return fmt.Errorf("amp pull failed: %w", err)
				}
				fmt.Println()
			}
		}
	}

	// Get staged threads
	staged, err := repo.GetStagedThreads()
	if err != nil {
		return err
	}

	if len(staged) == 0 {
		return fmt.Errorf("nothing to commit. Use \"tin add\" to stage threads")
	}

	// Auto-generate commit message if not provided
	if message == "" {
		message = generateCommitMessage(repo, staged)
	}

	// Stage all changed files
	if files, err := repo.GitGetChangedFiles(); err == nil && len(files) > 0 {
		repo.GitAdd(files)
	}

	// Create git commit if there are staged changes (BEFORE tin commit)
	hasGitChanges, _ := repo.GitHasStagedChanges()
	if hasGitChanges {
		gitMsg := formatGitCommitMessage(repo, staged, message)
		if err := repo.GitCommit(gitMsg); err != nil {
			return fmt.Errorf("failed to commit git changes: %w", err)
		}
	}

	// Get git hash (from new commit or existing HEAD)
	gitHash, _ := repo.GetCurrentGitHash()

	// Get current branch and parent commit
	branch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	parentCommit, err := repo.GetBranchCommit(branch)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	var parentCommitID string
	if parentCommit != nil {
		parentCommitID = parentCommit.ID
	}

	// Create commit
	commit := model.NewTinCommit(message, staged, gitHash, parentCommitID)

	// Save commit
	if err := repo.SaveCommit(commit); err != nil {
		return err
	}

	// Update branch to point to new commit
	if err := repo.WriteBranch(branch, commit.ID); err != nil {
		return err
	}

	// Mark staged threads as committed and update their GitCommitHash
	for _, ref := range staged {
		thread, err := repo.LoadThread(ref.ThreadID)
		if err != nil {
			continue
		}
		thread.GitCommitHash = gitHash
		thread.Status = model.ThreadStatusCommitted
		thread.CommittedContentHash = thread.ComputeContentHash()
		if err := repo.SaveThread(thread); err != nil {
			continue
		}
	}

	// Clear the index
	if err := repo.ClearIndex(); err != nil {
		return err
	}

	// Print summary
	fmt.Printf("[%s %s] %s\n", branch, commit.ShortID(), truncateCommitMessage(message))
	fmt.Printf(" %d thread(s) committed\n", len(staged))

	for _, ref := range staged {
		thread, err := repo.LoadThread(ref.ThreadID)
		if err != nil {
			fmt.Printf("  - %s\n", ref.ThreadID[:8])
			continue
		}
		preview := ""
		if first := thread.FirstHumanMessage(); first != nil {
			preview = truncate(first.Content, 40)
		}
		fmt.Printf("  - %s: %s\n", ref.ThreadID[:8], preview)
	}

	return nil
}

func truncateCommitMessage(msg string) string {
	// Get first line
	if idx := strings.Index(msg, "\n"); idx != -1 {
		msg = msg[:idx]
	}
	return truncate(msg, 50)
}

func printCommitHelp() {
	fmt.Println(`Record changes to the repository

Usage: tin commit [-m <message>] [--force]

Options:
  -m, --message <msg>  Commit message (optional, auto-generated if not provided)
  -f, --force          Commit even if tin and git branches don't match
  -h, --help           Show this help message

This command creates a new commit containing all staged threads.

If no message is provided, tin will automatically generate one from the
first human message in your thread(s).

Each commit records:
  - The commit message
  - References to all staged threads
  - The current git commit hash
  - A link to the parent commit

Examples:
  tin commit                              # Auto-generate message from thread
  tin commit -m "Add user authentication" # Use explicit message`)
}

// formatGitCommitMessage creates a git commit message with thread links
func formatGitCommitMessage(repo *storage.Repository, staged []model.ThreadRef, message string) string {
	message = strings.TrimSpace(message)

	// Split message into first line and rest
	firstLine := message
	restOfMessage := ""
	if idx := strings.Index(message, "\n"); idx != -1 {
		firstLine = strings.TrimSpace(message[:idx])
		restOfMessage = strings.TrimSpace(message[idx+1:])
	}

	var builder strings.Builder

	// Subject line: [tin <thread-id>] for single thread, [tin] for multiple
	if len(staged) == 1 {
		shortID := staged[0].ThreadID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		builder.WriteString(fmt.Sprintf("[tin %s] %s", shortID, firstLine))
	} else {
		builder.WriteString(fmt.Sprintf("[tin] %s", firstLine))
	}

	// Include rest of message if multi-line
	if restOfMessage != "" {
		builder.WriteString("\n\n")
		builder.WriteString(restOfMessage)
	}

	// Add thread URLs
	if len(staged) == 1 {
		url := repo.BuildThreadURL(staged[0].ThreadID, staged[0].ContentHash)
		if url != "" {
			builder.WriteString("\n\n")
			builder.WriteString(url)
		}
	} else if len(staged) > 1 {
		builder.WriteString("\n\nThreads:")
		for _, ref := range staged {
			url := repo.BuildThreadURL(ref.ThreadID, ref.ContentHash)
			if url != "" {
				builder.WriteString(fmt.Sprintf("\n- %s", url))
			}
		}
	}

	return builder.String()
}

// generateCommitMessage creates a commit message from staged threads
func generateCommitMessage(repo *storage.Repository, staged []model.ThreadRef) string {
	if len(staged) == 0 {
		return "empty commit"
	}

	// For a single thread, use its first human message
	if len(staged) == 1 {
		thread, err := repo.LoadThread(staged[0].ThreadID)
		if err == nil {
			if first := thread.FirstHumanMessage(); first != nil {
				msg := first.Content
				// Clean up the message
				msg = strings.TrimSpace(msg)
				msg = strings.ReplaceAll(msg, "\n", " ")
				// Truncate if too long
				if len(msg) > 720 {
					msg = msg[:717] + "..."
				}
				return msg
			}
		}
		return fmt.Sprintf("thread %s", staged[0].ThreadID[:8])
	}

	// For multiple threads, summarize
	var previews []string
	for _, ref := range staged {
		thread, err := repo.LoadThread(ref.ThreadID)
		if err != nil {
			continue
		}
		if first := thread.FirstHumanMessage(); first != nil {
			preview := strings.TrimSpace(first.Content)
			preview = strings.ReplaceAll(preview, "\n", " ")
			if len(preview) > 300 {
				preview = preview[:297] + "..."
			}
			previews = append(previews, preview)
		}
	}

	if len(previews) == 0 {
		return fmt.Sprintf("%d threads", len(staged))
	}

	// Join previews, but keep total length reasonable
	result := strings.Join(previews, "; ")
	if len(result) > 720 {
		result = result[:717] + "..."
	}
	return result
}

// shouldPromptForAmpPull checks if the user might want to run tin amp pull first
// Returns (shouldPrompt, reason)
func shouldPromptForAmpPull(repo *storage.Repository) (bool, string) {
	// Only prompt if there are unstaged git changes
	files, err := repo.GitGetChangedFiles()
	if err == nil && len(files) > 0 {
		return true, fmt.Sprintf("You have %d unstaged file(s).", len(files))
	}
	return false, ""
}
