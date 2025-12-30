package model

// Branch represents a named pointer to a commit
type Branch struct {
	Name     string `json:"name"`
	CommitID string `json:"commit_id"`
}

// NewBranch creates a new branch pointing to a commit
func NewBranch(name string, commitID string) *Branch {
	return &Branch{
		Name:     name,
		CommitID: commitID,
	}
}
