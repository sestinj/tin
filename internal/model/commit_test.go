package model

import (
	"testing"
	"time"
)

func TestNewTinCommit(t *testing.T) {
	threads := []ThreadRef{
		{ThreadID: "thread-1", MessageCount: 10},
		{ThreadID: "thread-2", MessageCount: 5},
	}

	commit := NewTinCommit("Test commit", threads, "gitabc123", "parent-commit-id")

	if commit.Message != "Test commit" {
		t.Errorf("expected message 'Test commit', got %s", commit.Message)
	}
	if commit.GitCommitHash != "gitabc123" {
		t.Errorf("expected git hash 'gitabc123', got %s", commit.GitCommitHash)
	}
	if commit.ParentCommitID != "parent-commit-id" {
		t.Errorf("expected parent 'parent-commit-id', got %s", commit.ParentCommitID)
	}
	if commit.ID == "" {
		t.Error("expected non-empty ID")
	}
	if commit.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if len(commit.Threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(commit.Threads))
	}
}

func TestNewTinCommit_NoParent(t *testing.T) {
	threads := []ThreadRef{
		{ThreadID: "thread-1", MessageCount: 10},
	}

	commit := NewTinCommit("Initial commit", threads, "gitabc123", "")

	if commit.ParentCommitID != "" {
		t.Errorf("expected empty parent, got %s", commit.ParentCommitID)
	}
	if commit.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestTinCommit_ComputeHash(t *testing.T) {
	threads := []ThreadRef{
		{ThreadID: "thread-1", MessageCount: 10},
	}

	commit := &TinCommit{
		ParentCommitID: "",
		Message:        "Test",
		Threads:        threads,
		GitCommitHash:  "git123",
		Timestamp:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	hash1 := commit.ComputeHash()
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}

	// Same content should produce same hash
	hash2 := commit.ComputeHash()
	if hash1 != hash2 {
		t.Error("identical commits should produce identical hashes")
	}
}

func TestTinCommit_ComputeHash_DifferentMessages(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	threads := []ThreadRef{{ThreadID: "t1", MessageCount: 1}}

	commit1 := &TinCommit{
		Message:       "Message A",
		Threads:       threads,
		GitCommitHash: "git123",
		Timestamp:     timestamp,
	}

	commit2 := &TinCommit{
		Message:       "Message B",
		Threads:       threads,
		GitCommitHash: "git123",
		Timestamp:     timestamp,
	}

	if commit1.ComputeHash() == commit2.ComputeHash() {
		t.Error("different messages should produce different hashes")
	}
}

func TestTinCommit_ComputeHash_DifferentThreads(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	commit1 := &TinCommit{
		Message:       "Same",
		Threads:       []ThreadRef{{ThreadID: "t1", MessageCount: 1}},
		GitCommitHash: "git123",
		Timestamp:     timestamp,
	}

	commit2 := &TinCommit{
		Message:       "Same",
		Threads:       []ThreadRef{{ThreadID: "t2", MessageCount: 1}},
		GitCommitHash: "git123",
		Timestamp:     timestamp,
	}

	if commit1.ComputeHash() == commit2.ComputeHash() {
		t.Error("different threads should produce different hashes")
	}
}

func TestTinCommit_ComputeHash_DifferentParent(t *testing.T) {
	timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	threads := []ThreadRef{{ThreadID: "t1", MessageCount: 1}}

	commit1 := &TinCommit{
		ParentCommitID: "parent1",
		Message:        "Same",
		Threads:        threads,
		GitCommitHash:  "git123",
		Timestamp:      timestamp,
	}

	commit2 := &TinCommit{
		ParentCommitID: "parent2",
		Message:        "Same",
		Threads:        threads,
		GitCommitHash:  "git123",
		Timestamp:      timestamp,
	}

	if commit1.ComputeHash() == commit2.ComputeHash() {
		t.Error("different parents should produce different hashes")
	}
}

func TestTinCommit_ShortID(t *testing.T) {
	commit := &TinCommit{
		ID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	}

	short := commit.ShortID()
	if short != "abcdef12" {
		t.Errorf("expected 'abcdef12', got %s", short)
	}
	if len(short) != 8 {
		t.Errorf("expected length 8, got %d", len(short))
	}
}

func TestTinCommit_ShortID_ShortHash(t *testing.T) {
	commit := &TinCommit{
		ID: "abc",
	}

	short := commit.ShortID()
	if short != "abc" {
		t.Errorf("expected 'abc', got %s", short)
	}
}

func TestTinCommit_ShortID_EmptyHash(t *testing.T) {
	commit := &TinCommit{
		ID: "",
	}

	short := commit.ShortID()
	if short != "" {
		t.Errorf("expected '', got %s", short)
	}
}

func TestTinCommit_ThreadCount(t *testing.T) {
	tests := []struct {
		threads []ThreadRef
		want    int
	}{
		{nil, 0},
		{[]ThreadRef{}, 0},
		{[]ThreadRef{{ThreadID: "t1", MessageCount: 1}}, 1},
		{[]ThreadRef{
			{ThreadID: "t1", MessageCount: 1},
			{ThreadID: "t2", MessageCount: 2},
			{ThreadID: "t3", MessageCount: 3},
		}, 3},
	}

	for _, tt := range tests {
		commit := &TinCommit{Threads: tt.threads}
		if got := commit.ThreadCount(); got != tt.want {
			t.Errorf("ThreadCount() with %d threads = %d, want %d", len(tt.threads), got, tt.want)
		}
	}
}

func TestThreadRef(t *testing.T) {
	ref := ThreadRef{
		ThreadID:     "test-thread-id",
		MessageCount: 42,
	}

	if ref.ThreadID != "test-thread-id" {
		t.Errorf("expected thread ID 'test-thread-id', got %s", ref.ThreadID)
	}
	if ref.MessageCount != 42 {
		t.Errorf("expected message count 42, got %d", ref.MessageCount)
	}
}

func TestTinCommit_Timestamp_IsUTC(t *testing.T) {
	threads := []ThreadRef{{ThreadID: "t1", MessageCount: 1}}
	commit := NewTinCommit("Test", threads, "git123", "")

	if commit.Timestamp.Location() != time.UTC {
		t.Error("Timestamp should be in UTC")
	}
}
