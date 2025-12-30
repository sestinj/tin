package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/danieladler/tin/internal/storage"
)

func Add(args []string) error {
	// Parse flags
	addAll := false
	var threadIDs []string

	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printAddHelp()
			return nil
		case "--all", "-A", ".":
			addAll = true
		default:
			if !strings.HasPrefix(arg, "-") {
				threadIDs = append(threadIDs, arg)
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

	if addAll {
		// Stage all unstaged threads
		unstaged, err := repo.GetUnstagedThreads()
		if err != nil {
			return err
		}

		if len(unstaged) == 0 {
			fmt.Println("No unstaged threads to add")
			return nil
		}

		for _, thread := range unstaged {
			if err := repo.StageThread(thread.ID, len(thread.Messages)); err != nil {
				return err
			}
			fmt.Printf("Staged thread %s\n", thread.ID[:8])
		}

		fmt.Printf("\n%d thread(s) staged for commit\n", len(unstaged))
		return nil
	}

	if len(threadIDs) == 0 {
		return fmt.Errorf("nothing specified, nothing added.\nUse \"tin add .\" to stage all threads")
	}

	// Stage specific threads
	for _, id := range threadIDs {
		// Check for @N suffix to stage partial thread
		threadPrefix := id
		var partialCount int
		if idx := strings.Index(id, "@"); idx != -1 {
			threadPrefix = id[:idx]
			fmt.Sscanf(id[idx:], "@%d", &partialCount)
		}

		thread, err := findThreadByPrefix(repo, threadPrefix)
		if err != nil {
			return err
		}

		messageCount := len(thread.Messages)
		if partialCount > 0 && partialCount <= len(thread.Messages) {
			messageCount = partialCount
		}

		if err := repo.StageThread(thread.ID, messageCount); err != nil {
			return err
		}

		fmt.Printf("Staged thread %s (%d messages)\n", thread.ID[:8], messageCount)
	}

	return nil
}

func printAddHelp() {
	fmt.Println(`Stage threads for commit

Usage: tin add [options] <thread-id>...

Options:
  -A, --all, .    Stage all unstaged threads

Arguments:
  <thread-id>  Thread ID (or prefix) to stage
               Use <thread-id>@N to stage only first N messages

Examples:
  tin add abc123           Stage thread abc123
  tin add abc123@10        Stage first 10 messages of thread abc123
  tin add .                Stage all unstaged threads
  tin add --all            Stage all unstaged threads`)
}
