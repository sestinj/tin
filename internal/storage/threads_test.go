package storage

import (
	"testing"

	"github.com/dadlerj/tin/internal/model"
)

func TestRepository_SaveAndLoadThread(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a thread
	thread := model.NewThread("claude-code", "session-123", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	thread.AddMessage(model.NewMessage(model.RoleAssistant, "Hi there!", "", nil))

	// Save
	if err := repo.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	// Load
	loaded, err := repo.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	if loaded.ID != thread.ID {
		t.Errorf("expected ID %s, got %s", thread.ID, loaded.ID)
	}
	if loaded.Agent != "claude-code" {
		t.Errorf("expected agent 'claude-code', got %s", loaded.Agent)
	}
	if loaded.AgentSessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", loaded.AgentSessionID)
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded.Messages))
	}
}

func TestRepository_LoadThread_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = repo.LoadThread("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRepository_ListThreads(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Initially empty
	threads, err := repo.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads, got %d", len(threads))
	}

	// Create and save threads
	for i := 0; i < 3; i++ {
		thread := model.NewThread("claude-code", "", "", "")
		thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
		if err := repo.SaveThread(thread); err != nil {
			t.Fatalf("SaveThread failed: %v", err)
		}
	}

	// List should return 3
	threads, err = repo.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 3 {
		t.Errorf("expected 3 threads, got %d", len(threads))
	}
}

func TestRepository_StageThread(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Stage a thread
	if err := repo.StageThread("thread-1", 10, "testhash1"); err != nil {
		t.Fatalf("StageThread failed: %v", err)
	}

	staged, err := repo.GetStagedThreads()
	if err != nil {
		t.Fatalf("GetStagedThreads failed: %v", err)
	}

	if len(staged) != 1 {
		t.Fatalf("expected 1 staged thread, got %d", len(staged))
	}
	if staged[0].ThreadID != "thread-1" {
		t.Errorf("expected thread-1, got %s", staged[0].ThreadID)
	}
	if staged[0].MessageCount != 10 {
		t.Errorf("expected 10 messages, got %d", staged[0].MessageCount)
	}
}

func TestRepository_StageThread_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Stage initially
	if err := repo.StageThread("thread-1", 10, "testhash1"); err != nil {
		t.Fatalf("StageThread failed: %v", err)
	}

	// Stage again with different count (updates)
	if err := repo.StageThread("thread-1", 15, "testhash2"); err != nil {
		t.Fatalf("StageThread update failed: %v", err)
	}

	staged, err := repo.GetStagedThreads()
	if err != nil {
		t.Fatalf("GetStagedThreads failed: %v", err)
	}

	if len(staged) != 1 {
		t.Fatalf("expected 1 staged thread after update, got %d", len(staged))
	}
	if staged[0].MessageCount != 15 {
		t.Errorf("expected 15 messages after update, got %d", staged[0].MessageCount)
	}
}

func TestRepository_UnstageThread(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Stage two threads
	repo.StageThread("thread-1", 10, "")
	repo.StageThread("thread-2", 20, "")

	// Unstage one
	if err := repo.UnstageThread("thread-1"); err != nil {
		t.Fatalf("UnstageThread failed: %v", err)
	}

	staged, err := repo.GetStagedThreads()
	if err != nil {
		t.Fatalf("GetStagedThreads failed: %v", err)
	}

	if len(staged) != 1 {
		t.Fatalf("expected 1 staged thread after unstage, got %d", len(staged))
	}
	if staged[0].ThreadID != "thread-2" {
		t.Errorf("expected thread-2 remaining, got %s", staged[0].ThreadID)
	}
}

func TestRepository_ClearIndex(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Stage some threads
	repo.StageThread("thread-1", 10, "")
	repo.StageThread("thread-2", 20, "")

	// Clear
	if err := repo.ClearIndex(); err != nil {
		t.Fatalf("ClearIndex failed: %v", err)
	}

	staged, err := repo.GetStagedThreads()
	if err != nil {
		t.Fatalf("GetStagedThreads failed: %v", err)
	}

	if len(staged) != 0 {
		t.Errorf("expected 0 staged threads after clear, got %d", len(staged))
	}
}

func TestRepository_GetUnstagedThreads(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create threads
	thread1 := model.NewThread("claude-code", "", "", "")
	thread1.AddMessage(model.NewMessage(model.RoleHuman, "Hello 1", "", nil))
	repo.SaveThread(thread1)

	thread2 := model.NewThread("claude-code", "", "", "")
	thread2.AddMessage(model.NewMessage(model.RoleHuman, "Hello 2", "", nil))
	repo.SaveThread(thread2)

	thread3 := model.NewThread("claude-code", "", "", "")
	thread3.AddMessage(model.NewMessage(model.RoleHuman, "Hello 3", "", nil))
	thread3.Status = model.ThreadStatusCommitted // Already committed
	thread3.CommittedContentHash = thread3.ComputeContentHash() // Content hash matches
	repo.SaveThread(thread3)

	// Stage thread1
	repo.StageThread(thread1.ID, len(thread1.Messages), "")

	// Get unstaged
	unstaged, err := repo.GetUnstagedThreads()
	if err != nil {
		t.Fatalf("GetUnstagedThreads failed: %v", err)
	}

	// Should only return thread2 (thread1 is staged, thread3 is committed)
	if len(unstaged) != 1 {
		t.Fatalf("expected 1 unstaged thread, got %d", len(unstaged))
	}
	if unstaged[0].ID != thread2.ID {
		t.Errorf("expected thread2, got %s", unstaged[0].ID)
	}
}

func TestRepository_GetActiveThread(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Initially none
	active, err := repo.GetActiveThread()
	if err != nil {
		t.Fatalf("GetActiveThread failed: %v", err)
	}
	if active != nil {
		t.Error("expected nil active thread initially")
	}

	// Create completed thread
	thread1 := model.NewThread("claude-code", "", "", "")
	thread1.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	thread1.Complete()
	repo.SaveThread(thread1)

	// Still no active
	active, err = repo.GetActiveThread()
	if err != nil {
		t.Fatalf("GetActiveThread failed: %v", err)
	}
	if active != nil {
		t.Error("expected nil active thread when only completed exists")
	}

	// Create active thread
	thread2 := model.NewThread("claude-code", "", "", "")
	thread2.AddMessage(model.NewMessage(model.RoleHuman, "Hello active", "", nil))
	repo.SaveThread(thread2)

	// Now should return active
	active, err = repo.GetActiveThread()
	if err != nil {
		t.Fatalf("GetActiveThread failed: %v", err)
	}
	if active == nil {
		t.Fatal("expected non-nil active thread")
	}
	if active.ID != thread2.ID {
		t.Errorf("expected thread2, got %s", active.ID)
	}
}
