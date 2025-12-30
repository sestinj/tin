package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danieladler/tin/internal/model"
)

// SaveThread saves a thread to the repository
func (r *Repository) SaveThread(thread *model.Thread) error {
	path := filepath.Join(r.TinPath, ThreadsDir, thread.ID+".json")
	data, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadThread loads a thread by ID
func (r *Repository) LoadThread(id string) (*model.Thread, error) {
	path := filepath.Join(r.TinPath, ThreadsDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var thread model.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return nil, err
	}
	return &thread, nil
}

// ListThreads returns all threads in the repository
func (r *Repository) ListThreads() ([]*model.Thread, error) {
	threadsPath := filepath.Join(r.TinPath, ThreadsDir)
	entries, err := os.ReadDir(threadsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Thread{}, nil
		}
		return nil, err
	}

	var threads []*model.Thread
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		thread, err := r.LoadThread(id)
		if err != nil {
			continue // Skip invalid threads
		}
		threads = append(threads, thread)
	}

	// Sort by start time, newest first
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].StartedAt.After(threads[j].StartedAt)
	})

	return threads, nil
}

// GetUnstagedThreads returns threads that are not in the index
func (r *Repository) GetUnstagedThreads() ([]*model.Thread, error) {
	threads, err := r.ListThreads()
	if err != nil {
		return nil, err
	}

	index, err := r.ReadIndex()
	if err != nil {
		return nil, err
	}

	// Build a set of staged thread IDs
	staged := make(map[string]bool)
	for _, ref := range index.Staged {
		staged[ref.ThreadID] = true
	}

	// Filter to unstaged threads that are not fully committed
	// A thread is fully committed only if status=committed AND content unchanged since commit
	var unstaged []*model.Thread
	for _, t := range threads {
		if staged[t.ID] {
			continue // Already staged
		}
		// Thread is fully committed only if status is committed AND content hash matches
		fullyCommitted := t.Status == model.ThreadStatusCommitted && t.ComputeContentHash() == t.CommittedContentHash
		if !fullyCommitted {
			unstaged = append(unstaged, t)
		}
	}

	return unstaged, nil
}

// GetStagedThreads returns threads that are in the index with their refs
func (r *Repository) GetStagedThreads() ([]model.ThreadRef, error) {
	index, err := r.ReadIndex()
	if err != nil {
		return nil, err
	}
	return index.Staged, nil
}

// StageThread adds a thread to the staging area
func (r *Repository) StageThread(threadID string, messageCount int) error {
	index, err := r.ReadIndex()
	if err != nil {
		return err
	}

	// Check if already staged
	for i, ref := range index.Staged {
		if ref.ThreadID == threadID {
			// Update message count
			index.Staged[i].MessageCount = messageCount
			return r.WriteIndex(index)
		}
	}

	// Add new entry
	index.Staged = append(index.Staged, model.ThreadRef{
		ThreadID:     threadID,
		MessageCount: messageCount,
	})

	return r.WriteIndex(index)
}

// UnstageThread removes a thread from the staging area
func (r *Repository) UnstageThread(threadID string) error {
	index, err := r.ReadIndex()
	if err != nil {
		return err
	}

	var newStaged []model.ThreadRef
	for _, ref := range index.Staged {
		if ref.ThreadID != threadID {
			newStaged = append(newStaged, ref)
		}
	}
	index.Staged = newStaged

	return r.WriteIndex(index)
}

// ClearIndex clears all staged threads
func (r *Repository) ClearIndex() error {
	return r.WriteIndex(&Index{Staged: []model.ThreadRef{}})
}

// GetActiveThread returns the currently active thread (if any)
func (r *Repository) GetActiveThread() (*model.Thread, error) {
	threads, err := r.ListThreads()
	if err != nil {
		return nil, err
	}

	for _, t := range threads {
		if t.Status == model.ThreadStatusActive {
			return t, nil
		}
	}

	return nil, nil
}

// DeleteThread deletes a thread from the repository
func (r *Repository) DeleteThread(id string) error {
	path := filepath.Join(r.TinPath, ThreadsDir, id+".json")
	return os.Remove(path)
}

// ThreadIsCommitted checks if a thread is referenced in any commit
func (r *Repository) ThreadIsCommitted(threadID string) (bool, error) {
	commits, err := r.ListCommits()
	if err != nil {
		return false, err
	}

	for _, commit := range commits {
		for _, ref := range commit.Threads {
			if ref.ThreadID == threadID {
				return true, nil
			}
		}
	}

	return false, nil
}

// PruneEmptyThreads deletes all threads that have zero messages
func (r *Repository) PruneEmptyThreads() {
	threads, err := r.ListThreads()
	if err != nil {
		return
	}

	for _, thread := range threads {
		if len(thread.Messages) == 0 {
			r.UnstageThread(thread.ID)
			r.DeleteThread(thread.ID)
		}
	}
}
