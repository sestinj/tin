package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/dadlerj/tin/internal/storage"
)

// Known config keys with descriptions
var configKeys = map[string]string{
	"thread_host_url": "Base URL for tin web viewer (e.g., http://localhost:8080)",
	"code_host_url":   "URL for code repository (e.g., https://github.com/user/repo)",
}

func Config(args []string) error {
	if len(args) == 0 {
		return configList()
	}

	switch args[0] {
	case "-h", "--help":
		printConfigHelp()
		return nil
	case "list":
		return configList()
	case "get":
		return configGet(args[1:])
	case "set":
		return configSet(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func configList() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	config, err := repo.ReadConfig()
	if err != nil {
		return err
	}

	fmt.Printf("version = %d\n", config.Version)

	if config.ThreadHostURL != "" {
		fmt.Printf("thread_host_url = %s\n", config.ThreadHostURL)
	}

	if config.CodeHostURL != "" {
		fmt.Printf("code_host_url = %s\n", config.CodeHostURL)
	}

	if len(config.Remotes) > 0 {
		fmt.Println("\nRemotes:")
		for _, r := range config.Remotes {
			fmt.Printf("  %s = %s\n", r.Name, r.URL)
		}
	}

	return nil
}

func configGet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tin config get <key>")
	}

	key := strings.ToLower(args[0])

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	config, err := repo.ReadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "thread_host_url":
		if config.ThreadHostURL != "" {
			fmt.Println(config.ThreadHostURL)
		}
	case "code_host_url":
		if config.CodeHostURL != "" {
			fmt.Println(config.CodeHostURL)
		}
	case "version":
		fmt.Println(config.Version)
	default:
		return fmt.Errorf("unknown config key: %s\n\nAvailable keys:\n%s", key, formatAvailableKeys())
	}

	return nil
}

func configSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tin config set <key> <value>")
	}

	key := strings.ToLower(args[0])
	value := args[1]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	config, err := repo.ReadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "thread_host_url":
		// Normalize: remove trailing slash
		config.ThreadHostURL = strings.TrimSuffix(value, "/")
	case "code_host_url":
		config.CodeHostURL = strings.TrimSuffix(value, "/")
	default:
		return fmt.Errorf("unknown config key: %s\n\nAvailable keys:\n%s", key, formatAvailableKeys())
	}

	if err := repo.WriteConfig(config); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func formatAvailableKeys() string {
	var lines []string
	for key, desc := range configKeys {
		lines = append(lines, fmt.Sprintf("  %s - %s", key, desc))
	}
	return strings.Join(lines, "\n")
}

func printConfigHelp() {
	fmt.Println(`Usage: tin config [command]

View and modify tin configuration.

Commands:
  (none), list     Show all configuration values
  get <key>        Get a specific configuration value
  set <key> <value> Set a configuration value

Available keys:
  thread_host_url  Base URL for tin web viewer (e.g., http://localhost:8080)
  code_host_url    URL for code repository (e.g., https://github.com/user/repo)

Examples:
  tin config                                    # List all config
  tin config get thread_host_url                # Get thread host URL
  tin config set thread_host_url http://localhost:8080`)
}
