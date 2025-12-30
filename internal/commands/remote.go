package commands

import (
	"fmt"
	"os"

	"github.com/danieladler/tin/internal/storage"
)

func Remote(args []string) error {
	if len(args) == 0 {
		return remoteList()
	}

	switch args[0] {
	case "-h", "--help":
		printRemoteHelp()
		return nil
	case "add":
		return remoteAdd(args[1:])
	case "remove", "rm":
		return remoteRemove(args[1:])
	default:
		return fmt.Errorf("unknown remote subcommand: %s", args[0])
	}
}

func remoteList() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	remotes, err := repo.ListRemotes()
	if err != nil {
		return err
	}

	if len(remotes) == 0 {
		return nil
	}

	for _, r := range remotes {
		fmt.Printf("%s\t%s\n", r.Name, r.URL)
	}

	return nil
}

func remoteAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tin remote add <name> <url>")
	}

	name := args[0]
	url := args[1]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	if err := repo.AddRemote(name, url); err != nil {
		return err
	}

	fmt.Printf("Added remote '%s' -> %s\n", name, url)
	return nil
}

func remoteRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tin remote remove <name>")
	}

	name := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repo, err := storage.Open(cwd)
	if err != nil {
		return err
	}

	if err := repo.RemoveRemote(name); err != nil {
		return err
	}

	fmt.Printf("Removed remote '%s'\n", name)
	return nil
}

func printRemoteHelp() {
	fmt.Println(`Usage: tin remote [command]

Manage remote repositories.

Commands:
  (none)          List all remotes
  add <name> <url>   Add a remote
  remove <name>      Remove a remote

Examples:
  tin remote
  tin remote add origin localhost:2323/path/to/repo.tin
  tin remote remove origin`)
}
