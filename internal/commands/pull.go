package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/danieladler/tin/internal/remote"
	"github.com/danieladler/tin/internal/storage"
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

	// Get remote URL
	remoteConfig, err := repo.GetRemote(remoteName)
	if err != nil {
		return fmt.Errorf("remote '%s' not found", remoteName)
	}

	fmt.Printf("Pulling from %s (%s)...\n", remoteName, remoteConfig.URL)

	// Connect to remote
	client, err := remote.Dial(remoteConfig.URL)
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
	fmt.Printf("Updated %s from %s/%s\n", branch, remoteName, branch)
	if commitID, ok := refs.Branches[branch]; ok {
		fmt.Printf("  -> %s\n", commitID[:12])
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
