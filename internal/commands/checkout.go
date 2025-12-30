package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

func Checkout(args []string) error {
	var createBranch bool

	// Parse flags
	var target string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printCheckoutHelp()
			return nil
		case "-b":
			createBranch = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				target = args[i]
			}
		}
	}

	if target == "" {
		return fmt.Errorf("branch or commit required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	// Check for active threads
	active, err := repo.GetActiveThread()
	if err != nil {
		return err
	}

	if active != nil {
		fmt.Printf("Warning: Active thread %s will be preserved but may have stale context.\n", active.ID[:8])
		fmt.Println("Consider completing or committing it before switching.")
		fmt.Println()
	}

	// Check for staged threads
	staged, err := repo.GetStagedThreads()
	if err != nil {
		return err
	}

	if len(staged) > 0 {
		fmt.Printf("Warning: %d staged thread(s) will remain staged.\n", len(staged))
		fmt.Println()
	}

	// Create branch if -b flag
	if createBranch {
		if repo.BranchExists(target) {
			return fmt.Errorf("branch '%s' already exists", target)
		}

		// Get current commit
		currentBranch, err := repo.ReadHead()
		if err != nil {
			return err
		}

		currentCommit, err := repo.GetBranchCommit(currentBranch)
		if err != nil && err != storage.ErrNotFound {
			return err
		}

		var commitID string
		if currentCommit != nil {
			commitID = currentCommit.ID
		}

		// Create and switch to new branch
		if err := repo.WriteBranch(target, commitID); err != nil {
			return err
		}

		if err := repo.WriteHead(target); err != nil {
			return err
		}

		fmt.Printf("Switched to a new branch '%s'\n", target)
		return nil
	}

	// Check if target is a branch
	if repo.BranchExists(target) {
		return checkoutBranch(repo, target)
	}

	// Check if target is a commit
	commit, err := repo.LoadCommit(target)
	if err == nil {
		return checkoutCommit(repo, commit)
	}

	// Try prefix match for commit
	commits, err := repo.ListCommits()
	if err != nil {
		return err
	}

	for _, c := range commits {
		if strings.HasPrefix(c.ID, target) {
			return checkoutCommit(repo, c)
		}
	}

	return fmt.Errorf("branch or commit not found: %s", target)
}

func checkoutBranch(repo *storage.Repository, name string) error {
	// Get the commit the branch points to
	commit, err := repo.GetBranchCommit(name)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	// Update HEAD
	if err := repo.WriteHead(name); err != nil {
		return err
	}

	// Checkout git state if commit exists
	if commit != nil && commit.GitCommitHash != "" {
		if err := repo.GitCheckout(commit.GitCommitHash); err != nil {
			fmt.Printf("Warning: Failed to checkout git state: %s\n", err)
		}
	}

	if commit != nil {
		fmt.Printf("Switched to branch '%s' at %s\n", name, commit.ShortID())
	} else {
		fmt.Printf("Switched to branch '%s' (no commits)\n", name)
	}

	return nil
}

func checkoutCommit(repo *storage.Repository, commit *model.TinCommit) error {
	// Detached HEAD state - we'll just restore git state
	// In a full implementation, we'd track detached HEAD properly

	if commit.GitCommitHash != "" {
		if err := repo.GitCheckout(commit.GitCommitHash); err != nil {
			return fmt.Errorf("failed to checkout git state: %w", err)
		}
	}

	fmt.Printf("Checked out commit %s\n", commit.ShortID())
	fmt.Printf("  %s\n", commit.Message)
	fmt.Println()
	fmt.Println("You are in 'detached HEAD' state. To create a branch here:")
	fmt.Printf("  tin checkout -b <new-branch-name>\n")

	return nil
}

func printCheckoutHelp() {
	fmt.Println(`Switch branches or restore working tree

Usage: tin checkout [options] <branch|commit>

Options:
  -b              Create a new branch and switch to it

This command switches to the specified branch or commit, updating the
working tree to match. When checking out a commit directly, you enter
'detached HEAD' state.

The git working tree is also updated to match the commit's recorded
git state.

Examples:
  tin checkout main           Switch to the main branch
  tin checkout feature-auth   Switch to the feature-auth branch
  tin checkout -b new-feature Create and switch to new-feature
  tin checkout abc123         Checkout a specific commit`)
}
