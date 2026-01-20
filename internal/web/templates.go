package web

import (
	"embed"
	"html/template"
	"io"
	"time"

	"github.com/sestinj/tin/internal/git"
	"github.com/sestinj/tin/internal/model"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates *template.Template

// MaxThreadMessages is the maximum number of messages to show per thread on commit pages
const MaxThreadMessages = 5

func init() {
	funcMap := template.FuncMap{
		"shortID":         shortID,
		"formatTime":      formatTime,
		"roleClass":       roleClass,
		"truncate":        truncate,
		"commitURL":       commitURL,
		"limitMessages":   limitMessages,
		"hasMoreMessages": hasMoreMessages,
		"isMergeCommit":   isMergeCommit,
		"agentIconPath":   agentIconPath,
		"agentIconClass":  agentIconClass,
	}

	templates = template.Must(template.New("").
		Funcs(funcMap).
		ParseFS(templateFS, "templates/*.html"))
}

// shortID returns first 7 characters of an ID (like git short hash)
func shortID(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// formatTime formats a time in a human-readable way
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// roleClass returns a CSS class based on message role
func roleClass(role model.Role) string {
	switch role {
	case model.RoleHuman:
		return "human"
	case model.RoleAssistant:
		return "assistant"
	default:
		return ""
	}
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// commitURL generates a full commit URL if code host is available
func commitURL(codeHost *git.CodeHostURL, hash string) string {
	if codeHost == nil || hash == "" {
		return ""
	}
	return codeHost.CommitURL(hash)
}

// renderTemplate executes a named template with the given data
func renderTemplate(w io.Writer, name string, data interface{}) error {
	return templates.ExecuteTemplate(w, name, data)
}

// limitMessages returns the first N messages from a thread
func limitMessages(messages []model.Message, limit int) []model.Message {
	if len(messages) <= limit {
		return messages
	}
	return messages[:limit]
}

// hasMoreMessages checks if a thread has more than N messages
func hasMoreMessages(messages []model.Message, limit int) bool {
	return len(messages) > limit
}

// isMergeCommit checks if a commit is a merge commit (has two parents)
func isMergeCommit(commit *model.TinCommit) bool {
	return commit != nil && commit.SecondParentID != ""
}

// agentIconPath returns the URL path to an agent's icon
func agentIconPath(agent string) string {
	switch agent {
	case "amp":
		return "/assets/amp-mark-color.svg"
	case "claude-code":
		return "/assets/claude-symbol-clay.png"
	default:
		return ""
	}
}

// agentIconClass returns a CSS class for the agent
func agentIconClass(agent string) string {
	switch agent {
	case "amp":
		return "agent-amp"
	case "claude-code":
		return "agent-claude"
	default:
		return "agent-unknown"
	}
}
