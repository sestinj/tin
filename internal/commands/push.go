package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/danieladler/tin/internal/remote"
	"github.com/danieladler/tin/internal/storage"
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

	fmt.Printf("Pushing to %s (%s)...\n", remoteName, remoteConfig.URL)

	// Connect to remote
	client, err := remote.Dial(remoteConfig.URL)
	if err != nil {
		return err
	}
	defer client.Close()

	// Push
	if err := client.Push(repo, branch, force); err != nil {
		return err
	}

	fmt.Printf("Pushed %s -> %s/%s\n", branch, remoteName, branch)
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
