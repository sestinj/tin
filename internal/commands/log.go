package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/dadlerj/tin/internal/storage"
)

func Log(args []string) error {
	limit := 10 // Default limit

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printLogHelp()
			return nil
		case "-n":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err == nil {
					limit = n
				}
				i++
			}
		case "--all":
			limit = 0
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

	// Get current branch and its commit
	branch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	headCommit, err := repo.GetBranchCommit(branch)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	if headCommit == nil {
		fmt.Println("No commits yet")
		return nil
	}

	// Get commit history
	history, err := repo.GetCommitHistory(headCommit.ID, limit)
	if err != nil {
		return err
	}

	for _, commit := range history {
		// Commit header
		fmt.Printf("\033[33mcommit %s\033[0m", commit.ID)
		if commit.ID == headCommit.ID {
			fmt.Printf(" \033[36m(HEAD -> %s)\033[0m", branch)
		}
		fmt.Println()

		fmt.Printf("Date:   %s\n", commit.Timestamp.Format("Mon Jan 2 15:04:05 2006 -0700"))
		fmt.Printf("Git:    %s\n", commit.GitCommitHash[:min(8, len(commit.GitCommitHash))])
		fmt.Println()
		fmt.Printf("    %s\n", commit.Message)
		fmt.Println()

		// Thread summaries
		if len(commit.Threads) > 0 {
			fmt.Printf("    Threads (%d):\n", len(commit.Threads))
			for _, ref := range commit.Threads {
				thread, err := repo.LoadThread(ref.ThreadID)
				if err != nil {
					fmt.Printf("      - %s (%d messages)\n", ref.ThreadID[:8], ref.MessageCount)
					continue
				}

				preview := ""
				if first := thread.FirstHumanMessage(); first != nil {
					preview = truncate(extractPreview(first.Content), 60)
				}
				fmt.Printf("      - %s (%d messages): %s\n", ref.ThreadID[:8], ref.MessageCount, preview)
			}
			fmt.Println()
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printLogHelp() {
	fmt.Println(`Show commit history

Usage: tin log [options]

Options:
  -n <number>  Limit to the last n commits (default: 10)
  --all        Show all commits (no limit)

The log shows each commit with:
  - Commit hash
  - Date and git commit reference
  - Commit message
  - Thread summaries (thread ID, message count, first human message preview)

Examples:
  tin log           Show last 10 commits
  tin log -n 5      Show last 5 commits
  tin log --all     Show all commits`)
}
