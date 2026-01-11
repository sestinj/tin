package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeSettings represents the Claude Code settings.json structure
type ClaudeSettings struct {
	Hooks map[string][]HookMatcher `json:"hooks,omitempty"`
}

// HookMatcher represents a hook matcher configuration
type HookMatcher struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []HookConfig `json:"hooks"`
}

// HookConfig represents a single hook configuration
type HookConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// getSettingsPath returns the path to Claude Code settings.json
func getSettingsPath(projectDir string, global bool) string {
	if global {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".claude", "settings.json")
	}
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// InstallHooks installs tin hooks into Claude Code settings
func InstallHooks(projectDir string, global bool, timeout int) error {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// Find tin binary path
	tinPath, err := findTinBinary()
	if err != nil {
		return err
	}

	settingsPath := getSettingsPath(projectDir, global)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}

	// Load existing settings or create new
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookMatcher),
	}

	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not parse existing settings, creating new\n")
			settings = &ClaudeSettings{
				Hooks: make(map[string][]HookMatcher),
			}
		}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]HookMatcher)
	}

	// Define our hooks
	tinHooks := map[string]string{
		"SessionStart":     "session-start",
		"UserPromptSubmit": "user-prompt",
		"Stop":             "stop",
		"SessionEnd":       "session-end",
	}

	for event, handler := range tinHooks {
		hookCmd := fmt.Sprintf("%s hook %s", tinPath, handler)

		// Check if hook already exists
		existing := settings.Hooks[event]
		alreadyInstalled := false
		for _, matcher := range existing {
			for _, hook := range matcher.Hooks {
				if strings.Contains(hook.Command, "tin hook") {
					alreadyInstalled = true
					break
				}
			}
		}

		if !alreadyInstalled {
			settings.Hooks[event] = append(settings.Hooks[event], HookMatcher{
				Hooks: []HookConfig{{
					Type:    "command",
					Command: hookCmd,
					Timeout: timeout,
				}},
			})
		}
	}

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0644)
}

// UninstallHooks removes tin hooks from Claude Code settings
func UninstallHooks(projectDir string, global bool) error {
	settingsPath := getSettingsPath(projectDir, global)

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to uninstall
		}
		return err
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	if settings.Hooks == nil {
		return nil
	}

	// Remove tin hooks
	events := []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"}
	for _, event := range events {
		matchers := settings.Hooks[event]
		var filtered []HookMatcher
		for _, matcher := range matchers {
			var filteredHooks []HookConfig
			for _, hook := range matcher.Hooks {
				if !strings.Contains(hook.Command, "tin hook") {
					filteredHooks = append(filteredHooks, hook)
				}
			}
			if len(filteredHooks) > 0 {
				matcher.Hooks = filteredHooks
				filtered = append(filtered, matcher)
			}
		}
		if len(filtered) > 0 {
			settings.Hooks[event] = filtered
		} else {
			delete(settings.Hooks, event)
		}
	}

	// Write settings
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0644)
}

// findTinBinary finds the absolute path to the tin binary
func findTinBinary() (string, error) {
	// First, try to find in PATH
	path, err := exec.LookPath("tin")
	if err == nil {
		return filepath.Abs(path)
	}

	// Try current executable
	exe, err := os.Executable()
	if err == nil {
		return filepath.Abs(exe)
	}

	return "", fmt.Errorf("could not find tin binary. Make sure it's in your PATH")
}

// Slash command definitions
var slashCommands = map[string]string{
	"branches": `---
description: List all tin branches, marking the current one
allowed-tools: Bash(tin branch:*)
---

Run ` + "`tin branch`" + ` and display the output to show all branches. The current branch is marked with *.
`,

	"commit": `---
description: Commit the current conversation thread to tin
allowed-tools: Bash(tin commit:*), Bash(tin status:*)
argument-hint: [message]
---

Commit the staged tin threads.

If $ARGUMENTS is provided, use it as the commit message:
` + "```" + `
tin commit -m "$ARGUMENTS"
` + "```" + `

If no message is provided, first run ` + "`tin status`" + ` to show what will be committed, then ask the user for a commit message before running the commit.
`,

	"checkout": `---
description: Switch to a different tin branch
allowed-tools: Bash(tin checkout:*), Bash(tin branch:*)
argument-hint: [branch]
---

Switch to a different tin branch.

If $ARGUMENTS is provided, checkout that branch:
` + "```" + `
tin checkout $ARGUMENTS
` + "```" + `

If no branch name is provided, first run ` + "`tin branch`" + ` to show available branches, then ask the user which branch to checkout.
`,
}

// InstallSlashCommands installs tin slash commands to Claude Code
func InstallSlashCommands(projectDir string, global bool) error {
	var commandsDir string
	if global {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		commandsDir = filepath.Join(homeDir, ".claude", "commands")
	} else {
		commandsDir = filepath.Join(projectDir, ".claude", "commands")
	}

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	for name, content := range slashCommands {
		filePath := filepath.Join(commandsDir, name+".md")

		// Check if file already exists with tin content
		if existingData, err := os.ReadFile(filePath); err == nil {
			if strings.Contains(string(existingData), "tin branch") ||
				strings.Contains(string(existingData), "tin commit") ||
				strings.Contains(string(existingData), "tin checkout") {
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s already exists and is not a tin command, skipping\n", filePath)
			continue
		}

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filePath, err)
		}
	}

	return nil
}

// UninstallSlashCommands removes tin slash commands from Claude Code
func UninstallSlashCommands(projectDir string, global bool) error {
	var commandsDir string
	if global {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		commandsDir = filepath.Join(homeDir, ".claude", "commands")
	} else {
		commandsDir = filepath.Join(projectDir, ".claude", "commands")
	}

	for name := range slashCommands {
		filePath := filepath.Join(commandsDir, name+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if strings.Contains(string(data), "tin branch") ||
				strings.Contains(string(data), "tin commit") ||
				strings.Contains(string(data), "tin checkout") {
				if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove %s: %w", filePath, err)
				}
			}
		}
	}

	return nil
}
