package commands

import (
	"testing"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

func TestStatus_EmptyRepo(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Status([]string{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
}

func TestStatus_WithThreads(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create a thread
	thread := model.NewThread("claude-code", "", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	repo.SaveThread(thread)

	err := Status([]string{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
}

func TestStatus_WithStagedThreads(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create and stage a thread
	thread := model.NewThread("claude-code", "", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	repo.SaveThread(thread)
	repo.StageThread(thread.ID, len(thread.Messages))

	err := Status([]string{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
}

func TestStatus_WithActiveThread(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create an active thread
	thread := model.NewThread("claude-code", "", "", "")
	thread.AddMessage(model.NewMessage(model.RoleHuman, "Hello", "", nil))
	// Status is active by default
	repo.SaveThread(thread)

	err := Status([]string{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
}

func TestStatus_WithCommit(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := storage.Open(tmpDir)

	// Create a commit
	threads := []model.ThreadRef{{ThreadID: "t1", MessageCount: 1}}
	commit := model.NewTinCommit("Test commit", threads, "git123", "")
	repo.SaveCommit(commit)
	repo.WriteBranch("main", commit.ID)

	err := Status([]string{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
}

func TestStatus_HelpFlag(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Status([]string{"--help"})
	if err != nil {
		t.Errorf("Status --help should not error: %v", err)
	}
}
