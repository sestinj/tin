package commands

import (
	"fmt"
	"os"

	"github.com/dadlerj/tin/internal/model"
	"github.com/dadlerj/tin/internal/storage"
)

func Status(args []string) error {
	// Parse flags
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printStatusHelp()
			return nil
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

	// Check for merge in progress
	if repo.IsMergeInProgress() {
		mergeState, err := repo.ReadMergeState()
		if err == nil {
			fmt.Println("\033[33mMerge in progress:\033[0m")
			fmt.Printf("  Merging '%s' into '%s'\n", mergeState.SourceBranch, mergeState.TargetBranch)
			if repo.GitHasMergeConflicts() {
				fmt.Println("  \033[31mConflicts detected - resolve and run 'tin merge --continue'\033[0m")
			} else {
				fmt.Println("  No conflicts - run 'tin merge --continue' to complete")
			}
			fmt.Println("  Or run 'tin merge --abort' to cancel")
			fmt.Println()
		}
	}

	// Check for branch mismatch and warn prominently
	state, err := repo.GetBranchState()
	if err == nil && !state.InSync && state.GitBranch != "" {
		fmt.Println("\033[33mWARNING: tin/git branch mismatch!\033[0m")
		fmt.Printf("  tin: %s\n", state.TinBranch)
		fmt.Printf("  git: %s\n", state.GitBranch)
		fmt.Println("  Run 'tin sync' to align, or 'tin sync --dry-run' to preview.")
		fmt.Println()
	}

	// Get current branch
	branch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	fmt.Printf("On branch %s\n", branch)

	// Get current commit
	commit, err := repo.GetHeadCommit()
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	if commit == nil {
		fmt.Println("\nNo commits yet")
	} else {
		fmt.Printf("Latest commit: %s %s\n", commit.ShortID(), truncate(commit.Message, 50))
	}

	// Get staged threads
	staged, err := repo.GetStagedThreads()
	if err != nil {
		return err
	}

	if len(staged) > 0 {
		fmt.Println("\nThreads staged for commit:")
		for _, ref := range staged {
			thread, err := repo.LoadThread(ref.ThreadID)
			if err != nil {
				fmt.Printf("  %s (unable to load)\n", ref.ThreadID[:8])
				continue
			}
			preview := ""
			if first := thread.FirstHumanMessage(); first != nil {
				preview = truncate(first.Content, 50)
			}
			fmt.Printf("  %s (%d messages) %s\n", ref.ThreadID[:8], ref.MessageCount, preview)
		}
	}

	// Get unstaged threads
	unstaged, err := repo.GetUnstagedThreads()
	if err != nil {
		return err
	}

	if len(unstaged) > 0 {
		fmt.Println("\nUnstaged threads:")
		for _, thread := range unstaged {
			status := ""
			if thread.Status == model.ThreadStatusActive {
				status = " (active)"
			}
			preview := ""
			if first := thread.FirstHumanMessage(); first != nil {
				preview = truncate(first.Content, 50)
			}
			fmt.Printf("  %s (%d messages)%s %s\n", thread.ID[:8], len(thread.Messages), status, preview)
		}
		fmt.Println("\nUse \"tin add <thread-id>\" to stage threads for commit")
	}

	// Get active thread
	active, err := repo.GetActiveThread()
	if err != nil {
		return err
	}

	if active != nil {
		fmt.Printf("\nActive thread: %s (%d messages)\n", active.ID[:8], len(active.Messages))
	}

	if len(staged) == 0 && len(unstaged) == 0 && active == nil {
		fmt.Println("\nNo threads. Start a conversation with your AI agent to create threads.")
	}

	return nil
}

func printStatusHelp() {
	fmt.Println(`Show the working tree status

Usage: tin status

Displays the current state of the tin repository including:
  - Current branch and latest commit
  - Threads staged for commit (ready to be committed)
  - Unstaged threads (need to be added with 'tin add')
  - Active threads (conversations in progress)

Thread states:
  staged     - Added to index, ready to commit
  unstaged   - Exists but not yet added
  active     - Currently being worked on (conversation in progress)
  committed  - Already part of a commit

Examples:
  tin status    Show repository status`)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
