package git

import (
	"encoding/json"

	"github.com/danieladler/tin/internal/model"
)

// WriteToolArgs represents the arguments for the Write tool
type WriteToolArgs struct {
	FilePath string `json:"file_path"`
}

// EditToolArgs represents the arguments for the Edit tool
type EditToolArgs struct {
	FilePath string `json:"file_path"`
}

// ExtractModifiedFiles extracts file paths from tool calls in messages.
// It parses Write and Edit tool calls to find which files were modified.
func ExtractModifiedFiles(messages []model.Message) []string {
	seen := make(map[string]bool)
	var files []string

	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			path := extractFilePathFromToolCall(tc)
			if path != "" && !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
		}
	}

	return files
}

func extractFilePathFromToolCall(tc model.ToolCall) string {
	switch tc.Name {
	case "Write":
		var args WriteToolArgs
		if err := json.Unmarshal(tc.Arguments, &args); err == nil {
			return args.FilePath
		}
	case "Edit":
		var args EditToolArgs
		if err := json.Unmarshal(tc.Arguments, &args); err == nil {
			return args.FilePath
		}
	}
	return ""
}
