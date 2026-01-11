package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CursorHooksConfig represents the Cursor hooks.json structure
type CursorHooksConfig struct {
	Version int                    `json:"version"`
	Hooks   map[string][]HookEntry `json:"hooks"`
}

// HookEntry represents a single hook configuration
type HookEntry struct {
	Command string `json:"command"`
}

// getHooksPath returns the path to Cursor hooks.json
func getHooksPath(projectDir string, global bool) string {
	if global {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".cursor", "hooks.json")
	}
	return filepath.Join(projectDir, ".cursor", "hooks.json")
}

// getGlobalHooksPath returns the enterprise/system-level hooks path
func getGlobalHooksPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/Cursor/hooks.json"
	case "linux":
		return "/etc/cursor/hooks.json"
	case "windows":
		return `C:\ProgramData\Cursor\hooks.json`
	default:
		return ""
	}
}

// InstallHooks installs tin hooks into Cursor's hooks.json
func InstallHooks(projectDir string, global bool, timeout int) error {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// Find tin binary path
	tinPath, err := findTinBinary()
	if err != nil {
		return err
	}

	hooksPath := getHooksPath(projectDir, global)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		return err
	}

	// Load existing config or create new
	config := &CursorHooksConfig{
		Version: 1,
		Hooks:   make(map[string][]HookEntry),
	}

	if data, err := os.ReadFile(hooksPath); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not parse existing hooks.json, creating new\n")
			config = &CursorHooksConfig{
				Version: 1,
				Hooks:   make(map[string][]HookEntry),
			}
		}
	}

	if config.Hooks == nil {
		config.Hooks = make(map[string][]HookEntry)
	}

	// Define our hooks
	// Cursor hook events we care about:
	// - beforeSubmitPrompt: User submits a prompt
	// - stop: Agent stops (completion)
	// - afterFileEdit: File was edited (for tracking)
	tinHooks := map[string]string{
		"beforeSubmitPrompt": "cursor-prompt",
		"stop":               "cursor-stop",
		"afterFileEdit":      "cursor-file-edit",
	}

	for event, handler := range tinHooks {
		hookCmd := fmt.Sprintf("%s hook %s", tinPath, handler)

		// Check if hook already exists
		existing := config.Hooks[event]
		alreadyInstalled := false
		for _, hook := range existing {
			if strings.Contains(hook.Command, "tin hook") {
				alreadyInstalled = true
				break
			}
		}

		if !alreadyInstalled {
			config.Hooks[event] = append(config.Hooks[event], HookEntry{
				Command: hookCmd,
			})
		}
	}

	// Write config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(hooksPath, data, 0644)
}

// UninstallHooks removes tin hooks from Cursor's hooks.json
func UninstallHooks(projectDir string, global bool) error {
	hooksPath := getHooksPath(projectDir, global)

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to uninstall
		}
		return err
	}

	var config CursorHooksConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Hooks == nil {
		return nil
	}

	// Remove tin hooks
	events := []string{"beforeSubmitPrompt", "stop", "afterFileEdit"}
	for _, event := range events {
		hooks := config.Hooks[event]
		var filtered []HookEntry
		for _, hook := range hooks {
			if !strings.Contains(hook.Command, "tin hook") {
				filtered = append(filtered, hook)
			}
		}
		if len(filtered) > 0 {
			config.Hooks[event] = filtered
		} else {
			delete(config.Hooks, event)
		}
	}

	// Write config
	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(hooksPath, data, 0644)
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
