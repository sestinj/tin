package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CredentialEntry represents stored credentials for a host
type CredentialEntry struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

// CredentialStore manages credentials for remote operations
type CredentialStore struct {
	repoPath string
}

// NewCredentialStore creates a new credential store for the given repository
func NewCredentialStore(repoPath string) *CredentialStore {
	return &CredentialStore{
		repoPath: repoPath,
	}
}

// Get retrieves credentials for the given host.
// It checks in order:
// 1. TIN_AUTH_TOKEN environment variable
// 2. Per-host credentials in .tin/config
// 3. Legacy auth_token in .tin/config (for backward compatibility)
func (s *CredentialStore) Get(host string) (*Credentials, error) {
	// 1. Check environment variable (highest priority)
	if token := os.Getenv("TIN_AUTH_TOKEN"); token != "" {
		return &Credentials{
			Username: "x-token-auth",
			Password: token,
		}, nil
	}

	// 2. Check per-host credentials in config
	config, err := s.readConfig()
	if err != nil {
		return nil, nil // No config file, no credentials
	}

	// Look for matching host
	for _, entry := range config.Credentials {
		if entry.Host == host {
			return &Credentials{
				Username: "x-token-auth",
				Password: entry.Token,
			}, nil
		}
	}

	// 3. Fall back to legacy auth_token (backward compatibility)
	if config.AuthToken != "" {
		return &Credentials{
			Username: "x-token-auth",
			Password: config.AuthToken,
		}, nil
	}

	return nil, nil // No credentials found
}

// Store saves credentials for the given host
func (s *CredentialStore) Store(host, token string) error {
	config, err := s.readConfig()
	if err != nil {
		// Create new config if it doesn't exist
		config = &configFile{
			Version:     1,
			Credentials: []CredentialEntry{},
		}
	}

	// Update or add entry
	found := false
	for i, entry := range config.Credentials {
		if entry.Host == host {
			config.Credentials[i].Token = token
			found = true
			break
		}
	}
	if !found {
		config.Credentials = append(config.Credentials, CredentialEntry{
			Host:  host,
			Token: token,
		})
	}

	return s.writeConfig(config)
}

// Remove removes credentials for the given host
func (s *CredentialStore) Remove(host string) error {
	config, err := s.readConfig()
	if err != nil {
		return nil // No config, nothing to remove
	}

	// Find and remove entry
	newCreds := make([]CredentialEntry, 0, len(config.Credentials))
	for _, entry := range config.Credentials {
		if entry.Host != host {
			newCreds = append(newCreds, entry)
		}
	}
	config.Credentials = newCreds

	return s.writeConfig(config)
}

// configFile represents the .tin/config file structure
// This is a local copy to avoid circular imports with storage package
type configFile struct {
	Version       int               `json:"version"`
	Remotes       []json.RawMessage `json:"remotes,omitempty"`
	CodeHostURL   string            `json:"code_host_url,omitempty"`
	ThreadHostURL string            `json:"thread_host_url,omitempty"`
	AuthToken     string            `json:"auth_token,omitempty"`
	Credentials   []CredentialEntry `json:"credentials,omitempty"`
}

func (s *CredentialStore) configPath() string {
	return filepath.Join(s.repoPath, ".tin", "config")
}

func (s *CredentialStore) readConfig() (*configFile, error) {
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		return nil, err
	}

	var config configFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (s *CredentialStore) writeConfig(config *configFile) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath(), data, 0600)
}
