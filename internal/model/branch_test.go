package model

import (
	"testing"
)

func TestNewBranch(t *testing.T) {
	branch := NewBranch("feature-auth", "commit-abc123")

	if branch.Name != "feature-auth" {
		t.Errorf("expected name 'feature-auth', got %s", branch.Name)
	}
	if branch.CommitID != "commit-abc123" {
		t.Errorf("expected commit ID 'commit-abc123', got %s", branch.CommitID)
	}
}

func TestNewBranch_EmptyCommit(t *testing.T) {
	branch := NewBranch("main", "")

	if branch.Name != "main" {
		t.Errorf("expected name 'main', got %s", branch.Name)
	}
	if branch.CommitID != "" {
		t.Errorf("expected empty commit ID, got %s", branch.CommitID)
	}
}

func TestBranch_Fields(t *testing.T) {
	branch := &Branch{
		Name:     "develop",
		CommitID: "abc123def456",
	}

	if branch.Name != "develop" {
		t.Errorf("expected name 'develop', got %s", branch.Name)
	}
	if branch.CommitID != "abc123def456" {
		t.Errorf("expected commit ID 'abc123def456', got %s", branch.CommitID)
	}
}
