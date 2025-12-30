package git

import (
	"regexp"
	"strings"
)

// CodeHostURL represents a parsed code host URL
type CodeHostURL struct {
	Host  string // e.g., "github.com"
	Owner string // e.g., "dadlerj"
	Repo  string // e.g., "tin"
}

// SSH format: git@github.com:owner/repo.git
var sshPattern = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)

// HTTPS format: https://github.com/owner/repo.git
var httpsPattern = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)

// ParseGitRemoteURL parses a git remote URL and extracts code host information.
// Supports GitHub SSH and HTTPS formats.
func ParseGitRemoteURL(url string) *CodeHostURL {
	url = strings.TrimSpace(url)

	// Try SSH format first
	if matches := sshPattern.FindStringSubmatch(url); matches != nil {
		return &CodeHostURL{
			Host:  matches[1],
			Owner: matches[2],
			Repo:  matches[3],
		}
	}

	// Try HTTPS format
	if matches := httpsPattern.FindStringSubmatch(url); matches != nil {
		return &CodeHostURL{
			Host:  matches[1],
			Owner: matches[2],
			Repo:  matches[3],
		}
	}

	return nil
}

// CommitURL generates the full URL to view a commit on the code host
func (c *CodeHostURL) CommitURL(hash string) string {
	// Currently only supports GitHub
	if c.Host == "github.com" {
		return "https://github.com/" + c.Owner + "/" + c.Repo + "/commit/" + hash
	}
	return ""
}

// BaseURL returns the base URL for the repository
func (c *CodeHostURL) BaseURL() string {
	if c.Host == "github.com" {
		return "https://github.com/" + c.Owner + "/" + c.Repo
	}
	return ""
}
