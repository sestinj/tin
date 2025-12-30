package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ThreadRef references a thread at a specific message count
type ThreadRef struct {
	ThreadID     string `json:"thread_id"`
	MessageCount int    `json:"message_count"`
}

// TinCommit represents a commit in tin's history
type TinCommit struct {
	ID             string      `json:"id"`
	ParentCommitID string      `json:"parent_commit_id,omitempty"`
	Message        string      `json:"message"`
	Threads        []ThreadRef `json:"threads"`
	GitCommitHash  string      `json:"git_commit_hash"`
	Timestamp      time.Time   `json:"timestamp"`
	Author         string      `json:"author,omitempty"`
}

// NewTinCommit creates a new commit
func NewTinCommit(message string, threads []ThreadRef, gitCommitHash string, parentCommitID string) *TinCommit {
	c := &TinCommit{
		ParentCommitID: parentCommitID,
		Message:        message,
		Threads:        threads,
		GitCommitHash:  gitCommitHash,
		Timestamp:      time.Now().UTC(),
	}
	c.ID = c.ComputeHash()
	return c
}

// ComputeHash generates a SHA256 hash for the commit
func (c *TinCommit) ComputeHash() string {
	h := sha256.New()

	h.Write([]byte(c.ParentCommitID))
	h.Write([]byte(c.Message))
	h.Write([]byte(c.GitCommitHash))
	h.Write([]byte(c.Timestamp.Format(time.RFC3339Nano)))

	threadsJSON, _ := json.Marshal(c.Threads)
	h.Write(threadsJSON)

	return hex.EncodeToString(h.Sum(nil))
}

// ShortID returns the first 8 characters of the commit ID
func (c *TinCommit) ShortID() string {
	if len(c.ID) < 8 {
		return c.ID
	}
	return c.ID[:8]
}

// ThreadCount returns the number of threads in this commit
func (c *TinCommit) ThreadCount() int {
	return len(c.Threads)
}
