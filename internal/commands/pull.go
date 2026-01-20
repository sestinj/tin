package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/sestinj/tin/internal/storage"
)

func Pull(args []string) error {
	remoteName := "origin"
	branch := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printPullHelp()
			return nil
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

	// Default to current branch
	if branch == "" {
		branch, err = repo.ReadHead()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Always do git pull first
	fmt.Printf("Pulling git from %s/%s...\n", remoteName, branch)
	if err := repo.GitPull(remoteName, branch); err != nil {
		return err
	}
	fmt.Printf("Git pulled %s <- %s/%s\n", branch, remoteName, branch)

	// Also pull tin data if a tin remote is configured
	remoteConfig, err := repo.GetRemote(remoteName)
	if err == nil {
		fmt.Printf("Pulling tin from %s (%s)...\n", remoteName, remoteConfig.URL)

		// Connect to remote with auth (prompts for credentials if needed)
		client, err := dialWithCredentials(remoteConfig.URL, repo)
		if err != nil {
			return err
		}
		defer client.Close()

		// Pull
		refs, err := client.Pull(repo, branch)
		if err != nil {
			return err
		}

		// Report what we got
		fmt.Printf("Tin pulled %s <- %s/%s\n", branch, remoteName, branch)
		if commitID, ok := refs.Branches[branch]; ok {
			fmt.Printf("  -> %s\n", commitID[:12])
		}
	}

	return nil
}

func printPullHelp() {
	fmt.Println(`Usage: tin pull [remote] [branch]

Pull commits and threads from a remote repository.

Arguments:
  remote         Remote name (default: origin)
  branch         Branch to pull (default: current branch)

Examples:
  tin pull
  tin pull origin main`)
}
