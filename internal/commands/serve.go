package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dadlerj/tin/internal/remote"
	"github.com/dadlerj/tin/internal/web"
)

func Serve(args []string) error {
	host := "localhost"
	port := 2323
	repoPath := ""
	rootPath := ""
	webMode := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printServeHelp()
			return nil
		case "--host":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				p, err := strconv.Atoi(args[i+1])
				if err != nil {
					return fmt.Errorf("invalid port: %s", args[i+1])
				}
				port = p
				i++
			}
		case "--repo", "-r":
			if i+1 < len(args) {
				repoPath = args[i+1]
				i++
			}
		case "--root":
			if i+1 < len(args) {
				rootPath = args[i+1]
				i++
			}
		case "--web":
			webMode = true
		default:
			if !strings.HasPrefix(args[i], "-") && repoPath == "" && rootPath == "" {
				repoPath = args[i]
			}
		}
	}

	// Web viewer mode
	if webMode {
		if rootPath == "" {
			return fmt.Errorf("--root is required for web mode")
		}
		server := web.NewWebServer(host, port, rootPath)
		return server.Start()
	}

	// Multi-repo mode (--root)
	if rootPath != "" {
		server := remote.NewMultiRepoServer(host, port, rootPath, true)
		return server.Start()
	}

	// Single-repo mode
	if repoPath == "" {
		return fmt.Errorf("repository path required (use --repo or --root)")
	}

	server := remote.NewServer(host, port, repoPath)
	return server.Start()
}

func printServeHelp() {
	fmt.Println(`Usage: tin serve [options] [repo-path]

Start a tin server to accept push/pull connections, or an HTML web viewer.

Options:
  --host <host>     Host to bind to (default: localhost)
  --port, -p <n>    Port to listen on (default: 2323)
  --repo, -r <path> Path to a single bare repository to serve
  --root <path>     Serve any repository under this root directory
                    (repos are auto-created on push)
  --web             Start HTML web viewer instead of push/pull server
                    (requires --root)

Single-repo mode:
  tin serve /path/to/repo.tin
  tin serve --repo /path/to/repo.tin

Multi-repo mode (recommended):
  tin serve --root /var/tin-repos
  # Clients can then push/pull to any path:
  #   tin remote add origin localhost:2323/myproject.tin
  #   tin push origin main  # creates /var/tin-repos/myproject.tin

Web viewer mode:
  tin serve --web --root ~/projects
  # Opens http://localhost:2323 with web interface

Examples:
  tin serve --root ~/tin-repos
  tin serve --host 0.0.0.0 --port 2323 --root /var/tin-repos
  tin serve --web --root ~/projects --port 8080`)
}
