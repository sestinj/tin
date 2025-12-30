package commands

import (
	"fmt"
	"os"

	"github.com/danieladler/tin/internal/storage"
)

func Init(args []string) error {
	bare := false
	path := ""

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printInitHelp()
			return nil
		case "--bare":
			bare = true
		default:
			if path == "" {
				path = args[i]
			}
		}
	}

	// Get path (default to current directory for normal repos)
	if path == "" {
		if bare {
			return fmt.Errorf("path required for bare repository")
		}
		var err error
		path, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Initialize repository
	if bare {
		repo, err := storage.InitBare(path)
		if err != nil {
			if err == storage.ErrAlreadyExists {
				return fmt.Errorf("tin repository already exists in %s", path)
			}
			return fmt.Errorf("failed to initialize bare repository: %w", err)
		}
		fmt.Printf("Initialized empty bare tin repository in %s\n", repo.RootPath)
	} else {
		repo, err := storage.Init(path)
		if err != nil {
			if err == storage.ErrAlreadyExists {
				return fmt.Errorf("tin repository already exists in %s", path)
			}
			return fmt.Errorf("failed to initialize repository: %w", err)
		}
		fmt.Printf("Initialized empty tin repository in %s/.tin/\n", repo.RootPath)
	}

	return nil
}

func printInitHelp() {
	fmt.Println(`Initialize a new tin repository

Usage: tin init [options] [path]

Options:
  --bare    Create a bare repository (for use as a remote)

This command creates an empty tin repository - essentially a .tin directory
with subdirectories for threads, commits, and refs. It also initializes a
git repository if one doesn't exist.

Use --bare to create a bare repository suitable for use as a remote. Bare
repositories have no working tree and are used for tin push/pull operations.

Examples:
  tin init                        # Initialize in current directory
  tin init --bare /path/to/repo.tin  # Create bare repo for remote use`)
}
