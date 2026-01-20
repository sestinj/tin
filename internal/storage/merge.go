package storage

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sestinj/tin/internal/model"
)

const (
	MergeHeadFile = "MERGE_HEAD"
)

// MergeState tracks an in-progress merge operation
type MergeState struct {
	SourceBranch     string            `json:"source_branch"`
	TargetBranch     string            `json:"target_branch"`
	SourceCommitID   string            `json:"source_commit_id"`
	TargetCommitID   string            `json:"target_commit_id"`
	GitMergeComplete bool              `json:"git_merge_complete"`
	CollectedThreads []model.ThreadRef `json:"collected_threads"`
	RenamedThreads   []RenamedThread   `json:"renamed_threads,omitempty"`
}

// RenamedThread tracks a thread that was renamed during merge conflict resolution
type RenamedThread struct {
	OriginalThreadID string `json:"original_thread_id"`
	NewThreadID      string `json:"new_thread_id"`
	SourceBranch     string `json:"source_branch"`
}

// WriteMergeState saves the merge state to MERGE_HEAD
func (r *Repository) WriteMergeState(state *MergeState) error {
	path := filepath.Join(r.TinPath, MergeHeadFile)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadMergeState reads the merge state from MERGE_HEAD
func (r *Repository) ReadMergeState() (*MergeState, error) {
	path := filepath.Join(r.TinPath, MergeHeadFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var state MergeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ClearMergeState removes the MERGE_HEAD file
func (r *Repository) ClearMergeState() error {
	path := filepath.Join(r.TinPath, MergeHeadFile)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsMergeInProgress returns true if there is an in-progress merge
func (r *Repository) IsMergeInProgress() bool {
	path := filepath.Join(r.TinPath, MergeHeadFile)
	_, err := os.Stat(path)
	return err == nil
}

// GitMerge starts a git merge with --no-commit flag
// Returns (hasConflicts, error)
func (r *Repository) GitMerge(branch string) (bool, error) {
	cmd := exec.Command("git", "merge", "--no-commit", branch)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		// Check if it's a conflict (exit code 1 with conflict markers)
		if strings.Contains(outputStr, "CONFLICT") ||
			strings.Contains(outputStr, "Automatic merge failed") {
			return true, nil
		}
		// Other error
		return false, err
	}
	return false, nil
}

// GitMergeAbort aborts an in-progress git merge
func (r *Repository) GitMergeAbort() error {
	cmd := exec.Command("git", "merge", "--abort")
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitError{Operation: "merge --abort", Output: string(output)}
	}
	return nil
}

// GitHasMergeConflicts checks if there are unresolved merge conflicts
func (r *Repository) GitHasMergeConflicts() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Look for unmerged files (status starts with U or has UU, AA, DD, etc.)
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 2 {
			continue
		}
		status := line[:2]
		// Unmerged statuses: UU (both modified), AA (both added), DD (both deleted),
		// UA, AU, UD, DU
		if status == "UU" || status == "AA" || status == "DD" ||
			status == "UA" || status == "AU" || status == "UD" || status == "DU" {
			return true
		}
	}
	return false
}

// GitIsInMergeState checks if git is in a merge state
func (r *Repository) GitIsInMergeState() bool {
	mergePath := filepath.Join(r.RootPath, ".git", "MERGE_HEAD")
	_, err := os.Stat(mergePath)
	return err == nil
}

// GitMergeFastForward performs a fast-forward merge
func (r *Repository) GitMergeFastForward(branch string) error {
	cmd := exec.Command("git", "merge", "--ff-only", branch)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitError{Operation: "merge --ff-only", Output: string(output)}
	}
	return nil
}

// GitCommitMerge commits an in-progress merge
func (r *Repository) GitCommitMerge(message string) error {
	// First stage all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = r.RootPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return &GitError{Operation: "add -A", Output: string(output)}
	}

	// Then commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitError{Operation: "commit", Output: string(output)}
	}
	return nil
}

// GitError provides detailed git operation errors
type GitError struct {
	Operation string
	Output    string
}

func (e *GitError) Error() string {
	return "git " + e.Operation + " failed: " + e.Output
}

// CollectThreadsFromHistory walks the commit history and collects all threads
// Returns a map of threadID -> ThreadRef (most recent version for each thread)
func (r *Repository) CollectThreadsFromHistory(startCommitID string) (map[string]model.ThreadRef, error) {
	threads := make(map[string]model.ThreadRef)

	if startCommitID == "" {
		return threads, nil
	}

	currentID := startCommitID
	for currentID != "" {
		commit, err := r.LoadCommit(currentID)
		if err != nil {
			break
		}

		// Add threads from this commit (first occurrence wins - most recent version)
		for _, ref := range commit.Threads {
			if _, exists := threads[ref.ThreadID]; !exists {
				threads[ref.ThreadID] = ref
			}
		}

		currentID = commit.ParentCommitID
	}

	return threads, nil
}

// IsAncestor checks if ancestorID is an ancestor of descendantID
func (r *Repository) IsAncestor(ancestorID, descendantID string) (bool, error) {
	if ancestorID == "" || descendantID == "" {
		return false, nil
	}

	currentID := descendantID
	for currentID != "" {
		if currentID == ancestorID {
			return true, nil
		}

		commit, err := r.LoadCommit(currentID)
		if err != nil {
			return false, nil
		}
		currentID = commit.ParentCommitID
	}

	return false, nil
}

// GitHasUncommittedChanges checks if there are uncommitted changes in the working tree
func (r *Repository) GitHasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}
