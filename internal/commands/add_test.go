package commands

import (
	"testing"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

func TestAdd_SingleThread(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create a thread
	thread := model.NewThread("claude-code", "", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	repo.SaveThread(thread)

	// Add it
	err := Add([]string{thread.ID[:8]})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify staged
	staged, _ := repo.GetStagedThreads()
	if len(staged) != 1 {
		t.Fatalf("expected 1 staged thread, got %d", len(staged))
	}
	if staged[0].ThreadID != thread.ID {
		t.Errorf("expected thread %s, got %s", thread.ID, staged[0].ThreadID)
	}
}

func TestAdd_AllFlag(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create multiple threads
	for i := 0; i < 3; i++ {
		thread := model.NewThread("claude-code", "", "", "")
		thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
		repo.SaveThread(thread)
	}

	// Add all
	err := Add([]string{"--all"})
	if err != nil {
		t.Fatalf("Add --all failed: %v", err)
	}

	// Verify all staged
	staged, _ := repo.GetStagedThreads()
	if len(staged) != 3 {
		t.Fatalf("expected 3 staged threads, got %d", len(staged))
	}
}

func TestAdd_AllFlag_NoThreads(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add all with no threads should succeed
	err := Add([]string{"--all"})
	if err != nil {
		t.Fatalf("Add --all with no threads should succeed: %v", err)
	}
}

func TestAdd_NoArgs(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add with no args should fail
	err := Add([]string{})
	if err == nil {
		t.Error("expected error with no arguments")
	}
}

func TestAdd_NonexistentThread(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Add([]string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent thread")
	}
}

func TestAdd_PartialThread(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create a thread with multiple messages
	thread := model.NewThread("claude-code", "", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	thread.AddMessage(model.NewMessage(model.RoleAssistant, "Hi", "", nil))
	thread.AddMessage(model.NewMessage(model.RoleHuman, "How are you?", "", nil))
	repo.SaveThread(thread)

	// Add only first 2 messages (using @N syntax)
	err := Add([]string{thread.ID[:8] + "@2"})
	if err != nil {
		t.Fatalf("Add partial failed: %v", err)
	}

	// Verify staged with correct message count
	staged, _ := repo.GetStagedThreads()
	if len(staged) != 1 {
		t.Fatalf("expected 1 staged thread, got %d", len(staged))
	}
	if staged[0].MessageCount != 2 {
		t.Errorf("expected 2 messages staged, got %d", staged[0].MessageCount)
	}
}

func TestAdd_HelpFlag(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Add([]string{"--help"})
	if err != nil {
		t.Errorf("Add --help should not error: %v", err)
	}
}
