package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danieladler/tin/internal/hooks"
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
	case "session-start":
		return hookSessionStart()
	case "user-prompt":
		return hookUserPrompt()
	case "stop":
		return hookStop()
	case "session-end":
		return hookSessionEnd()
	case "-h", "--help":
		printHooksHelp()
		return nil
	default:
		return fmt.Errorf("unknown hooks subcommand: %s", subcmd)
	}
}

func hooksInstall(args []string) error {
	global := false
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printHooksInstallHelp()
			return nil
		case "--global", "-g":
			global = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := hooks.InstallClaudeCodeHooks(cwd, global); err != nil {
		return err
	}

	if err := hooks.InstallSlashCommands(cwd, global); err != nil {
		return err
	}

	location := "project"
	if global {
		location = "global"
	}
	fmt.Printf("Installed tin hooks for Claude Code (%s)\n", location)
	fmt.Println("\nHooks installed:")
	fmt.Println("  - SessionStart: Creates/resumes thread tracking")
	fmt.Println("  - UserPromptSubmit: Records human messages")
	fmt.Println("  - Stop: Records assistant responses and tool calls")
	fmt.Println("  - SessionEnd: Marks thread as complete")
	fmt.Println("\nSlash commands installed:")
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

func printHooksHelp() {
	fmt.Println(`Manage tin hooks for AI coding agents

Usage: tin hooks <command>

Commands:
  install     Install hooks for Claude Code
  uninstall   Remove hooks from Claude Code

The hooks integration automatically tracks your conversations with
AI coding agents, creating threads that can be staged and committed.

Examples:
  tin hooks install         Install hooks for current project
  tin hooks install -g      Install hooks globally
  tin hooks uninstall       Remove hooks from current project`)
}

func printHooksInstallHelp() {
	fmt.Println(`Install tin hooks for Claude Code

Usage: tin hooks install [options]

Options:
  -g, --global  Install to global Claude Code settings (~/.claude/settings.json)
                instead of project settings (.claude/settings.json)

This command adds hooks to Claude Code that will automatically:
  - Track conversation sessions as threads
  - Record human prompts and assistant responses
  - Capture tool calls and git state changes
  - Auto-stage threads for easy committing

After installation, your conversations will be tracked automatically.
Use 'tin status' to see active threads and 'tin commit' to save your work.`)
}
