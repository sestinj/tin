package storage

import (
	"fmt"
	"os/exec"
	"strings"
)

// BranchState represents the current branch state of tin and git
type BranchState struct {
	TinBranch string
	GitBranch string
	InSync    bool
}

// BranchMismatchError indicates tin and git are on different branches
type BranchMismatchError struct {
	TinBranch string
	GitBranch string
}

func (e *BranchMismatchError) Error() string {
	return fmt.Sprintf("tin branch '%s' does not match git branch '%s'",
		e.TinBranch, e.GitBranch)
}

// GetCurrentGitBranch returns the current git branch name
func (r *Repository) GetCurrentGitBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		// Might be in detached HEAD state or no commits yet
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetBranchState returns the current tin and git branch state
func (r *Repository) GetBranchState() (*BranchState, error) {
	tinBranch, err := r.ReadHead()
	if err != nil {
		return nil, err
	}

	gitBranch, err := r.GetCurrentGitBranch()
	if err != nil {
		// Git might be in detached HEAD or no commits yet
		gitBranch = ""
	}

	return &BranchState{
		TinBranch: tinBranch,
		GitBranch: gitBranch,
		InSync:    tinBranch == gitBranch,
	}, nil
}

// CheckBranchSync returns an error if tin and git branches don't match
func (r *Repository) CheckBranchSync() error {
	state, err := r.GetBranchState()
	if err != nil {
		return err
	}

	if !state.InSync && state.GitBranch != "" {
		return &BranchMismatchError{
			TinBranch: state.TinBranch,
			GitBranch: state.GitBranch,
		}
	}
	return nil
}
