package commands

import (
	"fmt"
	"os"

	"github.com/sestinj/tin/internal/agents"
	_ "github.com/sestinj/tin/internal/agents/codex" // Register agent
)

// Codex handles the "tin codex" command
func Codex(args []string) error {
	if len(args) == 0 {
		printCodexHelp()
		return nil
	}

	subcmd := args[0]

	switch subcmd {
	case "setup":
		return codexSetup()
	case "-h", "--help":
		printCodexHelp()
		return nil
	default:
		return fmt.Errorf("unknown codex subcommand: %s", subcmd)
	}
}

func codexSetup() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	handler, ok := agents.GetNotify("codex")
	if !ok {
		return fmt.Errorf("codex agent not registered")
	}

	return handler.Setup(cwd)
}

func printCodexHelp() {
	fmt.Println(`Manage Codex CLI (OpenAI) agent integration

Usage: tin codex <command>

Commands:
  setup    Show configuration instructions for Codex CLI

Codex CLI uses a notification-based integration, which requires manual
configuration of your config.toml file. Run 'tin codex setup' for
detailed instructions.

Note: Codex only supports the 'agent-turn-complete' notification event,
so thread tracking is less comprehensive than hook-based agents.`)
}
