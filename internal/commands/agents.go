package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/sestinj/tin/internal/agents"
	// Import agent packages to trigger their init() registration
	_ "github.com/sestinj/tin/internal/agents/amp"
	_ "github.com/sestinj/tin/internal/agents/claudecode"
	_ "github.com/sestinj/tin/internal/agents/codex"
	_ "github.com/sestinj/tin/internal/agents/cursor"
)

// Agents handles the "tin agents" command
func Agents(args []string) error {
	if len(args) == 0 {
		printAgentsHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "list":
		return agentsList()
	case "status":
		return agentsStatus(subargs)
	case "-h", "--help":
		printAgentsHelp()
		return nil
	default:
		return fmt.Errorf("unknown agents subcommand: %s", subcmd)
	}
}

func agentsList() error {
	infos := agents.List()

	if len(infos) == 0 {
		fmt.Println("No agents registered")
		return nil
	}

	fmt.Println("Registered agents:")
	fmt.Println()

	// Group by paradigm
	hookAgents := []agents.AgentInfo{}
	notifyAgents := []agents.AgentInfo{}
	pullAgents := []agents.AgentInfo{}

	for _, info := range infos {
		switch info.Paradigm {
		case agents.ParadigmHook:
			hookAgents = append(hookAgents, info)
		case agents.ParadigmNotify:
			notifyAgents = append(notifyAgents, info)
		case agents.ParadigmPull:
			pullAgents = append(pullAgents, info)
		}
	}

	if len(hookAgents) > 0 {
		fmt.Println("Hook-based (real-time tracking):")
		for _, info := range hookAgents {
			status := getAgentStatus(info.Name)
			fmt.Printf("  %-15s %s %s\n", info.Name, info.DisplayName, status)
		}
		fmt.Println()
	}

	if len(notifyAgents) > 0 {
		fmt.Println("Notification-based (partial tracking):")
		for _, info := range notifyAgents {
			status := getAgentStatus(info.Name)
			fmt.Printf("  %-15s %s %s\n", info.Name, info.DisplayName, status)
		}
		fmt.Println()
	}

	if len(pullAgents) > 0 {
		fmt.Println("Pull-based (manual sync):")
		for _, info := range pullAgents {
			fmt.Printf("  %-15s %s\n", info.Name, info.DisplayName)
		}
		fmt.Println()
	}

	return nil
}

func getAgentStatus(name string) string {
	cwd, _ := os.Getwd()

	// Check if hooks are installed for hook-based agents
	if handler, ok := agents.GetHook(name); ok {
		installed, _ := handler.IsInstalled(cwd, false)
		globalInstalled, _ := handler.IsInstalled(cwd, true)
		if installed {
			return "(hooks installed: project)"
		}
		if globalInstalled {
			return "(hooks installed: global)"
		}
		return "(not installed)"
	}

	return ""
}

func agentsStatus(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	fmt.Printf("Agent status in: %s\n\n", cwd)

	infos := agents.List()

	for _, info := range infos {
		fmt.Printf("%s (%s):\n", info.DisplayName, info.Paradigm)

		switch info.Paradigm {
		case agents.ParadigmHook:
			handler, _ := agents.GetHook(info.Name)
			if handler != nil {
				projectInstalled, _ := handler.IsInstalled(cwd, false)
				globalInstalled, _ := handler.IsInstalled(cwd, true)

				if projectInstalled {
					fmt.Println("  Hooks: installed (project-level)")
				} else if globalInstalled {
					fmt.Println("  Hooks: installed (global)")
				} else {
					fmt.Println("  Hooks: not installed")
				}
			}

		case agents.ParadigmNotify:
			fmt.Println("  Status: requires manual config.toml setup")
			fmt.Printf("  Run 'tin %s setup' for instructions\n", info.Name)

		case agents.ParadigmPull:
			fmt.Println("  Status: pull-based (use 'tin amp pull' to sync)")
		}
		fmt.Println()
	}

	return nil
}

// HooksInstall handles "tin hooks install" with multi-agent support
func HooksInstall(args []string, global bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Parse which agents to install
	agentNames := []string{}
	installAll := false

	for _, arg := range args {
		switch arg {
		case "--all":
			installAll = true
		case "--claude-code", "--claudecode":
			agentNames = append(agentNames, "claude-code")
		case "--cursor":
			agentNames = append(agentNames, "cursor")
		case "--codex":
			// Codex doesn't use hooks, print instructions instead
			if handler, ok := agents.GetNotify("codex"); ok {
				handler.Setup(cwd)
			}
		default:
			if strings.HasPrefix(arg, "--") {
				return fmt.Errorf("unknown agent: %s", arg)
			}
		}
	}

	// Default to claude-code if no agents specified
	if len(agentNames) == 0 && !installAll {
		agentNames = []string{"claude-code"}
	}

	// Install all hook-based agents if --all
	if installAll {
		for _, name := range agents.List() {
			if name.Paradigm == agents.ParadigmHook {
				agentNames = append(agentNames, name.Name)
			}
		}
	}

	// Install hooks for each agent
	for _, name := range agentNames {
		handler, ok := agents.GetHook(name)
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: %s is not a hook-based agent, skipping\n", name)
			continue
		}

		if err := handler.Install(cwd, global); err != nil {
			return fmt.Errorf("failed to install %s hooks: %w", name, err)
		}

		scope := "project"
		if global {
			scope = "global"
		}
		fmt.Printf("Installed %s hooks (%s)\n", handler.Info().DisplayName, scope)
	}

	return nil
}

// HooksUninstall handles "tin hooks uninstall" with multi-agent support
func HooksUninstall(args []string, global bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Parse which agents to uninstall
	agentNames := []string{}
	uninstallAll := false

	for _, arg := range args {
		switch arg {
		case "--all":
			uninstallAll = true
		case "--claude-code", "--claudecode":
			agentNames = append(agentNames, "claude-code")
		case "--cursor":
			agentNames = append(agentNames, "cursor")
		default:
			if strings.HasPrefix(arg, "--") {
				return fmt.Errorf("unknown agent: %s", arg)
			}
		}
	}

	// Default to claude-code if no agents specified
	if len(agentNames) == 0 && !uninstallAll {
		agentNames = []string{"claude-code"}
	}

	// Uninstall all hook-based agents if --all
	if uninstallAll {
		for _, name := range agents.List() {
			if name.Paradigm == agents.ParadigmHook {
				agentNames = append(agentNames, name.Name)
			}
		}
	}

	// Uninstall hooks for each agent
	for _, name := range agentNames {
		handler, ok := agents.GetHook(name)
		if !ok {
			continue // Skip non-hook agents silently
		}

		if err := handler.Uninstall(cwd, global); err != nil {
			return fmt.Errorf("failed to uninstall %s hooks: %w", name, err)
		}

		scope := "project"
		if global {
			scope = "global"
		}
		fmt.Printf("Uninstalled %s hooks (%s)\n", handler.Info().DisplayName, scope)
	}

	return nil
}

func printAgentsHelp() {
	fmt.Println(`Manage agent integrations

Usage: tin agents <command>

Commands:
  list      List all registered agents
  status    Show agent status in current directory

Agent Types:
  Hook-based (real-time):     claude-code, cursor
  Notification-based:         codex
  Pull-based (manual sync):   amp

Use "tin hooks install --<agent>" to install hooks for specific agents.
Use "tin <agent> pull" for pull-based agents.`)
}
