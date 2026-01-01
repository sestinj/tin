package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/dadlerj/tin/internal/storage"
)

func Branch(args []string) error {
	var deleteBranch string
	var listAll bool

	// Parse flags
	var branchName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printBranchHelp()
			return nil
		case "-d", "--delete":
			if i+1 < len(args) {
				deleteBranch = args[i+1]
				i++
			}
		case "-a", "--all":
			listAll = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				branchName = args[i]
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

	// Delete branch
	if deleteBranch != "" {
		return deleteBranchCmd(repo, deleteBranch)
	}

	// Create branch
	if branchName != "" {
		return createBranch(repo, branchName)
	}

	// List branches (default)
	return listBranches(repo, listAll)
}

func listBranches(repo *storage.Repository, showAll bool) error {
	branches, err := repo.ListBranches()
	if err != nil {
		return err
	}

	if len(branches) == 0 {
		fmt.Println("No branches yet")
		return nil
	}

	currentBranch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	for _, branch := range branches {
		if branch == currentBranch {
			fmt.Printf("* \033[32m%s\033[0m\n", branch)
		} else {
			fmt.Printf("  %s\n", branch)
		}

		if showAll {
			commit, err := repo.GetBranchCommit(branch)
			if err == nil && commit != nil {
				fmt.Printf("    %s %s\n", commit.ShortID(), truncate(commit.Message, 40))
			}
		}
	}

	return nil
}

func createBranch(repo *storage.Repository, name string) error {
	// Check if branch already exists
	if repo.BranchExists(name) {
		return fmt.Errorf("branch '%s' already exists", name)
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

	// Create tin branch
	if err := repo.WriteBranch(name, commitID); err != nil {
		return err
	}

	// Also create git branch
	if err := repo.GitCreateBranch(name); err != nil {
		fmt.Printf("Warning: Failed to create git branch: %s\n", err)
	}

	if commitID != "" {
		fmt.Printf("Created branch '%s' at %s\n", name, commitID[:8])
	} else {
		fmt.Printf("Created branch '%s' (no commits yet)\n", name)
	}

	return nil
}

func deleteBranchCmd(repo *storage.Repository, name string) error {
	// Check if branch exists
	if !repo.BranchExists(name) {
		return fmt.Errorf("branch '%s' not found", name)
	}

	// Check if it's the current branch
	currentBranch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	if name == currentBranch {
		return fmt.Errorf("cannot delete the current branch '%s'", name)
	}

	// Don't allow deleting main/master
	if name == "main" || name == "master" {
		return fmt.Errorf("cannot delete branch '%s'", name)
	}

	// Delete tin branch
	if err := repo.DeleteBranch(name); err != nil {
		return err
	}

	// Also delete git branch (if it exists)
	if repo.GitBranchExists(name) {
		if err := repo.GitDeleteBranch(name); err != nil {
			fmt.Printf("Warning: Failed to delete git branch: %s\n", err)
		}
	}

	fmt.Printf("Deleted branch '%s'\n", name)
	return nil
}

func printBranchHelp() {
	fmt.Println(`List, create, or delete branches

Usage: tin branch [options] [<name>]

Options:
  -a, --all          Show branches with their commit info
  -d, --delete <n>   Delete a branch

With no arguments, lists all branches. With a name argument, creates
a new branch at the current commit.

Examples:
  tin branch                List all branches
  tin branch feature-auth   Create a new branch named 'feature-auth'
  tin branch -d old-branch  Delete the branch 'old-branch'`)
}
