package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dadlerj/tin/internal/agents"
	_ "github.com/dadlerj/tin/internal/agents/claudecode" // Register agent
	"github.com/dadlerj/tin/internal/agents/codex"
	_ "github.com/dadlerj/tin/internal/agents/cursor" // Register agent
	"github.com/dadlerj/tin/internal/hooks"
)

func Hooks(args []string) error {
	if len(args) == 0 {
		printHooksHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "install":
		return hooksInstall(subargs)
	case "uninstall":
		return hooksUninstall(subargs)
	// Claude Code hooks (legacy, maintained for backwards compatibility)
	case "session-start":
		return hookSessionStart()
	case "user-prompt":
		return hookUserPrompt()
	case "stop":
		return hookStop()
	case "session-end":
		return hookSessionEnd()
	// Cursor hooks
	case "cursor-prompt":
		return hookCursorPrompt()
	case "cursor-stop":
		return hookCursorStop()
	case "cursor-file-edit":
		return hookCursorFileEdit()
	// Codex notification
	case "codex-notify":
		return hookCodexNotify(subargs)
	case "-h", "--help":
		printHooksHelp()
		return nil
	default:
		return fmt.Errorf("unknown hooks subcommand: %s", subcmd)
	}
}

func hooksInstall(args []string) error {
	global := false
	agentNames := []string{}

	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printHooksInstallHelp()
			return nil
		case "--global", "-g":
			global = true
		case "--claude-code", "--claudecode":
			agentNames = append(agentNames, "claude-code")
		case "--cursor":
			agentNames = append(agentNames, "cursor")
		case "--all":
			agentNames = append(agentNames, "claude-code", "cursor")
		}
	}

	// Default to claude-code if no agents specified
	if len(agentNames) == 0 {
		agentNames = []string{"claude-code"}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	location := "project"
	if global {
		location = "global"
	}

	for _, name := range agentNames {
		switch name {
		case "claude-code":
			if err := hooks.InstallClaudeCodeHooks(cwd, global); err != nil {
				return err
			}
			if err := hooks.InstallSlashCommands(cwd, global); err != nil {
				return err
			}
			fmt.Printf("Installed tin hooks for Claude Code (%s)\n", location)

		case "cursor":
			if cursorHandler, ok := agents.GetHook("cursor"); ok {
				if err := cursorHandler.Install(cwd, global); err != nil {
					return err
				}
				fmt.Printf("Installed tin hooks for Cursor (%s)\n", location)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Cursor agent not registered\n")
			}
		}
	}

	fmt.Println("\nHooks installed:")
	fmt.Println("  - SessionStart: Creates/resumes thread tracking")
	fmt.Println("  - UserPromptSubmit: Records human messages")
	fmt.Println("  - Stop: Records assistant responses and tool calls")
	fmt.Println("  - SessionEnd: Marks thread as complete")
	fmt.Println("\nSlash commands installed (Claude Code only):")
	fmt.Println("  - /branches: List all branches (current marked with *)")
	fmt.Println("  - /commit [message]: Commit staged threads")
	fmt.Println("  - /checkout [branch]: Switch to another branch")
	fmt.Println("\nThreads will be auto-staged as you work.")
	fmt.Println("Use '/commit' or 'tin commit -m \"message\"' to commit your work.")

	return nil
}

func hooksUninstall(args []string) error {
	global := false
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			fmt.Println("Usage: tin hooks uninstall [--global]")
			return nil
		case "--global", "-g":
			global = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := hooks.UninstallClaudeCodeHooks(cwd, global); err != nil {
		return err
	}

	if err := hooks.UninstallSlashCommands(cwd, global); err != nil {
		return err
	}

	location := "project"
	if global {
		location = "global"
	}
	fmt.Printf("Uninstalled tin hooks and slash commands from Claude Code (%s)\n", location)

	return nil
}

// Hook event handlers - these read from stdin and process the event

func hookSessionStart() error {
	input, err := readHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil // Not a tin repo, skip silently
	}
	return hooks.HandleSessionStart(input)
}

func hookUserPrompt() error {
	input, err := readHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil // Not a tin repo, skip silently
	}
	return hooks.HandleUserPromptSubmit(input)
}

func hookStop() error {
	input, err := readHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil // Not a tin repo, skip silently
	}
	return hooks.HandleStop(input)
}

func hookSessionEnd() error {
	input, err := readHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil // Not a tin repo, skip silently
	}
	return hooks.HandleSessionEnd(input)
}

func readHookInput() (*hooks.HookInput, error) {
	var input hooks.HookInput
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&input); err != nil {
		// If we can't read input, check if we're even in a tin repo
		// If not, silently succeed - no need to track anything
		cwd, _ := os.Getwd()
		if cwd != "" {
			if _, statErr := os.Stat(cwd + "/.tin"); os.IsNotExist(statErr) {
				return nil, nil // Not a tin repo, skip silently
			}
		}
		return nil, fmt.Errorf("failed to read hook input: %w", err)
	}
	return &input, nil
}

// Cursor hook handlers

// CursorHookInput represents input from Cursor hooks
type CursorHookInput struct {
	ConversationID string `json:"conversation_id"`
	GenerationID   string `json:"generation_id"`
	Model          string `json:"model"`
	HookEventName  string `json:"hook_event_name"`
	WorkspaceRoots []string `json:"workspace_roots"`
	UserEmail      *string `json:"user_email"`
	// For beforeSubmitPrompt
	Prompt string `json:"prompt,omitempty"`
	// For stop
	Text     string `json:"text,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

func hookCursorPrompt() error {
	input, err := readCursorHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil
	}

	// Convert to agents.HookEvent and dispatch
	handler, ok := agents.GetHook("cursor")
	if !ok {
		return nil // Cursor agent not registered
	}

	cwd := ""
	if len(input.WorkspaceRoots) > 0 {
		cwd = input.WorkspaceRoots[0]
	}

	event := &agents.HookEvent{
		Type:      agents.HookEventUserPrompt,
		SessionID: input.ConversationID,
		Cwd:       cwd,
		Timestamp: time.Now().UTC(),
		Prompt:    input.Prompt,
	}

	_, err = handler.HandleEvent(event)
	return err
}

func hookCursorStop() error {
	input, err := readCursorHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil
	}

	handler, ok := agents.GetHook("cursor")
	if !ok {
		return nil
	}

	cwd := ""
	if len(input.WorkspaceRoots) > 0 {
		cwd = input.WorkspaceRoots[0]
	}

	event := &agents.HookEvent{
		Type:      agents.HookEventAssistantStop,
		SessionID: input.ConversationID,
		Cwd:       cwd,
		Timestamp: time.Now().UTC(),
		Response:  input.Text,
	}

	_, err = handler.HandleEvent(event)
	return err
}

func hookCursorFileEdit() error {
	input, err := readCursorHookInput()
	if err != nil {
		return err
	}
	if input == nil {
		return nil
	}

	handler, ok := agents.GetHook("cursor")
	if !ok {
		return nil
	}

	cwd := ""
	if len(input.WorkspaceRoots) > 0 {
		cwd = input.WorkspaceRoots[0]
	}

	event := &agents.HookEvent{
		Type:      agents.HookEventFileEdit,
		SessionID: input.ConversationID,
		Cwd:       cwd,
		Timestamp: time.Now().UTC(),
	}

	_, err = handler.HandleEvent(event)
	return err
}

func readCursorHookInput() (*CursorHookInput, error) {
	var input CursorHookInput
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&input); err != nil {
		cwd, _ := os.Getwd()
		if cwd != "" {
			if _, statErr := os.Stat(cwd + "/.tin"); os.IsNotExist(statErr) {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to read Cursor hook input: %w", err)
	}
	return &input, nil
}

// Codex notification handler

func hookCodexNotify(args []string) error {
	// Codex passes JSON as first argument, not stdin
	event, err := codex.ParseNotifyArgs(args)
	if err != nil {
		return err
	}

	handler, ok := agents.GetNotify("codex")
	if !ok {
		return nil // Codex agent not registered
	}

	return handler.HandleNotification(event)
}

func printHooksHelp() {
	fmt.Println(`Manage tin hooks for AI coding agents

Usage: tin hooks <command>

Commands:
  install     Install hooks for AI agents
  uninstall   Remove hooks from AI agents

Supported agents:
  --claude-code   Claude Code (default)
  --cursor        Cursor IDE
  --all           All hook-based agents

The hooks integration automatically tracks your conversations with
AI coding agents, creating threads that can be staged and committed.

Examples:
  tin hooks install               Install Claude Code hooks (project)
  tin hooks install -g            Install Claude Code hooks (global)
  tin hooks install --cursor      Install Cursor hooks
  tin hooks install --all         Install all available hooks
  tin hooks uninstall             Remove hooks from current project`)
}

func printHooksInstallHelp() {
	fmt.Println(`Install tin hooks for AI coding agents

Usage: tin hooks install [options] [--agent]

Options:
  -g, --global    Install globally instead of project-level

Agents:
  --claude-code   Claude Code (default if no agent specified)
  --cursor        Cursor IDE
  --all           All hook-based agents

This command adds hooks that will automatically:
  - Track conversation sessions as threads
  - Record human prompts and assistant responses
  - Capture tool calls and git state changes
  - Auto-stage threads for easy committing

After installation, your conversations will be tracked automatically.
Use 'tin status' to see active threads and 'tin commit' to save your work.

For Codex CLI (notification-based), run 'tin codex setup' for instructions.`)
}
