package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/sestinj/tin/internal/model"
	"github.com/sestinj/tin/internal/storage"
)

func Merge(args []string) error {
	var continueFlag, abortFlag bool
	var sourceBranch string

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printMergeHelp()
			return nil
		case "--continue":
			continueFlag = true
		case "--abort":
			abortFlag = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				sourceBranch = args[i]
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

	// Dispatch to appropriate handler
	if abortFlag {
		return mergeAbort(repo)
	}
	if continueFlag {
		return mergeContinue(repo)
	}

	if sourceBranch == "" {
		return fmt.Errorf("branch name required\n\nUsage: tin merge <branch>")
	}

	return mergeStart(repo, sourceBranch)
}

func mergeStart(repo *storage.Repository, sourceBranch string) error {
	// Check for in-progress merge
	if repo.IsMergeInProgress() {
		return fmt.Errorf("merge already in progress\n\nUse 'tin merge --continue' after resolving conflicts, or 'tin merge --abort' to cancel")
	}

	// Get current branch
	targetBranch, err := repo.ReadHead()
	if err != nil {
		return err
	}

	// Check not merging into self
	if sourceBranch == targetBranch {
		return fmt.Errorf("cannot merge branch '%s' into itself", sourceBranch)
	}

	// Check source branch exists
	if !repo.BranchExists(sourceBranch) {
		return fmt.Errorf("branch '%s' not found", sourceBranch)
	}

	// Check for uncommitted changes
	hasChanges, err := repo.GitHasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("you have uncommitted changes\n\nPlease commit or stash them before merging")
	}

	// Check for active threads
	active, err := repo.GetActiveThread()
	if err != nil {
		return err
	}
	if active != nil {
		fmt.Printf("Warning: Active thread %s will be preserved.\n", active.ID[:8])
		fmt.Println()
	}

	// Check for staged threads
	staged, err := repo.GetStagedThreads()
	if err != nil {
		return err
	}
	if len(staged) > 0 {
		fmt.Printf("Warning: %d staged thread(s) - consider committing before merge.\n", len(staged))
		fmt.Println()
	}

	// Get commit IDs for both branches
	targetCommit, err := repo.GetBranchCommit(targetBranch)
	if err != nil && err != storage.ErrNotFound {
		return err
	}
	sourceCommit, err := repo.GetBranchCommit(sourceBranch)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	var targetCommitID, sourceCommitID string
	if targetCommit != nil {
		targetCommitID = targetCommit.ID
	}
	if sourceCommit != nil {
		sourceCommitID = sourceCommit.ID
	}

	// Check if already up to date
	if sourceCommitID == targetCommitID {
		fmt.Println("Already up to date.")
		return nil
	}

	// Check if source is ancestor of target (nothing to merge)
	if sourceCommitID != "" && targetCommitID != "" {
		isAncestor, _ := repo.IsAncestor(sourceCommitID, targetCommitID)
		if isAncestor {
			fmt.Println("Already up to date.")
			return nil
		}
	}

	// Check if we can fast-forward
	if targetCommitID == "" || (sourceCommitID != "" && canFastForward(repo, targetCommitID, sourceCommitID)) {
		return fastForwardMerge(repo, targetBranch, sourceBranch, sourceCommitID)
	}

	// Full merge needed
	return fullMerge(repo, targetBranch, sourceBranch, targetCommitID, sourceCommitID)
}

func canFastForward(repo *storage.Repository, targetCommitID, sourceCommitID string) bool {
	// Can fast-forward if target is an ancestor of source
	isAncestor, _ := repo.IsAncestor(targetCommitID, sourceCommitID)
	return isAncestor
}

func fastForwardMerge(repo *storage.Repository, targetBranch, sourceBranch, sourceCommitID string) error {
	fmt.Printf("Fast-forward merge: %s -> %s\n", sourceBranch, targetBranch)

	// Do git fast-forward merge
	if err := repo.GitMergeFastForward(sourceBranch); err != nil {
		return fmt.Errorf("git fast-forward failed: %w", err)
	}

	// Update tin branch pointer
	if err := repo.WriteBranch(targetBranch, sourceCommitID); err != nil {
		return err
	}

	fmt.Printf("Merged '%s' into '%s' (fast-forward)\n", sourceBranch, targetBranch)
	return nil
}

func fullMerge(repo *storage.Repository, targetBranch, sourceBranch, targetCommitID, sourceCommitID string) error {
	// Start git merge
	hasConflicts, err := repo.GitMerge(sourceBranch)
	if err != nil {
		return fmt.Errorf("git merge failed: %w", err)
	}

	// Collect threads from both branches
	mergedThreads, renamedThreads, err := mergeThreads(repo, targetCommitID, sourceCommitID, sourceBranch)
	if err != nil {
		// Abort git merge if thread merging fails
		repo.GitMergeAbort()
		return fmt.Errorf("failed to merge threads: %w", err)
	}

	// Save merge state
	mergeState := &storage.MergeState{
		SourceBranch:     sourceBranch,
		TargetBranch:     targetBranch,
		SourceCommitID:   sourceCommitID,
		TargetCommitID:   targetCommitID,
		GitMergeComplete: !hasConflicts,
		CollectedThreads: mergedThreads,
		RenamedThreads:   renamedThreads,
	}

	if err := repo.WriteMergeState(mergeState); err != nil {
		repo.GitMergeAbort()
		return fmt.Errorf("failed to save merge state: %w", err)
	}

	if hasConflicts {
		fmt.Println("Automatic merge failed; fix conflicts and then run 'tin merge --continue'")
		fmt.Println("Or run 'tin merge --abort' to cancel the merge.")
		return nil
	}

	// No conflicts - complete the merge
	return completeMerge(repo, mergeState)
}

func mergeThreads(repo *storage.Repository, targetCommitID, sourceCommitID, sourceBranch string) ([]model.ThreadRef, []storage.RenamedThread, error) {
	// Collect threads from target branch
	targetThreads, err := repo.CollectThreadsFromHistory(targetCommitID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to collect target threads: %w", err)
	}

	// Collect threads from source branch
	sourceThreads, err := repo.CollectThreadsFromHistory(sourceCommitID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to collect source threads: %w", err)
	}

	// Merge with conflict detection
	merged := make(map[string]model.ThreadRef)
	var renamedThreads []storage.RenamedThread

	// Add all target threads
	for id, ref := range targetThreads {
		merged[id] = ref
	}

	// Add source threads, handling conflicts
	for id, sourceRef := range sourceThreads {
		targetRef, exists := targetThreads[id]
		if !exists {
			// Thread only on source - add it
			merged[id] = sourceRef
		} else if sourceRef.ContentHash != targetRef.ContentHash {
			// Conflict - same thread, different content
			// Create renamed copy of source thread
			newID, err := copyThreadWithRename(repo, id, sourceRef.ContentHash, sourceBranch)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to copy conflicting thread: %w", err)
			}

			merged[newID] = model.ThreadRef{
				ThreadID:     newID,
				MessageCount: sourceRef.MessageCount,
				ContentHash:  sourceRef.ContentHash,
			}

			renamedThreads = append(renamedThreads, storage.RenamedThread{
				OriginalThreadID: id,
				NewThreadID:      newID,
				SourceBranch:     sourceBranch,
			})

			fmt.Printf("Thread conflict: %s exists on both branches\n", id[:8])
			fmt.Printf("  Keeping target version as %s\n", id[:8])
			fmt.Printf("  Copied source version as %s_from_%s\n", id[:8], sourceBranch)
		}
		// If content is identical, no action needed (target version already in merged)
	}

	// Convert map to slice
	var result []model.ThreadRef
	for _, ref := range merged {
		result = append(result, ref)
	}

	return result, renamedThreads, nil
}

func copyThreadWithRename(repo *storage.Repository, threadID, contentHash, sourceBranch string) (string, error) {
	// Load the thread version
	thread, err := repo.LoadThreadVersion(threadID, contentHash)
	if err != nil {
		// Try loading from latest if versioned not available
		thread, err = repo.LoadThread(threadID)
		if err != nil {
			return "", fmt.Errorf("could not load thread %s: %w", threadID, err)
		}
	}

	// Generate new ID: hash of original ID + source branch
	h := sha256.New()
	h.Write([]byte(threadID))
	h.Write([]byte("_from_"))
	h.Write([]byte(sourceBranch))
	newID := hex.EncodeToString(h.Sum(nil))

	// Update thread ID and save
	thread.ID = newID
	if err := repo.SaveThread(thread); err != nil {
		return "", fmt.Errorf("failed to save renamed thread: %w", err)
	}

	return newID, nil
}

func completeMerge(repo *storage.Repository, state *storage.MergeState) error {
	// Commit git changes
	mergeMsg := fmt.Sprintf("Merge branch '%s' into %s", state.SourceBranch, state.TargetBranch)
	if err := repo.GitCommitMerge(mergeMsg); err != nil {
		return fmt.Errorf("failed to commit merge: %w", err)
	}

	// Get new git hash
	gitHash, err := repo.GetCurrentGitHash()
	if err != nil {
		return fmt.Errorf("failed to get git hash: %w", err)
	}

	// Create tin merge commit
	commit := model.NewMergeCommit(
		mergeMsg,
		state.CollectedThreads,
		gitHash,
		state.TargetCommitID,
		state.SourceCommitID,
	)
	commit.Author = repo.GitGetAuthor()

	if err := repo.SaveCommit(commit); err != nil {
		return fmt.Errorf("failed to save merge commit: %w", err)
	}

	// Update branch pointer
	if err := repo.WriteBranch(state.TargetBranch, commit.ID); err != nil {
		return err
	}

	// Clear merge state
	if err := repo.ClearMergeState(); err != nil {
		return err
	}

	fmt.Printf("Merged '%s' into '%s'\n", state.SourceBranch, state.TargetBranch)
	fmt.Printf("  Commit: %s\n", commit.ShortID())
	fmt.Printf("  Threads: %d\n", len(state.CollectedThreads))
	if len(state.RenamedThreads) > 0 {
		fmt.Printf("  Thread conflicts resolved: %d (kept both versions)\n", len(state.RenamedThreads))
	}

	return nil
}

func mergeContinue(repo *storage.Repository) error {
	// Check merge state exists
	state, err := repo.ReadMergeState()
	if err == storage.ErrNotFound {
		return fmt.Errorf("no merge in progress")
	}
	if err != nil {
		return fmt.Errorf("failed to read merge state: %w", err)
	}

	// Check for remaining git conflicts
	if repo.GitHasMergeConflicts() {
		return fmt.Errorf("you still have unresolved conflicts\n\nResolve them and then run 'tin merge --continue'")
	}

	// Complete the merge
	return completeMerge(repo, state)
}

func mergeAbort(repo *storage.Repository) error {
	// Check merge state exists
	state, err := repo.ReadMergeState()
	if err == storage.ErrNotFound {
		return fmt.Errorf("no merge in progress")
	}
	if err != nil {
		return fmt.Errorf("failed to read merge state: %w", err)
	}

	// Abort git merge if still in progress
	if repo.GitIsInMergeState() {
		if err := repo.GitMergeAbort(); err != nil {
			// Non-fatal - continue with cleanup
			fmt.Printf("Warning: git merge --abort failed: %v\n", err)
		}
	}

	// Delete any renamed threads created during merge attempt
	for _, renamed := range state.RenamedThreads {
		if err := repo.DeleteThread(renamed.NewThreadID); err != nil {
			// Non-fatal - continue with cleanup
			fmt.Printf("Warning: failed to delete renamed thread %s: %v\n", renamed.NewThreadID[:8], err)
		}
	}

	// Clear merge state
	if err := repo.ClearMergeState(); err != nil {
		return err
	}

	fmt.Println("Merge aborted.")
	return nil
}

func printMergeHelp() {
	fmt.Println(`Merge a branch into the current branch

Usage: tin merge <branch>
       tin merge --continue
       tin merge --abort

Options:
  --continue    Complete merge after resolving git conflicts
  --abort       Cancel an in-progress merge
  -h, --help    Show this help message

This command merges the specified branch into the current branch, combining
both git history and thread history.

Thread conflict handling:
  If the same thread exists on both branches with different content,
  both versions are kept. The source branch's version is renamed with
  a suffix (e.g., "thread-id_from_feature-branch").

Git conflicts:
  If git encounters merge conflicts, the merge will pause. Resolve the
  conflicts in your editor, then run 'tin merge --continue' to complete
  the merge, or 'tin merge --abort' to cancel.

Examples:
  tin merge feature-auth      Merge feature-auth into current branch
  tin merge --continue        Complete paused merge after conflict resolution
  tin merge --abort           Cancel in-progress merge`)
}
