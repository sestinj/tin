package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dadlerj/tin/internal/git"
	"github.com/dadlerj/tin/internal/remote"
	"github.com/dadlerj/tin/internal/storage"
)

func Push(args []string) error {
	force := false
	remoteName := "origin"
	branch := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printPushHelp()
			return nil
		case "-f", "--force":
			force = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				if remoteName == "origin" && i == 0 {
					remoteName = args[i]
				} else if branch == "" {
					branch = args[i]
				}
			}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	// Check tin/git state alignment (unless force)
	if !force {
		if err := repo.CheckBranchSync(); err != nil {
			if mismatch, ok := err.(*storage.BranchMismatchError); ok {
				return fmt.Errorf("%s\n\nUse 'tin sync' to align states, or 'tin push --force' to proceed anyway", mismatch)
			}
			return err
		}
	}

	// Default to current branch
	if branch == "" {
		branch, err = repo.ReadHead()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Always do git push first
	fmt.Printf("Pushing git to %s/%s...\n", remoteName, branch)
	if err := repo.GitPush(remoteName, branch, force); err != nil {
		return err
	}
	fmt.Printf("Git pushed %s -> %s/%s\n", branch, remoteName, branch)

	// Also push tin data if a tin remote is configured
	remoteConfig, err := repo.GetRemote(remoteName)
	if err == nil {
		fmt.Printf("Pushing tin to %s (%s)...\n", remoteName, remoteConfig.URL)

		// Connect to remote with auth
		client, err := dialWithCredentials(remoteConfig.URL, repo)
		if err != nil {
			return err
		}
		defer client.Close()

		// Push
		if err := client.Push(repo, branch, force); err != nil {
			return err
		}

		fmt.Printf("Tin pushed %s -> %s/%s\n", branch, remoteName, branch)

		// Sync code host URL
		if err := syncCodeHostURL(repo, remoteConfig.URL, remoteName); err != nil {
			// Non-fatal: just warn
			fmt.Printf("Warning: failed to sync code host URL: %v\n", err)
		}
	}

	return nil
}

// dialWithCredentials creates a client with credentials from the credential store
func dialWithCredentials(remoteURL string, repo *storage.Repository) (*remote.Client, error) {
	// Parse URL to get host for credential lookup
	parsedURL, err := remote.ParseURL(remoteURL)
	if err != nil {
		return nil, err
	}

	// Get credentials from store
	credStore := remote.NewCredentialStore()
	creds, _ := credStore.Get(parsedURL.Host)

	return remote.Dial(remoteURL, creds)
}

// syncCodeHostURL ensures the remote tin repo has the correct code_host_url
func syncCodeHostURL(repo *storage.Repository, remoteURL, remoteName string) error {
	// Get local git remote URL
	localGitURL, err := repo.GetGitRemoteURL(remoteName)
	if err != nil {
		return nil // No git remote, nothing to sync
	}

	// Parse and validate local URL
	localCodeHost := git.ParseGitRemoteURL(localGitURL)
	if localCodeHost == nil {
		return nil // Couldn't parse, nothing to sync
	}
	localBaseURL := localCodeHost.BaseURL()
	if localBaseURL == "" {
		return nil // Not a supported code host
	}

	// Connect to remote to check its config
	configClient, err := dialWithCredentials(remoteURL, repo)
	if err != nil {
		return fmt.Errorf("failed to connect for config sync: %w", err)
	}
	defer configClient.Close()

	remoteConfigData, err := configClient.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get remote config: %w", err)
	}

	// Compare and update
	if remoteConfigData.CodeHostURL == "" {
		// Remote has no URL, set it
		fmt.Printf("Setting remote code_host_url to %s\n", localBaseURL)
		setClient, err := dialWithCredentials(remoteURL, repo)
		if err != nil {
			return err
		}
		defer setClient.Close()
		return setClient.SetConfig(&remote.SetConfigMessage{CodeHostURL: localBaseURL})
	}

	if remoteConfigData.CodeHostURL == localBaseURL {
		// URLs match, nothing to do
		return nil
	}

	// URLs differ, prompt user
	fmt.Printf("\nRemote code_host_url mismatch:\n")
	fmt.Printf("  Remote: %s\n", remoteConfigData.CodeHostURL)
	fmt.Printf("  Local:  %s\n", localBaseURL)
	fmt.Printf("Update remote to match local? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		setClient, err := dialWithCredentials(remoteURL, repo)
		if err != nil {
			return err
		}
		defer setClient.Close()
		if err := setClient.SetConfig(&remote.SetConfigMessage{CodeHostURL: localBaseURL}); err != nil {
			return err
		}
		fmt.Printf("Updated remote code_host_url to %s\n", localBaseURL)
	}

	return nil
}

func printPushHelp() {
	fmt.Println(`Usage: tin push [options] [remote] [branch]

Push commits and threads to a remote repository.

Options:
  -f, --force    Force push (overwrite remote even if not fast-forward)

Arguments:
  remote         Remote name (default: origin)
  branch         Branch to push (default: current branch)

Examples:
  tin push
  tin push origin main
  tin push --force origin main`)
}
