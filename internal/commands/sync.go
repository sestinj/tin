package commands

import (
	"fmt"
	"os"

	"github.com/sestinj/tin/internal/storage"
)

func Sync(args []string) error {
	var dryRun bool
	var tinFollowsGit bool

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printSyncHelp()
			return nil
		case "-n", "--dry-run":
			dryRun = true
		case "--tin-follows-git":
			tinFollowsGit = true
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

	state, err := repo.GetBranchState()
	if err != nil {
		return err
	}

	// Show current state
	fmt.Printf("Tin branch:  %s\n", state.TinBranch)
	if state.GitBranch == "" {
		fmt.Println("Git branch:  (detached HEAD or no commits)")
	} else {
		fmt.Printf("Git branch:  %s\n", state.GitBranch)
	}

	if state.InSync {
		fmt.Println("\nAlready in sync!")
		return nil
	}

	fmt.Println()

	if tinFollowsGit {
		if state.GitBranch == "" {
			return fmt.Errorf("cannot sync: git is in detached HEAD state or has no commits")
		}

		// Update tin HEAD to match git
		if dryRun {
			fmt.Printf("Would update tin HEAD from '%s' to '%s'\n",
				state.TinBranch, state.GitBranch)
			return nil
		}

		// Check if tin branch exists for the git branch
		if !repo.BranchExists(state.GitBranch) {
			// Create the tin branch if it doesn't exist
			fmt.Printf("Creating tin branch '%s'...\n", state.GitBranch)
			if err := repo.WriteBranch(state.GitBranch, ""); err != nil {
				return fmt.Errorf("failed to create tin branch: %w", err)
			}
		}

		if err := repo.WriteHead(state.GitBranch); err != nil {
			return fmt.Errorf("failed to update tin HEAD: %w", err)
		}
		fmt.Printf("Updated tin HEAD to '%s'\n", state.GitBranch)
	} else {
		// Default: git follows tin
		if dryRun {
			fmt.Printf("Would run: git checkout %s\n", state.TinBranch)
			return nil
		}

		// Check if git branch exists
		if !repo.GitBranchExists(state.TinBranch) {
			// Create the git branch
			fmt.Printf("Creating git branch '%s'...\n", state.TinBranch)
			if err := repo.GitCreateBranch(state.TinBranch); err != nil {
				return fmt.Errorf("failed to create git branch: %w", err)
			}
		}

		if err := repo.GitCheckoutBranch(state.TinBranch); err != nil {
			return fmt.Errorf("failed to switch git branch: %w", err)
		}
		fmt.Printf("Switched git to '%s'\n", state.TinBranch)
	}

	fmt.Println("\nNow in sync!")
	return nil
}

func printSyncHelp() {
	fmt.Println(`Synchronize tin and git branch state

Usage: tin sync [options]

Options:
  -n, --dry-run        Show what would happen without making changes
  --tin-follows-git    Update tin HEAD to match git (default: git follows tin)
  -h, --help           Show this help message

By default, git will be switched to match tin's current branch.
Use --tin-follows-git to instead update tin's HEAD to match git.

Examples:
  tin sync                    Switch git branch to match tin
  tin sync --dry-run          Show what sync would do
  tin sync --tin-follows-git  Switch tin branch to match git`)
}
