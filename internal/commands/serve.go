package commands

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

// ServeHTTP starts an HTTP server for the TIN protocol
func ServeHTTP(args []string) error {
	addr := ":8443"
	rootPath := ""
	var authPairs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printServeHTTPHelp()
			return nil
		case "--addr", "-a":
			if i+1 < len(args) {
				addr = args[i+1]
				i++
			}
		case "--root":
			if i+1 < len(args) {
				rootPath = args[i+1]
				i++
			}
		case "--auth":
			if i+1 < len(args) {
				authPairs = append(authPairs, args[i+1])
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") && rootPath == "" {
				rootPath = args[i]
			}
		}
	}

	if rootPath == "" {
		return fmt.Errorf("repository root path required (use --root)")
	}

	// Build credentials map from --auth flags and TIN_SERVER_AUTH env var
	credentials := make(map[string]string)

	// Parse TIN_SERVER_AUTH env var (comma-separated user:pass pairs)
	if envAuth := os.Getenv("TIN_SERVER_AUTH"); envAuth != "" {
		for _, pair := range strings.Split(envAuth, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid TIN_SERVER_AUTH format: %q (expected user:pass)", pair)
			}
			credentials[parts[0]] = parts[1]
		}
	}

	// Parse --auth flags (user:pass format)
	for _, pair := range authPairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --auth format: %q (expected user:pass)", pair)
		}
		credentials[parts[0]] = parts[1]
	}

	// Create auth validator
	var authValidator remote.AuthValidator
	if len(credentials) > 0 {
		authValidator = remote.NewTokenAuthValidator(credentials)
		log.Printf("authentication enabled with %d user(s)", len(credentials))
	} else {
		log.Printf("WARNING: no authentication configured, accepting all credentials")
		log.Printf("  use --auth user:pass or TIN_SERVER_AUTH=user:pass to enable auth")
	}

	// Create HTTP handler with auto-create enabled
	handler := remote.NewHTTPHandler(rootPath, true, authValidator)

	log.Printf("tin HTTP server listening on %s", addr)
	log.Printf("serving repositories under: %s", rootPath)
	log.Printf("auto-create enabled: new repos will be created on push")
	log.Printf("\nClient usage:")
	log.Printf("  tin remote add origin https://localhost%s/user/repo", addr)
	log.Printf("  tin config credentials add localhost%s th_yourtoken", addr)
	log.Printf("  tin push origin main")

	return http.ListenAndServe(addr, handler)
}

func printServeHTTPHelp() {
	fmt.Println(`Usage: tin serve-http [options] [root-path]

Start an HTTP server for the TIN protocol with Basic Auth.

Options:
  --addr, -a <addr>   Address to listen on (default: :8443)
  --root <path>       Serve repositories under this root directory
                      (repos are auto-created on push)
  --auth <user:pass>  Add a valid username/password pair (can be repeated)

Authentication:
  Credentials can be provided via:
  - --auth flags:        --auth alice:secret123 --auth bob:hunter2
  - Environment var:     TIN_SERVER_AUTH=alice:secret123,bob:hunter2

  If no credentials are configured, the server accepts any auth (dev mode).

Clients connect using HTTPS URLs and Basic Auth:
  tin remote add origin https://host:port/user/repo
  tin config credentials add host:port alice:secret123
  tin push origin main

HTTP Endpoints:
  POST /{repo-path}/tin-receive-pack  Push (receive data from client)
  POST /{repo-path}/tin-upload-pack   Pull (send data to client)
  POST /{repo-path}/tin-config        Get/set repository config

Examples:
  # Dev mode (no auth validation)
  tin serve-http --root /var/tin-repos

  # With authentication
  tin serve-http --root /var/tin-repos --auth admin:secrettoken

  # Multiple users
  tin serve-http --root ~/repos --auth alice:pass1 --auth bob:pass2

  # Via environment variable
  TIN_SERVER_AUTH=alice:pass1,bob:pass2 tin serve-http --root ~/repos

Note: For production, use a reverse proxy (nginx, caddy) for TLS termination.`)
}
