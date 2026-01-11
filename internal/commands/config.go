package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/dadlerj/tin/internal/remote"
	"github.com/dadlerj/tin/internal/storage"
)

// Known config keys with descriptions
var configKeys = map[string]string{
	"thread_host_url": "Base URL for tin web viewer (e.g., http://localhost:8080)",
	"code_host_url":   "URL for code repository (e.g., https://github.com/user/repo)",
	"auth_token":      "Authentication token for remote operations (th_xxx)",
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
	case "credentials":
		return configCredentials(args[1:])
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

	if config.AuthToken != "" {
		// Mask the token for display
		fmt.Printf("auth_token = %s***\n", config.AuthToken[:min(6, len(config.AuthToken))])
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
	case "auth_token":
		if config.AuthToken != "" {
			fmt.Println(config.AuthToken)
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
	case "auth_token":
		config.AuthToken = value
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

// configCredentials handles credential management subcommands
func configCredentials(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	credStore := remote.NewCredentialStore(repo.RootPath)

	if len(args) == 0 {
		// List credentials
		return credentialsList(repo)
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("usage: tin config credentials add <host> <token>")
		}
		host := args[1]
		token := args[2]
		if err := credStore.Store(host, token); err != nil {
			return err
		}
		fmt.Printf("Stored credentials for %s\n", host)
		return nil

	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: tin config credentials remove <host>")
		}
		host := args[1]
		if err := credStore.Remove(host); err != nil {
			return err
		}
		fmt.Printf("Removed credentials for %s\n", host)
		return nil

	case "list":
		return credentialsList(repo)

	default:
		return fmt.Errorf("unknown credentials subcommand: %s\n\nUsage:\n  tin config credentials [list]           List stored credentials\n  tin config credentials add <host> <token>  Add credentials for a host\n  tin config credentials remove <host>       Remove credentials for a host", args[0])
	}
}

func credentialsList(repo *storage.Repository) error {
	config, err := repo.ReadConfig()
	if err != nil {
		return err
	}

	if len(config.Credentials) == 0 {
		fmt.Println("No stored credentials")
		if config.AuthToken != "" {
			fmt.Printf("\nLegacy auth_token: %s***\n", config.AuthToken[:min(6, len(config.AuthToken))])
		}
		return nil
	}

	fmt.Println("Stored credentials:")
	for _, cred := range config.Credentials {
		// Mask the token for display
		maskedToken := cred.Token[:min(6, len(cred.Token))] + "***"
		fmt.Printf("  %s = %s\n", cred.Host, maskedToken)
	}

	if config.AuthToken != "" {
		fmt.Printf("\nLegacy auth_token: %s***\n", config.AuthToken[:min(6, len(config.AuthToken))])
	}

	return nil
}

func printConfigHelp() {
	fmt.Println(`Usage: tin config [command]

View and modify tin configuration.

Commands:
  (none), list      Show all configuration values
  get <key>         Get a specific configuration value
  set <key> <value> Set a configuration value
  credentials       Manage per-host credentials (see below)

Credentials Commands:
  credentials [list]              List stored credentials
  credentials add <host> <token>  Store credentials for a host
  credentials remove <host>       Remove credentials for a host

Available config keys:
  thread_host_url  Base URL for tin web viewer (e.g., http://localhost:8080)
  code_host_url    URL for code repository (e.g., https://github.com/user/repo)
  auth_token       Legacy auth token (deprecated, use credentials instead)

Environment Variables:
  TIN_AUTH_TOKEN   If set, overrides all stored credentials

Examples:
  tin config                                      # List all config
  tin config get thread_host_url                  # Get thread host URL
  tin config set thread_host_url http://localhost:8080
  tin config credentials add tinhub.dev th_xxxxx  # Store credential for host
  tin config credentials                          # List stored credentials`)
}
