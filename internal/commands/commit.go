package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

func Commit(args []string) error {
	var message string

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

	// For any staged threads without a git commit (still in progress),
	// create a git commit for their changes now
	for _, ref := range staged {
		thread, err := repo.LoadThread(ref.ThreadID)
		if err != nil {
			continue
		}

		if thread.GitCommitHash == "" {
			// Thread is still in progress - create git commit for its changes
			if err := commitThreadChanges(repo, thread, ref.MessageCount); err != nil {
				// Log but don't fail
			}
		}
	}

	// Get git hash from the latest staged thread
	var gitHash string
	for i := len(staged) - 1; i >= 0; i-- {
		thread, err := repo.LoadThread(staged[i].ThreadID)
		if err != nil {
			continue
		}
		if thread.GitCommitHash != "" {
			gitHash = thread.GitCommitHash
			break
		}
	}

	// Fallback to current HEAD if no thread has a git hash (backward compatibility)
	if gitHash == "" {
		gitHash, _ = repo.GetCurrentGitHash()
	}

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

	// Mark staged threads as committed
	for _, ref := range staged {
		thread, err := repo.LoadThread(ref.ThreadID)
		if err != nil {
			continue
		}
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

	// Create git commit with tin commit link
	// Always create a commit if we have a thread host URL configured,
	// even if there are no file changes (to record the tin commit link)
	hasGitChanges, _ := repo.GitHasStagedChanges()
	commitURL := repo.BuildCommitURL(commit.ID)

	if hasGitChanges || commitURL != "" {
		gitMsg := formatGitCommitMessage(repo, commit.ID, message)
		var gitErr error
		if hasGitChanges {
			gitErr = repo.GitCommit(gitMsg)
		} else {
			// No file changes, but we want to record the tin commit link
			gitErr = repo.GitCommitEmpty(gitMsg)
		}
		if gitErr != nil {
			// Log but don't fail the tin commit
			fmt.Fprintf(os.Stderr, "Warning: failed to commit git changes: %v\n", gitErr)
		} else {
			// Update the commit's git hash to the new one
			newGitHash, _ := repo.GetCurrentGitHash()
			if newGitHash != "" {
				commit.GitCommitHash = newGitHash
				repo.SaveCommit(commit)
			}
		}
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

Usage: tin commit [-m <message>]

Options:
  -m, --message <msg>  Commit message (optional, auto-generated if not provided)

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

// commitThreadChanges creates a git commit for a thread's file changes
func commitThreadChanges(repo *storage.Repository, thread *model.Thread, messageCount int) error {
	// Get all changed files from git status (respects .gitignore, excludes .tin/)
	files, err := repo.GitGetChangedFiles()
	if err == nil && len(files) > 0 {
		// Stage the files
		if err := repo.GitAdd(files); err != nil {
			return err
		}

		// Check if there are actually staged changes
		hasChanges, err := repo.GitHasStagedChanges()
		if err != nil {
			return err
		}

		if hasChanges {
			// Create git commit with thread info
			commitMsg := formatThreadGitMessage(thread)
			if err := repo.GitCommit(commitMsg); err != nil {
				return err
			}
		}
	}

	// Store the current git hash
	gitHash, _ := repo.GetCurrentGitHash()
	thread.GitCommitHash = gitHash

	return repo.SaveThread(thread)
}

// formatThreadGitMessage creates a git commit message for a thread
func formatThreadGitMessage(thread *model.Thread) string {
	shortID := thread.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	message := "thread changes"
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

// formatGitCommitMessage creates a git commit message with tin commit link
func formatGitCommitMessage(repo *storage.Repository, tinCommitID string, message string) string {
	shortID := tinCommitID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Clean up message - normalize whitespace
	message = strings.TrimSpace(message)

	// Build the commit message:
	// Subject: [tin <id>] <message or first line>
	// Body: full message if multi-line, plus tin commit URL
	var builder strings.Builder

	// Subject line
	firstLine := message
	restOfMessage := ""
	if idx := strings.Index(message, "\n"); idx != -1 {
		firstLine = strings.TrimSpace(message[:idx])
		restOfMessage = strings.TrimSpace(message[idx+1:])
	}

	builder.WriteString(fmt.Sprintf("[tin %s] %s", shortID, firstLine))

	// Body section
	builder.WriteString("\n")

	// Include rest of message if there was more
	if restOfMessage != "" {
		builder.WriteString("\n")
		builder.WriteString(restOfMessage)
	}

	// Always add tin commit URL if available
	if commitURL := repo.BuildCommitURL(tinCommitID); commitURL != "" {
		builder.WriteString("\n\n")
		builder.WriteString(commitURL)
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
				if len(msg) > 72 {
					msg = msg[:69] + "..."
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
			if len(preview) > 30 {
				preview = preview[:27] + "..."
			}
			previews = append(previews, preview)
		}
	}

	if len(previews) == 0 {
		return fmt.Sprintf("%d threads", len(staged))
	}

	// Join previews, but keep total length reasonable
	result := strings.Join(previews, "; ")
	if len(result) > 72 {
		result = result[:69] + "..."
	}
	return result
}
