package remote

import (
	"fmt"
	"net/url"
	"strings"
)

// RemoteConfig represents a configured remote repository
type RemoteConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ParsedURL represents a parsed remote URL
type ParsedURL struct {
	Scheme string // "https", "tin", or "" (defaults to tcp)
	Host   string
	Port   string
	Path   string
}

// TransportType returns the transport type for this URL
func (p *ParsedURL) TransportType() string {
	switch p.Scheme {
	case "https":
		return "https"
	default:
		return "tcp"
	}
}

// ParseURL parses a remote URL in the format host:port/path or host/path
// Examples:
//   - localhost:2323/tmp/myproject.tin
//   - example.com:2323/repos/project.tin
//   - example.com/repos/project.tin (default port 2323)
//   - https://tinhub.dev/user/repo
//   - tin://example.com:2323/repos/project.tin
func ParseURL(rawURL string) (*ParsedURL, error) {
	// Handle HTTPS URLs
	if strings.HasPrefix(rawURL, "https://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		port := u.Port()
		if port == "" {
			port = "443"
		}
		return &ParsedURL{
			Scheme: "https",
			Host:   u.Hostname(),
			Port:   port,
			Path:   u.Path,
		}, nil
	}

	// Handle tin:// URLs
	if strings.HasPrefix(rawURL, "tin://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		port := u.Port()
		if port == "" {
			port = "2323"
		}
		return &ParsedURL{
			Scheme: "tin",
			Host:   u.Hostname(),
			Port:   port,
			Path:   u.Path,
		}, nil
	}

	// Handle host:port/path format
	// First, try to find the path separator
	slashIdx := strings.Index(rawURL, "/")
	if slashIdx == -1 {
		return nil, fmt.Errorf("invalid URL: missing path (expected host:port/path or host/path)")
	}

	hostPort := rawURL[:slashIdx]
	path := rawURL[slashIdx:]

	// Check for port
	colonIdx := strings.LastIndex(hostPort, ":")
	var host, port string
	if colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	} else {
		host = hostPort
		port = "2323" // default port
	}

	if host == "" {
		return nil, fmt.Errorf("invalid URL: missing host")
	}

	return &ParsedURL{
		Scheme: "",
		Host:   host,
		Port:   port,
		Path:   path,
	}, nil
}

// Address returns the host:port string for dialing
func (p *ParsedURL) Address() string {
	return fmt.Sprintf("%s:%s", p.Host, p.Port)
}

// String returns the full URL
func (p *ParsedURL) String() string {
	return fmt.Sprintf("%s:%s%s", p.Host, p.Port, p.Path)
}
