package commands

import (
	"os"
	"testing"

	"github.com/sestinj/tin/internal/storage"
)

func setupTestRepo(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)

	// Initialize repo
	_, err := storage.Init(tmpDir)
	if err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	return tmpDir, func() {
		os.Chdir(originalDir)
	}
}

func TestBranch_List_Empty(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Should not error even with no branches
	err := Branch([]string{})
	if err != nil {
		t.Fatalf("Branch list failed: %v", err)
	}
}

func TestBranch_Create(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch
	err := Branch([]string{"feature-test"})
	if err != nil {
		t.Fatalf("Branch create failed: %v", err)
	}

	// Verify branch exists
	repo, _ := storage.Open(tmpDir)
	if !repo.BranchExists("feature-test") {
		t.Error("expected feature-test branch to exist")
	}
}

func TestBranch_CreateDuplicate(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch
	if err := Branch([]string{"feature-test"}); err != nil {
		t.Fatalf("First branch create failed: %v", err)
	}

	// Try to create same branch again
	err := Branch([]string{"feature-test"})
	if err == nil {
		t.Error("expected error when creating duplicate branch")
	}
}

func TestBranch_Delete(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch
	Branch([]string{"feature-test"})

	// Delete it
	err := Branch([]string{"-d", "feature-test"})
	if err != nil {
		t.Fatalf("Branch delete failed: %v", err)
	}

	// Verify it's gone
	repo, _ := storage.Open(tmpDir)
	if repo.BranchExists("feature-test") {
		t.Error("expected feature-test branch to be deleted")
	}
}

func TestBranch_DeleteNonexistent(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Branch([]string{"-d", "nonexistent"})
	if err == nil {
		t.Error("expected error when deleting nonexistent branch")
	}
}

func TestBranch_DeleteCurrentBranch(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create main branch by adding a commit reference
	repo, _ := storage.Open(tmpDir)
	repo.WriteBranch("main", "")

	// Try to delete current branch (main)
	err := Branch([]string{"-d", "main"})
	if err == nil {
		t.Error("expected error when deleting main branch")
	}
}

func TestBranch_HelpFlag(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	err := Branch([]string{"--help"})
	if err != nil {
		t.Errorf("Branch --help should not error: %v", err)
	}
}

func TestBranch_AllFlag(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch
	Branch([]string{"feature-test"})

	// List with -a flag
	err := Branch([]string{"-a"})
	if err != nil {
		t.Fatalf("Branch -a failed: %v", err)
	}
}
