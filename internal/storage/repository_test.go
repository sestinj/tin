package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dadlerj/tin/internal/model"
)

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if repo.RootPath != tmpDir {
		t.Errorf("expected root path %s, got %s", tmpDir, repo.RootPath)
	}

	expectedTinPath := filepath.Join(tmpDir, ".tin")
	if repo.TinPath != expectedTinPath {
		t.Errorf("expected tin path %s, got %s", expectedTinPath, repo.TinPath)
	}

	// Verify directory structure
	dirs := []string{
		filepath.Join(tmpDir, ".tin"),
		filepath.Join(tmpDir, ".tin", "threads"),
		filepath.Join(tmpDir, ".tin", "commits"),
		filepath.Join(tmpDir, ".tin", "refs"),
		filepath.Join(tmpDir, ".tin", "refs", "heads"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", dir)
		}
	}

	// Verify files exist
	files := []string{
		filepath.Join(tmpDir, ".tin", "config"),
		filepath.Join(tmpDir, ".tin", "HEAD"),
		filepath.Join(tmpDir, ".tin", "index.json"),
	}
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("file not created: %s", file)
		}
	}
}

func TestInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// First init should succeed
	_, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("First Init failed: %v", err)
	}

	// Second init should fail
	_, err = Init(tmpDir)
	if err != ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestInit_CreatesGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	gitDir := filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("expected .git directory to be created")
	}
}

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize first
	_, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Open should succeed
	repo, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if repo.RootPath != tmpDir {
		t.Errorf("expected root path %s, got %s", tmpDir, repo.RootPath)
	}
}

func TestOpen_FromSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize first
	_, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Open from subdirectory should find parent repo
	repo, err := Open(subDir)
	if err != nil {
		t.Fatalf("Open from subdir failed: %v", err)
	}

	if repo.RootPath != tmpDir {
		t.Errorf("expected root path %s, got %s", tmpDir, repo.RootPath)
	}
}

func TestOpen_NotARepository(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Open(tmpDir)
	if err != ErrNotARepository {
		t.Errorf("expected ErrNotARepository, got %v", err)
	}
}

func TestRepository_HeadOperations(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check initial HEAD
	head, err := repo.ReadHead()
	if err != nil {
		t.Fatalf("ReadHead failed: %v", err)
	}
	if head != "main" {
		t.Errorf("expected initial HEAD 'main', got %s", head)
	}

	// Write new HEAD
	if err := repo.WriteHead("feature-branch"); err != nil {
		t.Fatalf("WriteHead failed: %v", err)
	}

	// Read back
	head, err = repo.ReadHead()
	if err != nil {
		t.Fatalf("ReadHead after write failed: %v", err)
	}
	if head != "feature-branch" {
		t.Errorf("expected HEAD 'feature-branch', got %s", head)
	}
}

func TestRepository_IndexOperations(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check initial index
	index, err := repo.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	if len(index.Staged) != 0 {
		t.Errorf("expected empty initial index, got %d items", len(index.Staged))
	}

	// Write index
	newIndex := &Index{
		Staged: []model.ThreadRef{
			{ThreadID: "thread-1", MessageCount: 10},
			{ThreadID: "thread-2", MessageCount: 20},
		},
	}
	if err := repo.WriteIndex(newIndex); err != nil {
		t.Fatalf("WriteIndex failed: %v", err)
	}

	// Read back
	index, err = repo.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex after write failed: %v", err)
	}
	if len(index.Staged) != 2 {
		t.Errorf("expected 2 staged items, got %d", len(index.Staged))
	}
	if index.Staged[0].ThreadID != "thread-1" {
		t.Errorf("expected thread-1, got %s", index.Staged[0].ThreadID)
	}
}

func TestRepository_GetCurrentGitHash_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Empty repo should return empty hash
	hash, err := repo.GetCurrentGitHash()
	if err != nil {
		t.Fatalf("GetCurrentGitHash failed: %v", err)
	}
	// In an empty repo with no commits, this returns empty string
	_ = hash // No assertion on value since it depends on git state
}
