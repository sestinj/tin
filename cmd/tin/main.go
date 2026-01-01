package main

import (
	"fmt"
	"os"

	"github.com/dadlerj/tin/internal/commands"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error

	switch cmd {
	case "init":
		err = commands.Init(args)
	case "status":
		err = commands.Status(args)
	case "branch":
		err = commands.Branch(args)
	case "checkout":
		err = commands.Checkout(args)
	case "add":
		err = commands.Add(args)
	case "commit":
		err = commands.Commit(args)
	case "log":
		err = commands.Log(args)
	case "thread":
		err = commands.Thread(args)
	case "hooks":
		err = commands.Hooks(args)
	case "hook":
		// Internal hook handlers (called by Claude Code hooks)
		err = commands.Hooks(args)
	case "remote":
		err = commands.Remote(args)
	case "push":
		err = commands.Push(args)
	case "sync":
		err = commands.Sync(args)
	case "pull":
		err = commands.Pull(args)
	case "serve":
		err = commands.Serve(args)
	case "config":
		err = commands.Config(args)
	case "version", "--version", "-v":
		fmt.Printf("tin version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "tin: '%s' is not a tin command. See 'tin help'.\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tin - Thread-based version control for conversational coding

Usage: tin <command> [arguments]

Commands:
  init        Initialize a new tin repository
  status      Show the current state of the repository
  branch      Create or list branches
  checkout    Switch branches or restore working tree
  add         Stage threads for commit
  commit      Record changes to the repository
  log         Show commit history with thread summaries
  thread      Manage threads (list, show, start, append)
  hooks       Manage Claude Code integration hooks
  sync        Synchronize tin and git branch state

Remote commands:
  remote      Manage remote repositories
  push        Push commits and threads to remote
  pull        Pull commits and threads from remote
  serve       Start a tin server
  config      View and modify configuration

Options:
  -h, --help     Show this help message
  -v, --version  Show version information

Use "tin <command> --help" for more information about a command.`)
}
